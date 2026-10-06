package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"wzap/internal/model"
	"wzap/internal/storage"
	"wzap/internal/storage/postgres/postgrestest"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool := postgrestest.NewPool(t)
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	return pool
}

func createTestInstance(t *testing.T, repo storage.InstanceRepository, name, externalRef string) *model.Instance {
	t.Helper()

	instance, err := repo.Create(context.Background(), model.Instance{
		ID:          uuid.New(),
		Name:        name,
		ExternalRef: externalRef,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance %q: %v", name, err)
	}
	return instance
}

func requireTimeBetween(t *testing.T, label string, got, start, end time.Time) {
	t.Helper()

	if got.IsZero() {
		t.Errorf("%s: time is zero", label)
		return
	}
	if got.Before(start) || got.After(end) {
		t.Errorf("%s: got %s, want between %s and %s", label, got.UTC(), start.UTC(), end.UTC())
	}
}

func requireTimeNear(t *testing.T, label string, got, want time.Time) {
	t.Helper()

	if got.IsZero() {
		t.Errorf("%s: time is zero", label)
		return
	}
	if diff := got.Sub(want); diff < -time.Second || diff > time.Second {
		t.Errorf("%s: got %s, want %s (±1s)", label, got.UTC(), want.UTC())
	}
}

func requireTimePtrNear(t *testing.T, label string, got *time.Time, want time.Time) {
	t.Helper()

	if got == nil {
		t.Errorf("%s: time pointer is nil", label)
		return
	}
	requireTimeNear(t, label, *got, want)
}

func TestInstanceRepositoryCreate(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)
	start := time.Now()

	created, err := repo.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "Account A",
		ExternalRef: "account-a",
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == uuid.Nil {
		t.Error("Create: ID is nil")
	}
	if created.Name != "Account A" {
		t.Errorf("Create: Name = %q, want %q", created.Name, "Account A")
	}
	if created.ExternalRef != "account-a" {
		t.Errorf("Create: ExternalRef = %q, want %q", created.ExternalRef, "account-a")
	}
	if created.Connection.Status != "disconnected" {
		t.Errorf("Create: Status = %q, want %q", created.Connection.Status, "disconnected")
	}
	if created.Connection.DeviceJID != "" {
		t.Errorf("Create: DeviceJID = %q, want empty", created.Connection.DeviceJID)
	}
	if created.Connection.LastConnectedAt != nil {
		t.Errorf("Create: LastConnectedAt = %v, want nil", created.Connection.LastConnectedAt)
	}
	if created.Connection.LastError != nil {
		t.Errorf("Create: LastError = %+v, want nil", created.Connection.LastError)
	}
	requireTimeBetween(t, "Create: CreatedAt", created.CreatedAt, start.Add(-time.Second), time.Now().Add(time.Second))
	requireTimeBetween(t, "Create: UpdatedAt", created.UpdatedAt, start.Add(-time.Second), time.Now().Add(time.Second))

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM instances WHERE id = $1`, created.ID).Scan(&count); err != nil {
		t.Fatalf("count instance: %v", err)
	}
	if count != 1 {
		t.Errorf("instances with created id = %d, want 1", count)
	}
}

func TestInstanceRepositoryCreateDuplicateExternalRef(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	createTestInstance(t, repo, "first", "same-ref")

	_, err := repo.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "second",
		ExternalRef: "same-ref",
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if !errors.Is(err, storage.ErrExternalRefTaken) {
		t.Fatalf("Create duplicate external_ref error = %v, want ErrExternalRefTaken", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM instances WHERE external_ref = 'same-ref'`).Scan(&count); err != nil {
		t.Fatalf("count instances: %v", err)
	}
	if count != 1 {
		t.Errorf("instances with external_ref = %d, want 1", count)
	}
}

