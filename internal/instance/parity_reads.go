package instance

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"wzap/internal/session"
)

// ErrContactNotFound reports that the requested contact does not exist
// upstream. The handler maps it to 404. It is distinct from ErrGroupNotFound
// so a missing contact never surfaces with a misleading group message.
var ErrContactNotFound = errors.New("contact not found")

// GetJoinedGroups returns one page of the live groups the instance is a
// member of, ordered by group JID with the cursor of the next page, empty on
// the last page. The cursor is the group JID of the last item of the previous
// page; paging is lexicographic, so an unknown cursor positions without
// failing. A limit <=0 defaults to 50 and values above 100 cap at 100. Error
// mapping follows CreateGroup.
func (s *Service) GetJoinedGroups(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]Group, string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, "", mapError("list joined groups", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, "", fmt.Errorf("list joined groups: create session: %w", err)
	}
	infos, err := sess.GetJoinedGroups(ctx)
	if err != nil {
		return nil, "", mapGroupError("list joined groups", err)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].JID < infos[j].JID })

	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}

	items := make([]Group, 0, len(infos))
	for _, info := range infos {
		if cursor != "" && info.JID <= cursor {
			continue
		}
		items = append(items, s.refreshGroupCache(ctx, id, groupFromSession(info)))
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = items[limit-1].JID
	}
	return items, next, nil
}

// GetGroupInvitePreview resolves inviteCode to the group metadata without
// joining. A blank code is ErrInvalidInput and an unknown or expired code is
// ErrInvalidInput (422 at the REST boundary: a preview is addressable only
// by its exact code, never a 404 listing probe). Other error mapping follows
// CreateGroup.
func (s *Service) GetGroupInvitePreview(ctx context.Context, id uuid.UUID, inviteCode string) (Group, error) {
	if strings.TrimSpace(inviteCode) == "" {
		return Group{}, fmt.Errorf("get group invite preview: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return Group{}, mapError("get group invite preview", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return Group{}, fmt.Errorf("get group invite preview: create session: %w", err)
	}
	info, err := sess.GetGroupInfoFromLink(ctx, inviteCode)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return Group{}, fmt.Errorf("get group invite preview: %w", ErrInvalidInput)
		}
		return Group{}, mapGroupError("get group invite preview", err)
	}
	return s.refreshGroupCache(ctx, id, groupFromSession(info)), nil
}

