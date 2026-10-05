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
		Status:      "disconnected",
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
		Status:      "disconnected",
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
	if created.Status != "disconnected" {
		t.Errorf("Create: Status = %q, want %q", created.Status, "disconnected")
	}
	if created.WhatsAppJID != "" {
		t.Errorf("Create: WhatsAppJID = %q, want empty", created.WhatsAppJID)
	}
	if created.LastConnectedAt != nil {
		t.Errorf("Create: LastConnectedAt = %v, want nil", created.LastConnectedAt)
	}
	if created.LastError != "" {
		t.Errorf("Create: LastError = %q, want empty", created.LastError)
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
		Status:      "disconnected",
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
		Status: "disconnected", OwnerUserID: &owner.ID,
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
	if got.WebhookURL != nil {
		t.Errorf("Get WebhookURL = %q, want nil by default", *got.WebhookURL)
	}
	if got.WebhookEnabled {
		t.Error("Get WebhookEnabled = true, want false by default")
	}
	wantEvents := []string{"message", "receipt", "connection", "message.status"}
	if len(got.WebhookEvents) != len(wantEvents) {
		t.Fatalf("Get WebhookEvents = %v, want %v", got.WebhookEvents, wantEvents)
	}
	for i := range wantEvents {
		if got.WebhookEvents[i] != wantEvents[i] {
			t.Fatalf("Get WebhookEvents = %v, want %v", got.WebhookEvents, wantEvents)
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

func TestInstanceRepositoryUpdate(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	instance := createTestInstance(t, repo, "original", "original-ref")

	connectedAt := time.Now().Add(-time.Minute).UTC()
	instance.Name = "renamed"
	instance.ExternalRef = "renamed-ref"
	instance.Status = "connected"
	instance.WhatsAppJID = "5511999999999@s.whatsapp.net"
	instance.LastError = "previous failure"
	instance.LastConnectedAt = &connectedAt

	updated, err := repo.Update(ctx, *instance)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "renamed" || updated.ExternalRef != "renamed-ref" {
		t.Errorf("Update returned %+v", updated)
	}
	if updated.Status != "connected" {
		t.Errorf("Update Status = %q, want connected", updated.Status)
	}
	if updated.WhatsAppJID != "5511999999999@s.whatsapp.net" {
		t.Errorf("Update WhatsAppJID = %q", updated.WhatsAppJID)
	}
	if updated.LastError != "previous failure" {
		t.Errorf("Update LastError = %q", updated.LastError)
	}
	requireTimePtrNear(t, "Update: LastConnectedAt", updated.LastConnectedAt, connectedAt)

	updated.ExternalRef = ""
	updated.WhatsAppJID = ""
	updated.LastError = ""
	updated.LastConnectedAt = nil

	cleared, err := repo.Update(ctx, *updated)
	if err != nil {
		t.Fatalf("Update(clear): %v", err)
	}
	if cleared.ExternalRef != "" || cleared.WhatsAppJID != "" || cleared.LastError != "" || cleared.LastConnectedAt != nil {
		t.Errorf("Update(clear) did not clear optional fields: %+v", cleared)
	}

	createTestInstance(t, repo, "reuses ref", "renamed-ref")
}

func TestInstanceRepositoryUpdateDuplicateExternalRef(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	first := createTestInstance(t, repo, "first", "first-ref")
	second := createTestInstance(t, repo, "second", "second-ref")

	second.ExternalRef = first.ExternalRef
	_, err := repo.Update(ctx, *second)
	if !errors.Is(err, storage.ErrExternalRefTaken) {
		t.Fatalf("Update duplicate external_ref error = %v, want ErrExternalRefTaken", err)
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

	_, err := repo.Update(ctx, model.Instance{
		ID:     uuid.New(),
		Name:   "ghost",
		Status: "disconnected",
	})
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Update(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestInstanceRepositorySetConnection(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	instance := createTestInstance(t, repo, "original", "original-ref")
	connectedAt := time.Now().Add(-time.Minute).UTC()
	instance.Status = "connected"
	instance.WhatsAppJID = "5511999999999@s.whatsapp.net"
	instance.LastError = "previous failure"
	instance.LastConnectedAt = &connectedAt
	if _, err := repo.Update(ctx, *instance); err != nil {
		t.Fatalf("Update seed: %v", err)
	}

	if err := repo.SetConnection(ctx, instance.ID, "disconnected", ""); err != nil {
		t.Fatalf("SetConnection: %v", err)
	}

	got, err := repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after SetConnection: %v", err)
	}
	if got.Status != "disconnected" {
		t.Errorf("status = %q, want disconnected", got.Status)
	}
	if got.WhatsAppJID != "" {
		t.Errorf("whatsapp_jid = %q, want empty", got.WhatsAppJID)
	}
	if got.Name != "original" || got.ExternalRef != "original-ref" {
		t.Errorf("SetConnection touched identity fields: %+v", got)
	}
	if got.LastError != "previous failure" {
		t.Errorf("last_error = %q, want the stored previous failure", got.LastError)
	}
	requireTimePtrNear(t, "SetConnection: LastConnectedAt", got.LastConnectedAt, connectedAt)

	if err := repo.SetConnection(ctx, instance.ID, "connected", "5511888888888@s.whatsapp.net"); err != nil {
		t.Fatalf("SetConnection(connected): %v", err)
	}
	got, err = repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after SetConnection(connected): %v", err)
	}
	if got.Status != "connected" || got.WhatsAppJID != "5511888888888@s.whatsapp.net" {
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
	instance.Status = "connected"
	instance.WhatsAppJID = "5511999999999@s.whatsapp.net"
	instance.LastConnectedAt = &connectedAt
	if _, err := repo.Update(ctx, *instance); err != nil {
		t.Fatalf("Update seed: %v", err)
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
	if got.Status != "error" || got.LastError != "temporary ban" {
		t.Errorf("state = %+v, want status error with the reason", got)
	}
	if got.WhatsAppJID != "5511999999999@s.whatsapp.net" {
		t.Errorf("whatsapp_jid = %q, want the stored JID kept", got.WhatsAppJID)
	}
	if got.Name != "original" || got.ExternalRef != "original-ref" {
		t.Errorf("SetConnectionState touched identity fields: %+v", got)
	}
	requireTimePtrNear(t, "SetConnectionState: LastConnectedAt", got.LastConnectedAt, connectedAt)

	// A connected transition stamps last_connected_at and clears last_error.
	newConnectedAt := time.Now().UTC()
	if err := repo.SetConnectionState(ctx, instance.ID, "connected", "5511888888888@s.whatsapp.net", "", &newConnectedAt); err != nil {
		t.Fatalf("SetConnectionState(connected): %v", err)
	}
	got, err = repo.Get(ctx, instance.ID)
	if err != nil {
		t.Fatalf("Get after SetConnectionState(connected): %v", err)
	}
	if got.Status != "connected" || got.WhatsAppJID != "5511888888888@s.whatsapp.net" {
		t.Errorf("connected state = %+v, want the new status and JID", got)
	}
	if got.LastError != "" {
		t.Errorf("last_error = %q, want empty after a clean connect", got.LastError)
	}
	requireTimePtrNear(t, "SetConnectionState(connected): LastConnectedAt", got.LastConnectedAt, newConnectedAt)

	if err := repo.SetConnectionState(ctx, uuid.New(), "disconnected", "", "", nil); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("SetConnectionState(unknown) error = %v, want ErrNotFound", err)
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