func TestInstanceRepositoryCreateWithoutExternalRef(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	createTestInstance(t, repo, "first", "")
	createTestInstance(t, repo, "second", "")

	var nulls int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM instances WHERE external_ref IS NULL`).Scan(&nulls); err != nil {
		t.Fatalf("count null external_ref: %v", err)
	}
	if nulls != 2 {
		t.Errorf("instances with NULL external_ref = %d, want 2", nulls)
	}

	_, err := repo.GetByExternalRef(ctx, "")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetByExternalRef(%q) error = %v, want ErrNotFound", "", err)
	}
}

func TestInstanceRepositoryCreateWithOwner(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	instances := NewInstanceRepository(pool)
	users := NewUserRepository(pool)

	owner := createTestUser(t, users, "owner@example.com", "user", 3)

	created, err := instances.Create(ctx, model.Instance{
		ID: uuid.New(), Name: "owned", ExternalRef: "owned-ref",
		Connection: model.InstanceConnection{Status: "disconnected"}, OwnerUserID: &owner.ID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.OwnerUserID == nil || *created.OwnerUserID != owner.ID {
		t.Fatalf("Create OwnerUserID = %v, want %s", created.OwnerUserID, owner.ID)
	}

	var storedOwner uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT owner_user_id FROM instances WHERE id = $1`, created.ID).Scan(&storedOwner); err != nil {
		t.Fatalf("select owner_user_id: %v", err)
	}
	if storedOwner != owner.ID {
		t.Errorf("owner_user_id column = %s, want %s", storedOwner, owner.ID)
	}

	got, err := instances.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.OwnerUserID == nil || *got.OwnerUserID != owner.ID {
		t.Errorf("Get OwnerUserID = %v, want %s", got.OwnerUserID, owner.ID)
	}
	if got.Webhook.URL != nil {
		t.Errorf("Get WebhookURL = %q, want nil by default", *got.Webhook.URL)
	}
	if got.Webhook.IsEnabled {
		t.Error("Get WebhookEnabled = true, want false by default")
	}
	wantEvents := []string{"message", "receipt", "connection", "message.status"}
	if len(got.Webhook.Events) != len(wantEvents) {
		t.Fatalf("Get WebhookEvents = %v, want %v", got.Webhook.Events, wantEvents)
	}
	for i := range wantEvents {
		if got.Webhook.Events[i] != wantEvents[i] {
			t.Fatalf("Get WebhookEvents = %v, want %v", got.Webhook.Events, wantEvents)
		}
	}
}

