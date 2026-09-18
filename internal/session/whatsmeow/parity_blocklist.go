package whatsmeow

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"wzap/internal/session"
)

// blocklistChangeActionUnblock is the unblock literal. go doc
// go.mau.fi/whatsmeow/types/events.BlocklistChangeAction on the pinned
// version (v0.0.0-20260915211301-f376da267f95) exposes only
// BlocklistChangeActionBlock ("block") as a named constant, so the validated
// "unblock" string converts explicitly instead of inventing a constant in
// the events package.
const blocklistChangeActionUnblock = events.BlocklistChangeAction("unblock")

// GetBlocklist returns the JIDs the instance has blocked. An offline client
// is ErrNotConnected.
func (s *instanceSession) GetBlocklist(ctx context.Context) ([]string, error) {
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: list blocklist", session.ErrNotConnected)
	}
	list, err := s.client.GetBlocklist(ctx)
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	out := make([]string, 0)
	if list != nil {
		out = make([]string, 0, len(list.JIDs))
		for _, jid := range list.JIDs {
			out = append(out, jid.String())
		}
	}
	return out, nil
}

// UpdateBlocklist applies action (block or unblock) to jid. An unknown
// action is ErrInvalidRecipient; a malformed jid is ErrInvalidRecipient.
func (s *instanceSession) UpdateBlocklist(ctx context.Context, jid, action string) error {
	var change events.BlocklistChangeAction
	switch action {
	case "block":
		change = events.BlocklistChangeActionBlock
	case "unblock":
		change = blocklistChangeActionUnblock
	default:
		return fmt.Errorf("%w: unknown blocklist action %q", session.ErrInvalidRecipient, action)
	}
	parsed, err := types.ParseJID(jid)
	if err != nil || parsed.IsEmpty() {
		return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, jid)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: update blocklist", session.ErrNotConnected)
	}
	if _, err := s.client.UpdateBlocklist(ctx, parsed, change); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}
