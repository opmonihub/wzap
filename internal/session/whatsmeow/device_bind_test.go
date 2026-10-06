package whatsmeow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/storage"
)

type stubInstanceRepo struct {
	byJID map[string]model.Instance
}

func (s *stubInstanceRepo) Create(context.Context, model.Instance) (*model.Instance, error) {
	panic("unexpected")
}
func (s *stubInstanceRepo) Get(context.Context, uuid.UUID) (*model.Instance, error) {
	panic("unexpected")
}
func (s *stubInstanceRepo) GetByName(context.Context, string) (*model.Instance, error) {
	panic("unexpected")
}
func (s *stubInstanceRepo) GetByExternalRef(context.Context, string) (*model.Instance, error) {
	panic("unexpected")
}
func (s *stubInstanceRepo) GetByDeviceJID(_ context.Context, deviceJID string) (*model.Instance, error) {
	instance, ok := s.byJID[deviceJID]
	if !ok {
		return nil, storage.ErrNotFound
	}
	stored := instance
	return &stored, nil
}
func (s *stubInstanceRepo) List(context.Context) ([]model.Instance, error) {
	panic("unexpected")
}
func (s *stubInstanceRepo) UpdateIdentity(context.Context, uuid.UUID, string, string) (*model.Instance, error) {
	panic("unexpected")
}
func (s *stubInstanceRepo) SetWebhook(context.Context, uuid.UUID, *string, bool, []string) error {
	panic("unexpected")
}
func (s *stubInstanceRepo) SetConnection(context.Context, uuid.UUID, string, string) error {
	panic("unexpected")
}
func (s *stubInstanceRepo) SetConnectionState(context.Context, uuid.UUID, string, string, string, *time.Time) error {
	panic("unexpected")
}
func (s *stubInstanceRepo) Delete(context.Context, uuid.UUID) error { panic("unexpected") }

func TestDeviceForRejectsJIDBoundToOtherInstance(t *testing.T) {
	manager := newTestManager(t)
	ownerID := uuid.New()
	otherID := uuid.New()
	jid := types.NewJID("5511999999999", types.DefaultUserServer)
	saveTestDevice(t, manager.devices, "5511999999999")

	manager.instances = &stubInstanceRepo{byJID: map[string]model.Instance{
		jid.String(): {ID: ownerID, Connection: model.InstanceConnection{DeviceJID: jid.String()}},
	}}

	_, err := manager.deviceFor(context.Background(), &model.Instance{
		ID: otherID, Connection: model.InstanceConnection{DeviceJID: jid.String()},
	})
	if !errors.Is(err, session.ErrDeviceJIDTaken) {
		t.Fatalf("deviceFor = %v, want ErrDeviceJIDTaken", err)
	}
}
