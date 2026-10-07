package instance

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"wzap/internal/session"
)

// UpdateBlocklist applies action (block or unblock) to jid. A blank jid or an
// action outside block|unblock is ErrInvalidInput before the session is
// touched. Error mapping follows RevokeMessage.
func (s *Service) UpdateBlocklist(ctx context.Context, id uuid.UUID, jid, action string) error {
	if strings.TrimSpace(jid) == "" {
		return fmt.Errorf("update blocklist: %w", ErrInvalidInput)
	}
	if action != "block" && action != "unblock" {
		return fmt.Errorf("update blocklist: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("update blocklist", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("update blocklist: create session: %w", err)
	}
	if err := sess.UpdateBlocklist(ctx, jid, action); err != nil {
		return mapSessionError("update blocklist", err)
	}
	return nil
}

// validDisappearingDuration reports whether duration is on the protocol
// allowlist: off (0), 24h, 7d or 90d.
func validDisappearingDuration(duration time.Duration) bool {
	switch duration {
	case session.DisappearingOff, session.Disappearing24h, session.Disappearing7d, session.Disappearing90d:
		return true
	default:
		return false
	}
}

// SetDisappearingTimer sets the disappearing timer of chatJID. A blank chat
// or a duration off the allowlist is ErrInvalidInput before the session is
// touched. Error mapping follows RevokeMessage.
func (s *Service) SetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string, duration time.Duration) error {
	if strings.TrimSpace(chatJID) == "" {
		return fmt.Errorf("set disappearing timer: %w", ErrInvalidInput)
	}
	if !validDisappearingDuration(duration) {
		return fmt.Errorf("set disappearing timer: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("set disappearing timer", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("set disappearing timer: create session: %w", err)
	}
	if err := sess.SetDisappearingTimer(ctx, chatJID, duration); err != nil {
		return mapSessionError("set disappearing timer", err)
	}
	return nil
}

// SetDefaultDisappearingTimer sets the default disappearing timer for new
// chats and persists the echo of the applied value (read back as
// settings.default_disappearing). A duration off the allowlist is
// ErrInvalidInput before the session is touched. Error mapping follows
// RevokeMessage; a failed echo persist answers 500.
func (s *Service) SetDefaultDisappearingTimer(ctx context.Context, id uuid.UUID, duration time.Duration) error {
	if !validDisappearingDuration(duration) {
		return fmt.Errorf("set default disappearing timer: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("set default disappearing timer", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("set default disappearing timer: create session: %w", err)
	}
	if err := sess.SetDefaultDisappearingTimer(ctx, duration); err != nil {
		return mapSessionError("set default disappearing timer", err)
	}
	// Persist the echo of the applied timer so settings.default_disappearing
	// can report it later (instance_chat_settings satellite). The write runs
	// only after the upstream accepted the change; a failed persist fails the
	// call (500) so the stored echo never drifts from what was applied.
	if err := s.repo.SetDefaultDisappearing(ctx, id, duration); err != nil {
		return mapError("set default disappearing timer", err)
	}
	return nil
}

// SubscribePresence subscribes to the presence of jid. A blank jid is
// ErrInvalidInput before the session is touched. Error mapping follows
// RevokeMessage.
func (s *Service) SubscribePresence(ctx context.Context, id uuid.UUID, jid string) error {
	if strings.TrimSpace(jid) == "" {
		return fmt.Errorf("subscribe presence: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("subscribe presence", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("subscribe presence: create session: %w", err)
	}
	if err := sess.SubscribePresence(ctx, jid); err != nil {
		return mapSessionError("subscribe presence", err)
	}
	return nil
}

// GetContactQRLink returns the own contact QR link, revoking it first when
// revoke is true. Error mapping follows RevokeMessage.
func (s *Service) GetContactQRLink(ctx context.Context, id uuid.UUID, revoke bool) (string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", mapError("get contact qr link", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return "", fmt.Errorf("get contact qr link: create session: %w", err)
	}
	link, err := sess.GetContactQRLink(ctx, revoke)
	if err != nil {
		return "", mapSessionError("get contact qr link", err)
	}
	return link, nil
}

// CreateNewsletter creates a channel with title and description and returns
// its metadata. A blank title or one longer than 100 runes, or a description
// longer than 500 runes, is ErrInvalidInput before the session is touched; a
// duplicate title is ErrForbidden. Like FollowNewsletter, the cache is
// refreshed from the created metadata without failing the call. Error mapping
// follows FollowNewsletter.
func (s *Service) CreateNewsletter(ctx context.Context, id uuid.UUID, title, description string) (Newsletter, error) {
	if utf8.RuneCountInString(strings.TrimSpace(title)) == 0 || utf8.RuneCountInString(title) > 100 {
		return Newsletter{}, fmt.Errorf("create newsletter: %w", ErrInvalidInput)
	}
	if utf8.RuneCountInString(description) > 500 {
		return Newsletter{}, fmt.Errorf("create newsletter: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return Newsletter{}, mapError("create newsletter", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return Newsletter{}, fmt.Errorf("create newsletter: create session: %w", err)
	}
	info, err := sess.CreateNewsletter(ctx, title, description)
	if err != nil {
		return Newsletter{}, mapNewsletterError("create newsletter", err)
	}
	return s.refreshNewsletterCache(ctx, id, newsletterFromSession(info)), nil
}

// MuteNewsletter mutes or unmutes channelJID. A blank channel is
// ErrInvalidInput before the session is touched. Error mapping follows
// FollowNewsletter.
func (s *Service) MuteNewsletter(ctx context.Context, id uuid.UUID, channel string, muted bool) error {
	if strings.TrimSpace(channel) == "" {
		return fmt.Errorf("mute newsletter: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("mute newsletter", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("mute newsletter: create session: %w", err)
	}
	if err := sess.NewsletterToggleMute(ctx, channel, muted); err != nil {
		return mapNewsletterError("mute newsletter", err)
	}
	return nil
}

// MarkNewsletterViewed marks serverIDs of channel as viewed. A blank channel
// or a batch with zero or more than 100 ids is ErrInvalidInput before the
// session is touched. Error mapping follows FollowNewsletter.
func (s *Service) MarkNewsletterViewed(ctx context.Context, id uuid.UUID, channel string, serverIDs []string) error {
	if strings.TrimSpace(channel) == "" {
		return fmt.Errorf("mark newsletter viewed: %w", ErrInvalidInput)
	}
	if len(serverIDs) == 0 || len(serverIDs) > 100 {
		return fmt.Errorf("mark newsletter viewed: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("mark newsletter viewed", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("mark newsletter viewed: create session: %w", err)
	}
	if err := sess.NewsletterMarkViewed(ctx, channel, serverIDs); err != nil {
		return mapNewsletterError("mark newsletter viewed", err)
	}
	return nil
}

// ReactNewsletter sends reaction to serverID of channel. A blank channel or
// serverID is ErrInvalidInput before the session is touched; an empty
// reaction is valid and removes the reaction. Error mapping follows
// FollowNewsletter.
func (s *Service) ReactNewsletter(ctx context.Context, id uuid.UUID, channel, serverID, reaction string) error {
	if strings.TrimSpace(channel) == "" {
		return fmt.Errorf("react newsletter: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(serverID) == "" {
		return fmt.Errorf("react newsletter: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("react newsletter", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("react newsletter: create session: %w", err)
	}
	if err := sess.NewsletterSendReaction(ctx, channel, serverID, reaction); err != nil {
		return mapNewsletterError("react newsletter", err)
	}
	return nil
}
