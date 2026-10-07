package instances

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/session"
)

type InstanceService interface {
	Create(ctx context.Context, input instance.CreateInput) (*model.Instance, string, error)
	OldestAdmin(ctx context.Context) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)
	GetByName(ctx context.Context, name string) (*model.Instance, error)
	List(ctx context.Context) ([]model.Instance, error)
	Update(ctx context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Disconnect(ctx context.Context, id uuid.UUID) error
	Connect(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)
	QR(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)

	SendPresence(ctx context.Context, id uuid.UUID, chatJID, state string) error
	PairPhone(ctx context.Context, id uuid.UUID, phone string) (instance.PairPhoneResult, error)

	RejectCall(ctx context.Context, id uuid.UUID, fromJID, callID string) error
	GetProfile(ctx context.Context, id uuid.UUID) (session.Profile, error)

	GetPrivacy(ctx context.Context, id uuid.UUID) (session.Privacy, error)

	GetStatusPrivacy(ctx context.Context, id uuid.UUID) (session.StatusPrivacy, error)
}
