package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/storage/postgres/postgrestest"
)

func TestDeadLetterRecordAndDedupe(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:         uuid.New(),
		Name:       "dead-letter",
		Connection: model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	repo := NewDeadLetterRepository(pool)
	eventID := uuid.New()
	payload := []byte(`{"event_id":"` + eventID.String() + `"}`)
	if err := repo.RecordDeadLetter(ctx, instance.ID, eventID, "message", payload, 8, "connection refused"); err != nil {
		t.Fatalf("RecordDeadLetter: %v", err)
	}
	// A replayed event re-records silently instead of failing.
	if err := repo.RecordDeadLetter(ctx, instance.ID, eventID, "message", payload, 8, "connection refused"); err != nil {
		t.Fatalf("RecordDeadLetter duplicate: %v", err)
	}

	var count int
	var eventType, lastError string
	var attempts int
	var stored []byte
	if err := pool.QueryRow(ctx, `SELECT count(*), max(event_type), max(attempt_count), max(last_error_message), max(envelope::text)::bytea FROM webhook_dead_letters WHERE instance_id = $1`,
		instance.ID).Scan(&count, &eventType, &attempts, &lastError, &stored); err != nil {
		t.Fatalf("read dead letters: %v", err)
	}
	if count != 1 {
		t.Errorf("dead letters = %d, want 1 (deduplicated by event_id)", count)
	}
	if eventType != "message" {
		t.Errorf("event_type = %q, want %q", eventType, "message")
	}
	if attempts != 8 {
		t.Errorf("attempts = %d, want 8", attempts)
	}
	if lastError != "connection refused" {
		t.Errorf("last_error = %q, want the final failure", lastError)
	}
	if string(stored) != string(payload) {
		// jsonb normalizes spacing: compare the decoded documents.
		var gotDoc, wantDoc map[string]any
		if err := json.Unmarshal(stored, &gotDoc); err != nil {
			t.Fatalf("stored payload is not JSON: %v", err)
		}
		if err := json.Unmarshal(payload, &wantDoc); err != nil {
			t.Fatalf("want payload is not JSON: %v", err)
		}
		if len(gotDoc) != len(wantDoc) || gotDoc["event_id"] != wantDoc["event_id"] {
			t.Errorf("payload = %s, want %s", stored, payload)
		}
	}
}
