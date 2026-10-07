package contacts

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/session"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)

	CheckContacts(ctx context.Context, id uuid.UUID, phones []string) ([]session.ContactCheckResult, error)
	GetContactDevices(ctx context.Context, id uuid.UUID, jid string) ([]string, error)
	GetContactPhoto(ctx context.Context, id uuid.UUID, jid string) (session.ProfilePictureInfo, error)
	GetContactBusiness(ctx context.Context, id uuid.UUID, jid string) (session.BusinessProfile, error)
	GetBlocklist(ctx context.Context, id uuid.UUID) ([]string, error)

	UpdateBlocklist(ctx context.Context, id uuid.UUID, jid, action string) error

	SubscribePresence(ctx context.Context, id uuid.UUID, jid string) error
	GetContactQRLink(ctx context.Context, id uuid.UUID, revoke bool) (string, error)
}
