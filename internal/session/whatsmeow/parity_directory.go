package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

// CheckContacts looks up phones on WhatsApp, one result per input in order.
// An empty batch or more than 50 phones is ErrInvalidRecipient; an offline
// client is ErrNotConnected. Malformed numbers report IsOnWhatsApp false,
// never an error. LastSeen is always nil: the pinned lookup carries no
// presence timestamp.
func (s *instanceSession) CheckContacts(ctx context.Context, phones []string) ([]session.ContactCheckResult, error) {
	if len(phones) == 0 || len(phones) > 50 {
		return nil, fmt.Errorf("%w: invalid contact batch size %d", session.ErrInvalidRecipient, len(phones))
	}
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: check contacts", session.ErrNotConnected)
	}
	out := make([]session.ContactCheckResult, 0, len(phones))
	queries := make([]string, 0, len(phones))
	at := make([]int, 0, len(phones))
	for _, raw := range phones {
		res := session.ContactCheckResult{Phone: raw}
		digits := normalizeContactPhone(raw)
		if digits == "" {
			out = append(out, res)
			continue
		}
		// ParseJID in the pinned library only splits user/server, so the
		// constructed address validates the shape before the lookup.
		if jid, err := types.ParseJID(digits + "@s.whatsapp.net"); err != nil || jid.IsEmpty() {
			out = append(out, res)
			continue
		}
		at = append(at, len(out))
		queries = append(queries, "+"+digits)
		out = append(out, res)
	}
	if len(queries) == 0 {
		return out, nil
	}
	responses, err := s.client.IsOnWhatsApp(ctx, queries)
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	if len(responses) != len(queries) {
		return nil, fmt.Errorf("%w: contact check returned %d results for %d queries", session.ErrTransient, len(responses), len(queries))
	}
	for i, resp := range responses {
		if resp.IsIn && !resp.JID.IsEmpty() {
			out[at[i]].JID = resp.JID.String()
			out[at[i]].IsOnWhatsApp = true
		}
	}
	return out, nil
}

// GetContactDevices lists the companion device JIDs of jid. A malformed jid
// is ErrInvalidRecipient; an offline client is ErrNotConnected.
func (s *instanceSession) GetContactDevices(ctx context.Context, jid string) ([]string, error) {
	parsed, err := types.ParseJID(jid)
	if err != nil || parsed.IsEmpty() {
		return nil, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, jid)
	}
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: list contact devices", session.ErrNotConnected)
	}
	devices, err := s.client.GetUserDevices(ctx, []types.JID{parsed})
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	out := make([]string, 0, len(devices))
	for _, device := range devices {
		out = append(out, device.String())
	}
	return out, nil
}

// GetProfilePictureInfo returns the picture URL of jid with its version
// token. A contact without picture returns an empty URL without failing; an
// unknown contact is ErrNotFound.
func (s *instanceSession) GetProfilePictureInfo(ctx context.Context, jid string) (session.ProfilePictureInfo, error) {
	parsed, err := types.ParseJID(jid)
	if err != nil || parsed.IsEmpty() {
		return session.ProfilePictureInfo{}, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, jid)
	}
	if !s.client.IsConnected() {
		return session.ProfilePictureInfo{}, fmt.Errorf("%w: contact picture", session.ErrNotConnected)
	}
	info, err := s.client.GetProfilePictureInfo(ctx, parsed, nil)
	// A nil params is the default image query, not a panic: the pinned
	// GetProfilePictureInfo replaces nil with &GetProfilePictureParams{}
	// (user.go) and the Session signature stays unchanged.
	if err != nil {
		return session.ProfilePictureInfo{}, classifyRemoteError(err)
	}
	if info == nil {
		return session.ProfilePictureInfo{}, nil
	}
	return session.ProfilePictureInfo{URL: info.URL, Version: info.ID}, nil
}

// GetBusinessProfile returns the business profile of jid. A contact without
// business profile is ErrNotFound. The pinned w:biz query exposes no
// display-name field, so Name and VerifiedName report the verified business
// name from a best-effort user-info lookup (empty when unverified or
// unknown); Description reads the profile_options description slot.
func (s *instanceSession) GetBusinessProfile(ctx context.Context, jid string) (session.BusinessProfile, error) {
	parsed, err := types.ParseJID(jid)
	if err != nil || parsed.IsEmpty() {
		return session.BusinessProfile{}, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, jid)
	}
	if !s.client.IsConnected() {
		return session.BusinessProfile{}, fmt.Errorf("%w: business profile", session.ErrNotConnected)
	}
	profile, err := s.client.GetBusinessProfile(ctx, parsed)
	if err != nil {
		// A contact without business profile answers without the profile
		// node instead of an IQ error, so it needs its own mapping.
		// Sentinel only: the upstream error may carry IQ XML/JID bytes.
		var missing *whatsmeow.ElementMissingError
		if errors.As(err, &missing) {
			return session.BusinessProfile{}, fmt.Errorf("%w: business profile", session.ErrNotFound)
		}
		return session.BusinessProfile{}, classifyRemoteError(err)
	}
	// "description" is a child tag of the upstream <profile_options> node,
	// whose children the pinned parser copies verbatim into
	// BusinessProfile.ProfileOptions keyed by tag (user.go); a missing key
	// reads "".
	out := session.BusinessProfile{Description: profile.ProfileOptions["description"]}
	if infos, uerr := s.client.GetUserInfo(ctx, []types.JID{parsed}); uerr == nil {
		// The Details getter is nil-safe on the pinned protobufs (returns
		// "" on a nil receiver), so the explicit Details check below is
		// belt-and-braces, not load-bearing.
		if info, ok := infos[parsed]; ok && info.VerifiedName != nil && info.VerifiedName.Details != nil {
			out.Name = info.VerifiedName.Details.GetVerifiedName()
			out.VerifiedName = out.Name
		}
	}
	return out, nil
}

// GetContactQRLink returns the own contact QR link, revoking it first when
// revoke is true. An offline client is ErrNotConnected.
func (s *instanceSession) GetContactQRLink(ctx context.Context, revoke bool) (string, error) {
	if !s.client.IsConnected() {
		return "", fmt.Errorf("%w: contact qr link", session.ErrNotConnected)
	}
	link, err := s.client.GetContactQRLink(ctx, revoke)
	if err != nil {
		return "", classifyRemoteError(err)
	}
	return link, nil
}

// normalizeContactPhone keeps only the digits of raw, ignoring the server of
// a JID. It mirrors message.NormalizePhone without importing the domain
// layer: internal/message already depends on internal/session, so importing
// it from the session adapter would invert the domain→session boundary the
// packages are split on. The Brazilian 9th-digit rule is not duplicated
// (single-attempt lookup).
func normalizeContactPhone(raw string) string {
	if user, _, found := strings.Cut(raw, "@"); found {
		raw = user
	}
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
}
