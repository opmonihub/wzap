package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/storage"
)

// TestInstanceRepositoryWebhookRoundtrip proves the webhook satellite stores
// the three config fields: Create persists them, SetWebhook replaces them,
// and an unset URL with an empty subscription round-trips back.
func TestInstanceRepositoryWebhookRoundtrip(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewInstanceRepository(pool)

	url := "https://hooks.example.com/wzap"
	created, err := repo.Create(ctx, model.Instance{
		ID: uuid.New(), Name: "webhook",
		Connection: model.InstanceConnection{Status: "disconnected"},
		Webhook: model.InstanceWebhook{
			URL: &url, IsEnabled: true,
			Events: []string{"message", "message.status"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Webhook.URL == nil || *created.Webhook.URL != url {
		t.Errorf("Create WebhookURL = %v, want %q", created.Webhook.URL, url)
	}
	if !created.Webhook.IsEnabled {
		t.Error("Create WebhookEnabled = false, want true")
	}
	if want := []string{"message", "message.status"}; !reflect.DeepEqual(created.Webhook.Events, want) {
		t.Errorf("Create WebhookEvents = %v, want %v", created.Webhook.Events, want)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Webhook.URL == nil || *got.Webhook.URL != url {
		t.Errorf("Get WebhookURL = %v, want %q", got.Webhook.URL, url)
	}
	if !reflect.DeepEqual(got.Webhook.Events, []string{"message", "message.status"}) {
		t.Errorf("Get WebhookEvents = %v, want the stored subscription", got.Webhook.Events)
	}

	// SetWebhook replaces the stored config without touching identity or
	// connection columns.
	if err := repo.SetConnection(ctx, created.ID, "connected", "5511999999999@s.whatsapp.net"); err != nil {
		t.Fatalf("SetConnection seed: %v", err)
	}
	if err := repo.SetWebhook(ctx, created.ID, nil, false, []string{}); err != nil {
		t.Fatalf("SetWebhook: %v", err)
	}

	cleared, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get after SetWebhook: %v", err)
	}
	if cleared.Webhook.URL != nil {
		t.Errorf("Get WebhookURL = %q, want nil (unset)", *cleared.Webhook.URL)
	}
	if cleared.Webhook.IsEnabled {
		t.Error("Get WebhookEnabled = true, want the stored false")
	}
	if cleared.Webhook.Events == nil || len(cleared.Webhook.Events) != 0 {
		t.Errorf("Get WebhookEvents = %v, want the explicit empty list", cleared.Webhook.Events)
	}
	if cleared.Connection.Status != "connected" || cleared.Connection.DeviceJID != "5511999999999@s.whatsapp.net" {
		t.Errorf("SetWebhook touched the connection state: %+v", cleared.Connection)
	}

	if err := repo.SetWebhook(ctx, uuid.New(), nil, false, nil); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("SetWebhook(unknown) error = %v, want ErrNotFound", err)
	}
}
