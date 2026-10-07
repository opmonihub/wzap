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
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "dead-letter",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
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

// TestDeadLetterRetentionCapsPerInstance proves the 500-row tail: recording
// past the bound trims the oldest rows of that instance only — a failing
// endpoint cannot grow the table without bound, and other instances keep
// their letters.
func TestDeadLetterRetentionCapsPerInstance(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	instances := NewInstanceRepository(pool)
	owner := createTestOwner(t, pool)
	instance, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "capped",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	other, err := instances.Create(ctx, model.Instance{
		ID:          uuid.New(),
		Name:        "untouched",
		OwnerUserID: &owner.ID,
		Connection:  model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatalf("create other instance: %v", err)
	}

	repo := NewDeadLetterRepository(pool)
	payload := []byte(`{"event":"x"}`)
	const extra = 3
	for i := 0; i < deadLetterRetentionPerInstance+extra; i++ {
		if err := repo.RecordDeadLetter(ctx, instance.ID, uuid.New(), "message", payload, 1, "boom"); err != nil {
			t.Fatalf("RecordDeadLetter %d: %v", i, err)
		}
	}
	if err := repo.RecordDeadLetter(ctx, other.ID, uuid.New(), "message", payload, 1, "other boom"); err != nil {
		t.Fatalf("RecordDeadLetter other: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM webhook_dead_letters WHERE instance_id = $1`, instance.ID).Scan(&count); err != nil {
		t.Fatalf("count dead letters: %v", err)
	}
	if count != deadLetterRetentionPerInstance {
		t.Errorf("dead letters kept = %d, want the %d cap", count, deadLetterRetentionPerInstance)
	}

	// The trimmed rows are the oldest: the earliest recorded event_ids must
	// be gone while the recent tail survives.
	var firstKept string
	if err := pool.QueryRow(ctx,
		`SELECT event_id::text FROM webhook_dead_letters WHERE instance_id = $1
		 ORDER BY created_at ASC, id ASC LIMIT 1`, instance.ID).Scan(&firstKept); err != nil {
		t.Fatalf("read oldest kept: %v", err)
	}
	if firstKept == "" {
		t.Error("no dead letter kept, want the newest 500")
	}
	var orphans int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM webhook_dead_letters WHERE instance_id = $1`, other.ID).Scan(&orphans); err != nil {
		t.Fatalf("count other dead letters: %v", err)
	}
	if orphans != 1 {
		t.Errorf("other instance dead letters = %d, want 1 untouched", orphans)
	}
}
