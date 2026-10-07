package messages

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/model"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)

	RevokeMessage(ctx context.Context, id uuid.UUID, chatJID, messageID string) error

	EditMessage(ctx context.Context, id uuid.UUID, chatJID, messageID, text string) (string, error)
}
