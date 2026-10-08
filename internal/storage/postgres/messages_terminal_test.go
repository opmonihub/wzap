package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/storage"
)

func TestMessageRepositoryRequeueStuckExcludesActiveClaims(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewMessageRepository(pool)
	instance := createTestInstance(t, pool, NewInstanceRepository(pool), "recovery-active", "")
	active := createTestMessage(t, repo, instance.ID, `{"text":"active"}`)
	stale := createTestMessage(t, repo, instance.ID, `{"text":"stale"}`)
	if claimed, err := repo.ClaimQueued(ctx, 2); err != nil || len(claimed) != 2 {
		t.Fatalf("claim = %+v, err = %v", claimed, err)
	}
	old := time.Now().Add(-10 * time.Minute)
	backdateUpdatedAt(t, pool, active.ID, old)
	backdateUpdatedAt(t, pool, stale.ID, old)
	count, err := repo.RequeueStuck(ctx, time.Now().Add(-5*time.Minute), []uuid.UUID{active.ID})
	if err != nil || count != 1 {
		t.Fatalf("recovered = %d, err = %v, want one inactive claim", count, err)
	}
	stored, err := repo.Get(ctx, active.ID)
	if err != nil || stored.Status != "sending" {
		t.Errorf("active claim = %+v, err = %v, want sending", stored, err)
	}
	claimed, err := repo.ClaimQueued(ctx, 2)
	if err != nil || len(claimed) != 1 || claimed[0].ID != stale.ID {
		t.Errorf("recovered claims = %+v, err = %v, want only %s", claimed, err, stale.ID)
	}
}

func TestMessageRepositoryTerminalOutcomeRejectsConflicts(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := NewMessageRepository(pool)
	instance := createTestInstance(t, pool, NewInstanceRepository(pool), "terminal-conflict", "")
	msg := createTestMessage(t, repo, instance.ID, `{"text":"terminal"}`)
	event := testTerminalEvent(t, msg, "sent", "original-wa-id", "")
	if changed, err := repo.MarkSent(ctx, msg.ID, "original-wa-id", event); err != nil || !changed {
		t.Fatalf("first confirmation: changed=%v, err=%v", changed, err)
	}
	if changed, err := repo.MarkSent(ctx, msg.ID, "different-wa-id", testTerminalEvent(t, msg, "sent", "different-wa-id", "")); changed || !errors.Is(err, storage.ErrMessageOutcomeConflict) {
		t.Errorf("different send outcome: changed=%v, err=%v, want conflict", changed, err)
	}
	if changed, err := repo.MarkFailed(ctx, msg.ID, "late failure", testTerminalEvent(t, msg, "failed", "", "late failure")); changed || !errors.Is(err, storage.ErrMessageOutcomeConflict) {
		t.Errorf("opposite terminal outcome: changed=%v, err=%v, want conflict", changed, err)
	}
	stored, err := repo.Get(ctx, msg.ID)
	if err != nil || stored.Status != "sent" || stored.WhatsAppMessageID != "original-wa-id" || stored.LastError != "" {
		t.Errorf("conflict changed confirmed outcome: stored=%+v, err=%v", stored, err)
	}
	pending, err := NewEventOutboxRepository(pool).ClaimPending(ctx, 10)
	if err != nil || len(pending) != 1 || pending[0].ID != event.ID {
		t.Errorf("conflict changed durable events: pending=%+v, err=%v", pending, err)
	}
}
