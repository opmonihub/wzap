package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/storage"
	"wzap/internal/storage/postgres/postgrestest"
)

func TestChatwootPendingPutKeepsCompletedSendAndRecipient(t *testing.T) {
	for _, tc := range []struct {
		name        string
		complete    bool
		previousJID string
	}{
		{name: "first outbound conversation"},
		{name: "sent event arrives before correlation", complete: true},
		{name: "queue recipient overrides previous conversation", previousJID: "obsolete@s.whatsapp.net"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := postgrestest.NewPool(t)
			if err := Migrate(ctx, pool); err != nil {
				t.Fatal(err)
			}
			owner := createTestOwner(t, pool)
			instance, err := NewInstanceRepository(pool).Create(ctx, model.Instance{
				ID: uuid.New(), Name: "pending-regression", OwnerUserID: &owner.ID,
				Connection: model.InstanceConnection{Status: "disconnected"},
			})
			if err != nil {
				t.Fatal(err)
			}
			messages := NewMessageRepository(pool)
			queue := createTestMessage(t, messages, instance.ID, `{"text":"outbound"}`)
			correlations := NewChatwootMessageRepository(pool)
			if tc.complete {
				if _, err := messages.MarkSent(ctx, queue.ID, "WAMID-BEFORE-CORRELATION", testTerminalEvent(t, queue, "sent", "WAMID-BEFORE-CORRELATION", "")); err != nil {
					t.Fatal(err)
				}
				if promoted, err := correlations.PromotePending(ctx, instance.ID, queue.ID, "WAMID-BEFORE-CORRELATION"); err != nil || promoted {
					t.Fatalf("promotion before correlation = %v, %v", promoted, err)
				}
			}
			stored, err := correlations.Put(ctx, model.ChatwootMessage{
				InstanceID: instance.ID, MessageID: &queue.ID, WAKey: "pending:" + queue.ID.String(),
				ChatwootMessageID: 55, ConversationID: 12, InboxID: 4, ContactSourceID: tc.previousJID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if stored.ContactSourceID != "5511999999999@s.whatsapp.net" {
				t.Errorf("recipient = %q, want queue recipient", stored.ContactSourceID)
			}
			if !tc.complete {
				if _, err := messages.MarkSent(ctx, queue.ID, "WAMID-BEFORE-CORRELATION", testTerminalEvent(t, queue, "sent", "WAMID-BEFORE-CORRELATION", "")); err != nil {
					t.Fatal(err)
				}
				if promoted, err := correlations.PromotePending(ctx, instance.ID, queue.ID, "WAMID-BEFORE-CORRELATION"); err != nil || !promoted {
					t.Fatalf("promotion after correlation = %v, %v", promoted, err)
				}
			}
			got, err := correlations.GetByWAKey(ctx, instance.ID, "WAMID-BEFORE-CORRELATION")
			if err != nil {
				t.Fatalf("completed send lost its correlation: %v", err)
			}
			if got.ContactSourceID != "5511999999999@s.whatsapp.net" || got.ChatwootMessageID != 55 || got.MessageID == nil || *got.MessageID != queue.ID {
				t.Errorf("completed correlation = %+v", got)
			}
			if _, err := correlations.GetByWAKey(ctx, instance.ID, "pending:"+queue.ID.String()); !errors.Is(err, storage.ErrNotFound) {
				t.Errorf("provisional correlation remained after send: %v", err)
			}
		})
	}
}

func TestChatwootPendingPutConcurrentWithSent(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	owner := createTestOwner(t, pool)
	instance, err := NewInstanceRepository(pool).Create(ctx, model.Instance{
		ID: uuid.New(), Name: "concurrent-correlation", OwnerUserID: &owner.ID,
		Connection: model.InstanceConnection{Status: "disconnected"},
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := NewMessageRepository(pool)
	correlations := NewChatwootMessageRepository(pool)
	for i := 0; i < 12; i++ {
		queue := createTestMessage(t, messages, instance.ID, `{"text":"attachment"}`)
		waID := fmt.Sprintf("WAMID-CONCURRENT-%d", i)
		event := testTerminalEvent(t, queue, "sent", waID, "")
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			_, err := correlations.Put(ctx, model.ChatwootMessage{
				InstanceID: instance.ID, MessageID: &queue.ID, WAKey: "pending:" + queue.ID.String(),
				ChatwootMessageID: 55, ConversationID: 12, InboxID: 4,
			})
			results <- err
		}()
		go func() {
			<-start
			if _, err := messages.MarkSent(ctx, queue.ID, waID, event); err != nil {
				results <- err
				return
			}
			_, err := correlations.PromotePending(ctx, instance.ID, queue.ID, waID)
			results <- err
		}()
		close(start)
		for j := 0; j < 2; j++ {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		got, err := correlations.GetByWAKey(ctx, instance.ID, waID)
		if err != nil {
			t.Fatalf("concurrent completion lost correlation %d: %v", i, err)
		}
		if got.ContactSourceID != "5511999999999@s.whatsapp.net" || got.MessageID == nil || *got.MessageID != queue.ID {
			t.Errorf("concurrent correlation %d = %+v", i, got)
		}
		if _, err := correlations.GetByWAKey(ctx, instance.ID, "pending:"+queue.ID.String()); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("concurrent send left provisional key: %v", err)
		}
	}
}
