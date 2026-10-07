package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/storage"
	"wzap/internal/storage/postgres/postgrestest"
)

func TestGroupNewsletterMetadataRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	for _, table := range []string{"group_metadata", "channel_metadata"} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = $1
			)`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("table %s does not exist after Migrate", table)
		}
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "metadata",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	groups := NewGroupMetadataRepository(pool)
	stored, err := groups.Upsert(ctx, model.GroupMetadata{
		InstanceID:       instance.ID,
		GroupJID:         "120363000000000000@g.us",
		Name:             "Time do churrasco",
		Description:      "Só coisa séria",
		ParticipantCount: 3,
	})
	if err != nil {
		t.Fatalf("upsert group metadata: %v", err)
	}
	if stored.UpdatedAt.IsZero() {
		t.Error("group updated_at is zero, want the refresh instant")
	}

	got, err := groups.Get(ctx, instance.ID, "120363000000000000@g.us")
	if err != nil {
		t.Fatalf("get group metadata: %v", err)
	}
	if got.Name != "Time do churrasco" || got.Description != "Só coisa séria" || got.ParticipantCount != 3 {
		t.Errorf("group metadata = %+v, want the upserted refresh", got)
	}

	refreshed, err := groups.Upsert(ctx, model.GroupMetadata{
		InstanceID:       instance.ID,
		GroupJID:         "120363000000000000@g.us",
		Name:             "Novo assunto",
		ParticipantCount: 4,
	})
	if err != nil {
		t.Fatalf("refresh group metadata: %v", err)
	}
	if refreshed.Name != "Novo assunto" || refreshed.ParticipantCount != 4 {
		t.Errorf("refreshed = %+v, want the new subject and count", refreshed)
	}
	if !refreshed.UpdatedAt.After(stored.UpdatedAt) && !refreshed.UpdatedAt.Equal(stored.UpdatedAt) {
		t.Errorf("refreshed updated_at = %v, want >= %v", refreshed.UpdatedAt, stored.UpdatedAt)
	}

	if _, err := groups.Get(ctx, instance.ID, "120363099999999999@g.us"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("get unknown group error = %v, want %v", err, storage.ErrNotFound)
	}

	newsletters := NewNewsletterMetadataRepository(pool)
	if _, err := newsletters.Upsert(ctx, model.NewsletterMetadata{
		InstanceID:    instance.ID,
		ChannelJID:    "12345@newsletter",
		Title:         "Canal da loja",
		Description:   "Ofertas",
		FollowerCount: 41,
	}); err != nil {
		t.Fatalf("upsert newsletter metadata: %v", err)
	}
	if _, err := newsletters.Upsert(ctx, model.NewsletterMetadata{
		InstanceID: instance.ID,
		ChannelJID: "22222@newsletter",
		Title:      "Outro",
	}); err != nil {
		t.Fatalf("upsert second newsletter metadata: %v", err)
	}

	channel, err := newsletters.Get(ctx, instance.ID, "12345@newsletter")
	if err != nil {
		t.Fatalf("get newsletter metadata: %v", err)
	}
	if channel.Title != "Canal da loja" || channel.FollowerCount != 41 {
		t.Errorf("newsletter metadata = %+v, want the upserted refresh", channel)
	}
	if channel.UpdatedAt.IsZero() {
		t.Error("newsletter updated_at is zero, want the refresh instant")
	}

	listed, err := newsletters.ListByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list newsletter metadata: %v", err)
	}
	if len(listed) != 2 || listed[0].ChannelJID != "12345@newsletter" || listed[1].ChannelJID != "22222@newsletter" {
		t.Errorf("listed = %+v, want both channels ordered by JID", listed)
	}

	if _, err := newsletters.Get(ctx, instance.ID, "99999@newsletter"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("get unknown channel error = %v, want %v", err, storage.ErrNotFound)
	}

	if n, err := groups.DeleteByInstance(ctx, instance.ID); err != nil || n != 1 {
		t.Errorf("delete group metadata = (%d, %v), want (1, nil)", n, err)
	}
	if n, err := newsletters.DeleteByInstance(ctx, instance.ID); err != nil || n != 2 {
		t.Errorf("delete newsletter metadata = (%d, %v), want (2, nil)", n, err)
	}
}
