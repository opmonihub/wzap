package users

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/model"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)
}
