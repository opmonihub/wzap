package chats

import (
	"context"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
)

type InstanceService interface {
	MarkRead(ctx context.Context, id uuid.UUID, chatJID, senderJID, messageID string) error
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)

	GetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string) (time.Duration, bool, error)

	SetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string, duration time.Duration) error
	SetDefaultDisappearingTimer(ctx context.Context, id uuid.UUID, duration time.Duration) error
}
