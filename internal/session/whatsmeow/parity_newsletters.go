package whatsmeow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

// CreateNewsletter creates a channel with title and description and returns
// its metadata. A title outside 1..100 runes or a description above 500
// runes is ErrInvalidRecipient; an offline client is ErrNotConnected.
func (s *instanceSession) CreateNewsletter(ctx context.Context, title, description string) (session.NewsletterInfo, error) {
	if utf8.RuneCountInString(title) == 0 || utf8.RuneCountInString(title) > 100 {
		return session.NewsletterInfo{}, fmt.Errorf("%w: invalid newsletter title", session.ErrInvalidRecipient)
	}
	if utf8.RuneCountInString(description) > 500 {
		return session.NewsletterInfo{}, fmt.Errorf("%w: invalid newsletter description", session.ErrInvalidRecipient)
	}
	if !s.client.IsConnected() {
		return session.NewsletterInfo{}, fmt.Errorf("%w: create newsletter", session.ErrNotConnected)
	}
	meta, err := s.client.CreateNewsletter(ctx, whatsmeow.CreateNewsletterParams{Name: title, Description: description})
	if err != nil {
		return session.NewsletterInfo{}, classifyRemoteError(err)
	}
	return newsletterFromMeta(meta), nil
}

// NewsletterToggleMute mutes or unmutes channelJID. An unknown channel is
// ErrNotFound.
func (s *instanceSession) NewsletterToggleMute(ctx context.Context, channelJID string, muted bool) error {
	channel, err := parseChannelJID(channelJID)
	if err != nil {
		return err
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: toggle newsletter mute", session.ErrNotConnected)
	}
	if err := s.client.NewsletterToggleMute(ctx, channel, muted); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// NewsletterMarkViewed marks serverIDs of channelJID as viewed. An empty
// batch or more than 100 ids is ErrInvalidRecipient, as is any server id
// that is not a number.
func (s *instanceSession) NewsletterMarkViewed(ctx context.Context, channelJID string, serverIDs []string) error {
	channel, err := parseChannelJID(channelJID)
	if err != nil {
		return err
	}
	if len(serverIDs) == 0 || len(serverIDs) > 100 {
		return fmt.Errorf("%w: invalid newsletter viewed batch size %d", session.ErrInvalidRecipient, len(serverIDs))
	}
	parsed := make([]types.MessageServerID, 0, len(serverIDs))
	for _, raw := range serverIDs {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("%w: invalid newsletter server id %q", session.ErrInvalidRecipient, raw)
		}
		parsed = append(parsed, types.MessageServerID(n))
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: mark newsletter viewed", session.ErrNotConnected)
	}
	if err := s.client.NewsletterMarkViewed(ctx, channel, parsed); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// NewsletterSendReaction sends reaction to serverID of channelJID. An empty
// reaction removes the earlier one and stays valid; only serverID is
// required. An unknown message is ErrNotFound.
func (s *instanceSession) NewsletterSendReaction(ctx context.Context, channelJID, serverID, reaction string) error {
	channel, err := parseChannelJID(channelJID)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(strings.TrimSpace(serverID))
	if strings.TrimSpace(serverID) == "" || err != nil {
		return fmt.Errorf("%w: invalid newsletter server id %q", session.ErrInvalidRecipient, serverID)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: send newsletter reaction", session.ErrNotConnected)
	}
	// An empty reaction id lets the library mint the reaction message id.
	if err := s.client.NewsletterSendReaction(ctx, channel, types.MessageServerID(n), reaction, ""); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// GetNewsletterMessages pages the messages of channelJID from cursor with
// limit entries. An empty limit defaults to 50 and caps at 100; cursor is
// the opaque server id of the page edge, empty for the first page.
// Upstream (GetNewsletterMessagesParams{Count, Before} on the pinned
// version) only pages backward: Before fetches messages older than the id
// and Before 0 omits the attribute for the latest page, so there is no
// After direction to use. nextCursor is the server id of the last entry,
// empty on the final page so callers stop without an extra fetch. An
// unknown channel is ErrNotFound.
func (s *instanceSession) GetNewsletterMessages(ctx context.Context, channelJID, cursor string, limit int) ([]session.NewsletterMessage, string, error) {
	channel, err := parseChannelJID(channelJID)
	if err != nil {
		return nil, "", err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var before types.MessageServerID
	if strings.TrimSpace(cursor) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(cursor))
		if err != nil {
			return nil, "", fmt.Errorf("%w: invalid newsletter cursor %q", session.ErrInvalidRecipient, cursor)
		}
		before = types.MessageServerID(n)
	}
	if !s.client.IsConnected() {
		return nil, "", fmt.Errorf("%w: list newsletter messages", session.ErrNotConnected)
	}
	msgs, err := s.client.GetNewsletterMessages(ctx, channel, &whatsmeow.GetNewsletterMessagesParams{Count: limit, Before: before})
	if err != nil {
		return nil, "", classifyRemoteError(err)
	}
	out := newsletterMessagesFromTypes(msgs)
	var nextCursor string
	if len(out) > 0 {
		nextCursor = out[len(out)-1].ServerID
	}
	return out, nextCursor, nil
}

// GetNewsletterMessageUpdates returns the pending message updates of
// channelJID (reaction and view counters). An unknown channel is
// ErrNotFound.
func (s *instanceSession) GetNewsletterMessageUpdates(ctx context.Context, channelJID string) ([]session.NewsletterMessage, error) {
	channel, err := parseChannelJID(channelJID)
	if err != nil {
		return nil, err
	}
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: list newsletter updates", session.ErrNotConnected)
	}
	msgs, err := s.client.GetNewsletterMessageUpdates(ctx, channel, &whatsmeow.GetNewsletterUpdatesParams{})
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	return newsletterMessagesFromTypes(msgs), nil
}

// newsletterMessagesFromTypes translates upstream channel messages away from
// the library types. Content is the text snapshot, best-effort like the
// inbound text extractor.
func newsletterMessagesFromTypes(msgs []*types.NewsletterMessage) []session.NewsletterMessage {
	out := make([]session.NewsletterMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		var content string
		if msg.Message != nil {
			content = messageText(msg.Message)
		}
		out = append(out, session.NewsletterMessage{
			ServerID:  strconv.Itoa(int(msg.MessageServerID)),
			Content:   content,
			Timestamp: msg.Timestamp,
		})
	}
	return out
}
