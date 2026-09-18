package instance

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) EditMessage(ctx context.Context, id uuid.UUID, chatJID, messageID, text string) (string, error) {
	if utf8.RuneCountInString(strings.TrimSpace(text)) == 0 || utf8.RuneCountInString(text) > 4096 {
		return "", fmt.Errorf("edit message: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(chatJID) == "" || strings.TrimSpace(messageID) == "" {
		return "", fmt.Errorf("edit message: %w", ErrInvalidInput)
	}
	inst, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", mapError("edit message", err)
	}
	sess, err := s.sessionFor(ctx, inst)
	if err != nil {
		return "", fmt.Errorf("edit message: create session: %w", err)
	}
	out, err := sess.EditMessage(ctx, chatJID, messageID, text)
	if err != nil {
		return "", mapGroupError("edit message", err)
	}
	return out, nil
}
