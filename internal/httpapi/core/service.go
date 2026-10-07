package core

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/model"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)
	GetByName(ctx context.Context, name string) (*model.Instance, error)
}

type InstanceReader interface {
	Get(context.Context, uuid.UUID) (*model.Instance, error)
}
