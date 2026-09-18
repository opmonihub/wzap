package whatsmeow

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"wzap/internal/session"
)

// EditMessage replaces the text of messageID in chatJID. An empty or
// over-long text is ErrInvalidRecipient; an offline client is
// ErrNotConnected. The upstream edit carries no new id when the server
// echoes none, so the original id is returned then.
func (s *instanceSession) EditMessage(ctx context.Context, chatJID, messageID, text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if utf8.RuneCountInString(trimmed) == 0 || utf8.RuneCountInString(trimmed) > 4096 {
		return "", fmt.Errorf("%w: invalid edit text", session.ErrInvalidRecipient)
	}
	chat, err := types.ParseJID(chatJID)
	if err != nil || chat.IsEmpty() {
		return "", fmt.Errorf("%w: %s", session.ErrInvalidRecipient, chatJID)
	}
	if strings.TrimSpace(messageID) == "" {
		return "", fmt.Errorf("%w: empty message id", session.ErrInvalidRecipient)
	}
	if !s.client.IsConnected() {
		return "", fmt.Errorf("%w: edit message", session.ErrNotConnected)
	}
	built := s.client.BuildEdit(chat, messageID, &waE2E.Message{Conversation: proto.String(trimmed)})
	resp, err := s.client.SendMessage(ctx, chat, built)
	if err != nil {
		return "", classifySessionError(err)
	}
	if resp.ID == "" {
		return messageID, nil
	}
	return resp.ID, nil
}