func TestInstanceRepositoryGet(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	created := createTestInstance(t, repo, "Account A", "account-a")

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID || got.Name != created.Name || got.ExternalRef != created.ExternalRef {
		t.Errorf("Get returned %+v, want %+v", got, created)
	}

	_, err = repo.Get(ctx, uuid.New())
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestInstanceRepositoryGetByExternalRef(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	created := createTestInstance(t, repo, "Account A", "account-a")

	got, err := repo.GetByExternalRef(ctx, "account-a")
	if err != nil {
		t.Fatalf("GetByExternalRef: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetByExternalRef ID = %s, want %s", got.ID, created.ID)
	}

	_, err = repo.GetByExternalRef(ctx, "missing-ref")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetByExternalRef(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestInstanceRepositoryListCompleteAndDeterministic(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)
	empty, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List(empty): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("List(empty) = %+v, want a non-nil empty slice", empty)
	}
	ids := make([]uuid.UUID, 124)
	oldest := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 123; i++ {
		ids[i] = uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i))
		createdAt := oldest
		if i <= 3 {
			createdAt = oldest.Add(time.Hour)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO instances (id, name, created_at) VALUES ($1, $2, $3)`,
			ids[i], fmt.Sprintf("instance-%d", i), createdAt); err != nil {
			t.Fatalf("insert instance %d: %v", i, err)
		}
	}
	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 123 {
		t.Fatalf("List returned %d instances, want all 123", len(items))
	}
	want := []uuid.UUID{ids[3], ids[2], ids[1]}
	for i := 123; i >= 4; i-- {
		want = append(want, ids[i])
	}
	for i := range want {
		if items[i].ID != want[i] {
			t.Errorf("item %d = %s, want %s (created_at DESC, id DESC)", i, items[i].ID, want[i])
		}
	}
}

func TestInstanceRepositoryUpdateIdentity(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	instance := createTestInstance(t, repo, "original", "original-ref")

	updated, err := repo.UpdateIdentity(ctx, instance.ID, "renamed", "renamed-ref")
	if err != nil {
		t.Fatalf("UpdateIdentity: %v", err)
	}
	if updated.Name != "renamed" || updated.ExternalRef != "renamed-ref" {
		t.Errorf("UpdateIdentity returned %+v", updated)
	}
	if updated.Connection.Status != "disconnected" {
		t.Errorf("UpdateIdentity Status = %q, want disconnected (untouched)", updated.Connection.Status)
	}

	cleared, err := repo.UpdateIdentity(ctx, instance.ID, "renamed", "")
	if err != nil {
		t.Fatalf("UpdateIdentity(clear): %v", err)
	}
	if cleared.ExternalRef != "" {
		t.Errorf("UpdateIdentity(clear) ExternalRef = %q, want empty", cleared.ExternalRef)
	}

	createTestInstance(t, repo, "reuses ref", "renamed-ref")
}

// TestInstanceRepositoryUpdateIdentityDoesNotTouchConnection is the
// lost-update regression test: a concurrent identity edit must not regress a
// connection transition. Before the split, Update rewrote the whole snapshot —
// a PATCH name racing a connection event regraved the stale status.
func TestInstanceRepositoryUpdateIdentityDoesNotTouchConnection(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	instance := createTestInstance(t, repo, "original", "original-ref")
	connectedAt := time.Now().UTC()
	if err := repo.SetConnectionState(ctx, instance.ID, "connected", "5511999999999@s.whatsapp.net", "", &connectedAt); err != nil {
		t.Fatalf("SetConnectionState seed: %v", err)
	}

	// Interleave the two commands: identity edit racing a status transition.
	start := make(chan struct{})
	done := make(chan error, 2)
	go func() {
		<-start
		_, err := repo.UpdateIdentity(ctx, instance.ID, "renamed", "original-ref")
		done <- err
	}()
	go func() {
		<-start
		done <- repo.SetConnectionState(ctx, instance.ID, "error", "", "temporary ban", nil)
	}()
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent write: %v", err)
		}
	}

	got, err := repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after concurrent writes: %v", err)
	}
	if got.Name != "renamed" {
		t.Errorf("name = %q, want renamed", got.Name)
	}
	if got.Connection.Status != "error" {
		t.Errorf("status = %q, want error (identity edit must not regress it)", got.Connection.Status)
	}
	if got.LastErrorMessage() != "temporary ban" {
		t.Errorf("last error = %q, want temporary ban", got.LastErrorMessage())
	}
	if got.Connection.DeviceJID != "5511999999999@s.whatsapp.net" {
		t.Errorf("device_jid = %q, want the bound device kept", got.Connection.DeviceJID)
	}
}

func TestInstanceRepositoryUpdateDuplicateExternalRef(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	first := createTestInstance(t, repo, "first", "first-ref")
	second := createTestInstance(t, repo, "second", "second-ref")

	_, err := repo.UpdateIdentity(ctx, second.ID, second.Name, first.ExternalRef)
	if !errors.Is(err, storage.ErrExternalRefTaken) {
		t.Fatalf("UpdateIdentity duplicate external_ref error = %v, want ErrExternalRefTaken", err)
	}

	got, err := repo.Get(ctx, second.ID)
	if err != nil {
		t.Fatalf("Get after failed update: %v", err)
	}
	if got.ExternalRef != "second-ref" {
		t.Errorf("external_ref changed to %q after failed update, want second-ref", got.ExternalRef)
	}
}

func TestInstanceRepositoryUpdateNotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	_, err := repo.UpdateIdentity(ctx, uuid.New(), "ghost", "")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("UpdateIdentity(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestInstanceRepositorySetConnection(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	instance := createTestInstance(t, repo, "original", "original-ref")
	connectedAt := time.Now().Add(-time.Minute).UTC()
	if err := repo.SetConnectionState(ctx, instance.ID, "connected", "5511999999999@s.whatsapp.net", "previous failure", &connectedAt); err != nil {
		t.Fatalf("SetConnectionState seed: %v", err)
	}

	if err := repo.SetConnection(ctx, instance.ID, "disconnected", ""); err != nil {
		t.Fatalf("SetConnection: %v", err)
	}

	got, err := repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after SetConnection: %v", err)
	}
	if got.Connection.Status != "disconnected" {
		t.Errorf("status = %q, want disconnected", got.Connection.Status)
	}
	if got.Connection.DeviceJID != "" {
		t.Errorf("device_jid = %q, want empty", got.Connection.DeviceJID)
	}
	if got.Name != "original" || got.ExternalRef != "original-ref" {
		t.Errorf("SetConnection touched identity fields: %+v", got)
	}
	if got.Connection.LastError != nil {
		t.Errorf("last_error = %+v, want cleared when pairing is reset", got.Connection.LastError)
	}
	requireTimePtrNear(t, "SetConnection: LastConnectedAt", got.Connection.LastConnectedAt, connectedAt)

	if err := repo.SetConnection(ctx, instance.ID, "connected", "5511888888888@s.whatsapp.net"); err != nil {
		t.Fatalf("SetConnection(connected): %v", err)
	}
	got, err = repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after SetConnection(connected): %v", err)
	}
	if got.Connection.Status != "connected" || got.Connection.DeviceJID != "5511888888888@s.whatsapp.net" {
		t.Errorf("connected state = %+v, want the new status and JID", got)
	}

	if err := repo.SetConnection(ctx, uuid.New(), "disconnected", ""); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("SetConnection(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestInstanceRepositorySetConnectionState(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	instance := createTestInstance(t, repo, "original", "original-ref")
	connectedAt := time.Now().Add(-time.Minute).UTC()
	if err := repo.SetConnectionState(ctx, instance.ID, "connected", "5511999999999@s.whatsapp.net", "", &connectedAt); err != nil {
		t.Fatalf("SetConnectionState seed: %v", err)
	}

	// An error transition with an empty JID keeps the stored JID and
	// last_connected_at, and replaces last_error.
	if err := repo.SetConnectionState(ctx, instance.ID, "error", "", "temporary ban", nil); err != nil {
		t.Fatalf("SetConnectionState: %v", err)
	}
	got, err := repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after SetConnectionState: %v", err)
	}
	if got.Connection.Status != "error" || got.LastErrorMessage() != "temporary ban" {
		t.Errorf("state = %+v, want status error with the reason", got)
	}
	if got.Connection.DeviceJID != "5511999999999@s.whatsapp.net" {
		t.Errorf("device_jid = %q, want the stored JID kept", got.Connection.DeviceJID)
	}
	if got.Name != "original" || got.ExternalRef != "original-ref" {
		t.Errorf("SetConnectionState touched identity fields: %+v", got)
	}
	requireTimePtrNear(t, "SetConnectionState: LastConnectedAt", got.Connection.LastConnectedAt, connectedAt)

	// A connected transition stamps last_connected_at and clears last_error.
	newConnectedAt := time.Now().UTC()
	if err := repo.SetConnectionState(ctx, instance.ID, "connected", "5511888888888@s.whatsapp.net", "", &newConnectedAt); err != nil {
		t.Fatalf("SetConnectionState(connected): %v", err)
	}
	got, err = repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after SetConnectionState(connected): %v", err)
	}
	if got.Connection.Status != "connected" || got.Connection.DeviceJID != "5511888888888@s.whatsapp.net" {
		t.Errorf("connected state = %+v, want the new status and JID", got)
	}
	if got.Connection.LastError != nil {
		t.Errorf("last_error = %+v, want nil after a clean connect", got.Connection.LastError)
	}
	requireTimePtrNear(t, "SetConnectionState(connected): LastConnectedAt", got.Connection.LastConnectedAt, newConnectedAt)

	if err := repo.SetConnectionState(ctx, uuid.New(), "disconnected", "", "", nil); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("SetConnectionState(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestInstanceRepositoryGetByDeviceJID(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	instance := createTestInstance(t, repo, "bound", "bound-ref")
	if err := repo.SetConnection(ctx, instance.ID, "connected", "5511777777777@s.whatsapp.net"); err != nil {
		t.Fatalf("SetConnection: %v", err)
	}

	got, err := repo.GetByDeviceJID(ctx, "5511777777777@s.whatsapp.net")
	if err != nil {
		t.Fatalf("GetByDeviceJID: %v", err)
	}
	if got.ID != instance.ID {
		t.Errorf("id = %s, want %s", got.ID, instance.ID)
	}

	if _, err := repo.GetByDeviceJID(ctx, "missing@s.whatsapp.net"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetByDeviceJID(missing) = %v, want ErrNotFound", err)
	}
}

func TestInstanceRepositoryDeviceJIDUnique(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	first := createTestInstance(t, repo, "first", "first-ref")
	second := createTestInstance(t, repo, "second", "second-ref")
	jid := "5511666666666@s.whatsapp.net"
	if err := repo.SetConnection(ctx, first.ID, "connected", jid); err != nil {
		t.Fatalf("SetConnection(first): %v", err)
	}
	if err := repo.SetConnection(ctx, second.ID, "connected", jid); !errors.Is(err, storage.ErrDeviceJIDTaken) {
		t.Fatalf("SetConnection(second) = %v, want ErrDeviceJIDTaken", err)
	}
}

func TestInstanceRepositoryDelete(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)
	messages := NewMessageRepository(pool)

	instance := createTestInstance(t, repo, "to delete", "delete-ref")
	message := createTestMessage(t, messages, instance.ID, `{"text":"hi"}`)

	if err := repo.Delete(ctx, instance.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.Get(ctx, instance.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get after Delete error = %v, want ErrNotFound", err)
	}
	if _, err := messages.Get(ctx, message.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("message Get after instance Delete error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, instance.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Delete twice error = %v, want ErrNotFound", err)
	}
}