// CheckContacts looks up phones on WhatsApp, one result per input in order.
// A batch with zero or more than 50 phones is ErrInvalidInput before the
// session is touched. Error mapping follows the directory reads.
func (s *Service) CheckContacts(ctx context.Context, id uuid.UUID, phones []string) ([]session.ContactCheckResult, error) {
	if len(phones) == 0 || len(phones) > 50 {
		return nil, fmt.Errorf("check contacts: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapError("check contacts", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("check contacts: create session: %w", err)
	}
	results, err := sess.CheckContacts(ctx, phones)
	if err != nil {
		return nil, mapContactError("check contacts", err)
	}
	return results, nil
}

// GetContactDevices lists the companion device JIDs of jid. A blank jid is
// ErrInvalidInput; an unknown contact is ErrContactNotFound. Other error
// mapping follows the directory reads.
func (s *Service) GetContactDevices(ctx context.Context, id uuid.UUID, jid string) ([]string, error) {
	if strings.TrimSpace(jid) == "" {
		return nil, fmt.Errorf("get contact devices: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapError("get contact devices", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("get contact devices: create session: %w", err)
	}
	devices, err := sess.GetContactDevices(ctx, jid)
	if err != nil {
		return nil, mapContactError("get contact devices", err)
	}
	return devices, nil
}

// GetContactPhoto returns the picture URL of jid with its version token,
// empty when the contact carries no picture. A blank jid is ErrInvalidInput
// and an unknown contact is ErrContactNotFound. Other error mapping follows
// the directory reads.
func (s *Service) GetContactPhoto(ctx context.Context, id uuid.UUID, jid string) (session.ProfilePictureInfo, error) {
	if strings.TrimSpace(jid) == "" {
		return session.ProfilePictureInfo{}, fmt.Errorf("get contact photo: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return session.ProfilePictureInfo{}, mapError("get contact photo", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return session.ProfilePictureInfo{}, fmt.Errorf("get contact photo: create session: %w", err)
	}
	info, err := sess.GetProfilePictureInfo(ctx, jid)
	if err != nil {
		return session.ProfilePictureInfo{}, mapContactError("get contact photo", err)
	}
	return info, nil
}

// GetContactBusiness returns the business profile of jid. A blank jid is
// ErrInvalidInput and a contact without business profile is
// ErrContactNotFound. Other error mapping follows the directory reads.
func (s *Service) GetContactBusiness(ctx context.Context, id uuid.UUID, jid string) (session.BusinessProfile, error) {
	if strings.TrimSpace(jid) == "" {
		return session.BusinessProfile{}, fmt.Errorf("get contact business: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return session.BusinessProfile{}, mapError("get contact business", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return session.BusinessProfile{}, fmt.Errorf("get contact business: create session: %w", err)
	}
	profile, err := sess.GetBusinessProfile(ctx, jid)
	if err != nil {
		return session.BusinessProfile{}, mapContactError("get contact business", err)
	}
	return profile, nil
}

// GetBlocklist returns the JIDs the instance has blocked. Error mapping
// follows RevokeMessage.
func (s *Service) GetBlocklist(ctx context.Context, id uuid.UUID) ([]string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapError("get blocklist", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("get blocklist: create session: %w", err)
	}
	list, err := sess.GetBlocklist(ctx)
	if err != nil {
		return nil, mapSessionError("get blocklist", err)
	}
	return list, nil
}

// GetStatusPrivacy returns the own status privacy settings. Error mapping
// follows RevokeMessage.
func (s *Service) GetStatusPrivacy(ctx context.Context, id uuid.UUID) (session.StatusPrivacy, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return session.StatusPrivacy{}, mapError("get status privacy", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return session.StatusPrivacy{}, fmt.Errorf("get status privacy: create session: %w", err)
	}
	privacy, err := sess.GetStatusPrivacy(ctx)
	if err != nil {
		return session.StatusPrivacy{}, mapSessionError("get status privacy", err)
	}
	return privacy, nil
}

// GetDisappearingTimer returns the disappearing timer of chatJID, with found
// false when the chat carries no timer. A blank chat is ErrInvalidInput.
// Error mapping follows RevokeMessage.
func (s *Service) GetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string) (time.Duration, bool, error) {
	if strings.TrimSpace(chatJID) == "" {
		return 0, false, fmt.Errorf("get disappearing timer: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return 0, false, mapError("get disappearing timer", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return 0, false, fmt.Errorf("get disappearing timer: create session: %w", err)
	}
	duration, found, err := sess.GetDisappearingTimer(ctx, chatJID)
	if err != nil {
		return 0, false, mapSessionError("get disappearing timer", err)
	}
	return duration, found, nil
}

// GetNewsletterMessages pages the messages of channel from cursor with limit
// entries. A blank channel is ErrInvalidInput; a limit <=0 defaults to 50
// and values above 100 cap at 100. An unknown channel is
// ErrNewsletterNotFound. Error mapping follows FollowNewsletter.
func (s *Service) GetNewsletterMessages(ctx context.Context, id uuid.UUID, channel, cursor string, limit int) ([]session.NewsletterMessage, string, error) {
	if strings.TrimSpace(channel) == "" {
		return nil, "", fmt.Errorf("get newsletter messages: %w", ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, "", mapError("get newsletter messages", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, "", fmt.Errorf("get newsletter messages: create session: %w", err)
	}
	msgs, next, err := sess.GetNewsletterMessages(ctx, channel, cursor, limit)
	if err != nil {
		return nil, "", mapNewsletterError("get newsletter messages", err)
	}
	return msgs, next, nil
}

// GetNewsletterUpdates returns the pending message updates of channel. A
// blank channel is ErrInvalidInput and an unknown channel is
// ErrNewsletterNotFound. Error mapping follows FollowNewsletter.
func (s *Service) GetNewsletterUpdates(ctx context.Context, id uuid.UUID, channel string) ([]session.NewsletterMessage, error) {
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("get newsletter updates: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapError("get newsletter updates", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("get newsletter updates: create session: %w", err)
	}
	msgs, err := sess.GetNewsletterMessageUpdates(ctx, channel)
	if err != nil {
		return nil, mapNewsletterError("get newsletter updates", err)
	}
	return msgs, nil
}

// mapContactError translates a contacts-directory session call failure into
// the service sentinel the HTTP layer maps to a status code, preserving the
// operation context for the logs. An unknown contact is ErrContactNotFound
// (404), never ErrGroupNotFound. Unknown failures pass through unchanged for
// a 500 without leaking their cause (the handler never echoes them).
func mapContactError(op string, err error) error {
	switch {
	case errors.Is(err, session.ErrNotFound):
		return fmt.Errorf("%s: %w", op, ErrContactNotFound)
	default:
		return mapSessionError(op, err)
	}
}
