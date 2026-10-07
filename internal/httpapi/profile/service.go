package profile

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/session"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)

	GetProfile(ctx context.Context, id uuid.UUID) (session.Profile, error)
	SetProfileName(ctx context.Context, id uuid.UUID, name string) error
	SetProfileStatusText(ctx context.Context, id uuid.UUID, text string) error
	SetProfilePhoto(ctx context.Context, id uuid.UUID, image []byte) error
	GetPrivacy(ctx context.Context, id uuid.UUID) (session.Privacy, error)
	SetPrivacy(ctx context.Context, id uuid.UUID, input session.Privacy) (session.Privacy, error)
}
