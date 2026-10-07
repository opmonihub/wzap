package statuses

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/session"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)

	PublishStatus(ctx context.Context, id uuid.UUID, input session.StatusInput) (string, error)
	ListStatuses(ctx context.Context, id uuid.UUID) ([]session.StatusInfo, error)
	DeleteStatus(ctx context.Context, id uuid.UUID, statusID string) error

	GetStatusPrivacy(ctx context.Context, id uuid.UUID) (session.StatusPrivacy, error)
}
