package whatsmeow

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

// validDisappearing reports whether duration is on the protocol allowlist:
// off, 24 hours, 7 days or 90 days.
func validDisappearing(d time.Duration) bool {
	switch d {
	case 0, 24 * time.Hour, 7 * 24 * time.Hour, 90 * 24 * time.Hour:
		return true
	}
	return false
}

// SetDisappearingTimer sets the disappearing timer of chatJID. A duration
// off the allowlist is ErrInvalidRecipient; an offline client is
// ErrNotConnected. The timer is recorded locally on success: the pinned
// library exposes no fetch, so GetDisappearingTimer only knows what this
// session set.
func (s *instanceSession) SetDisappearingTimer(ctx context.Context, chatJID string, duration time.Duration) error {
	if !validDisappearing(duration) {
		return fmt.Errorf("%w: unsupported disappearing duration %s", session.ErrInvalidRecipient, duration)
	}
	chat, err := types.ParseJID(chatJID)
	if err != nil || chat.IsEmpty() {
		return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, chatJID)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set disappearing timer", session.ErrNotConnected)
	}
	if err := s.client.SetDisappearingTimer(ctx, chat, duration, time.Now()); err != nil {
		return classifyRemoteError(err)
	}
	s.disappearingMu.Lock()
	if s.disappearing == nil {
		s.disappearing = make(map[string]time.Duration)
	}
	s.disappearing[chatJID] = duration
	s.disappearingMu.Unlock()
	return nil
}

// SetDefaultDisappearingTimer sets the default disappearing timer for new
// chats. A duration off the allowlist is ErrInvalidRecipient.
func (s *instanceSession) SetDefaultDisappearingTimer(ctx context.Context, duration time.Duration) error {
	if !validDisappearing(duration) {
		return fmt.Errorf("%w: unsupported disappearing duration %s", session.ErrInvalidRecipient, duration)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set default disappearing timer", session.ErrNotConnected)
	}
	if err := s.client.SetDefaultDisappearingTimer(ctx, duration); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// GetDisappearingTimer returns the disappearing timer of chatJID, with found
// false when the chat carries no known timer. Best-effort: the pinned
// library exposes no fetch, so only timers set through this session lifetime
// are reported; anything else answers (0, false, nil).
func (s *instanceSession) GetDisappearingTimer(ctx context.Context, chatJID string) (time.Duration, bool, error) {
	_ = ctx
	chat, err := types.ParseJID(chatJID)
	if err != nil || chat.IsEmpty() {
		return 0, false, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, chatJID)
	}
	if !s.client.IsConnected() {
		return 0, false, fmt.Errorf("%w: get disappearing timer", session.ErrNotConnected)
	}
	s.disappearingMu.RLock()
	duration, ok := s.disappearing[chatJID]
	s.disappearingMu.RUnlock()
	if !ok {
		return 0, false, nil
	}
	return duration, true, nil
}

// GetStatusPrivacy returns the own status privacy settings. Upstream knows
// contacts, blacklist and whitelist (types.StatusPrivacyType*); the session
// contract only allows contacts, contact_blacklist and none, so whitelist
// maps to none. The JIDs are preserved best-effort for caller visibility
// even in the none case: upstream none (whitelist) still carries its member
// list, and dropping it would hide who the setting applies to. An empty
// upstream answers the contacts default, like the fake does.
func (s *instanceSession) GetStatusPrivacy(ctx context.Context) (session.StatusPrivacy, error) {
	if !s.client.IsConnected() {
		return session.StatusPrivacy{}, fmt.Errorf("%w: status privacy", session.ErrNotConnected)
	}
	list, err := s.client.GetStatusPrivacy(ctx)
	if err != nil {
		return session.StatusPrivacy{}, classifyRemoteError(err)
	}
	return statusPrivacyFromTypes(list), nil
}

// statusPrivacyFromTypes translates the upstream status audience away from
// the library types, keeping the first (default) entry.
func statusPrivacyFromTypes(list []types.StatusPrivacy) session.StatusPrivacy {
	if len(list) == 0 {
		return session.StatusPrivacy{Mode: "contacts"}
	}
	first := list[0]
	// Default none covers whitelist (no session mode for it, see
	// GetStatusPrivacy) and any future upstream type.
	mode := "none"
	switch first.Type {
	case types.StatusPrivacyTypeContacts:
		mode = "contacts"
	case types.StatusPrivacyTypeBlacklist:
		mode = "contact_blacklist"
	}
	out := session.StatusPrivacy{Mode: mode, JIDs: make([]string, 0, len(first.List))}
	for _, jid := range first.List {
		out.JIDs = append(out.JIDs, jid.String())
	}
	return out
}

// SubscribePresence subscribes to the presence of jid with a single signal:
// no heartbeat loop is started. A malformed jid is ErrInvalidRecipient.
func (s *instanceSession) SubscribePresence(ctx context.Context, jid string) error {
	parsed, err := types.ParseJID(jid)
	if err != nil || parsed.IsEmpty() {
		return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, jid)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: subscribe presence", session.ErrNotConnected)
	}
	if err := s.client.SubscribePresence(ctx, parsed); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}
