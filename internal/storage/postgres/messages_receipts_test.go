package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/events"
	"wzap/internal/message"
	"wzap/internal/model"
	"wzap/internal/session"
)

// Removing instance_id from the receipt lookup must change the second
// instance's milestones and leak its WhatsApp IDs into the first one's event.
func TestReceiptsIsolateMessagesAndEventByInstance(t *testing.T) {
	for _, status := range []string{"delivered", "read", "played"} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			pool := newTestPool(t)
			instances := NewInstanceRepository(pool)
			messages := NewMessageRepository(pool)
			outbox := NewEventOutboxRepository(pool)
			first := createTestInstance(t, pool, instances, "receipt-first", "")
			second := createTestInstance(t, pool, instances, "receipt-second", "")
			create := func(instanceID uuid.UUID, waID string) *model.OutboundMessage {
				t.Helper()
				stored, err := messages.Create(ctx, model.OutboundMessage{
					ID: uuid.New(), InstanceID: instanceID, Type: "text",
					RecipientJID: "synthetic@s.whatsapp.net", Payload: []byte(`{"text":"receipt"}`),
					Status: "sent", WhatsAppMessageID: waID,
				})
				if err != nil {
					t.Fatal(err)
				}
				return stored
			}
			target := create(first.ID, "shared-wa-id")
			other := create(second.ID, "shared-wa-id")
			foreign := create(second.ID, "foreign-wa-id")
			at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
			projector := message.NewReceipts(messages, events.NewWriter(outbox), 1<<20)
			if err := projector.Apply(ctx, session.Receipt{
				InstanceID: first.ID, MessageIDs: []string{"shared-wa-id", "foreign-wa-id", "absent-wa-id"},
				Status: status, Timestamp: at,
			}); err != nil {
				t.Fatal(err)
			}
			got, err := messages.Get(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.DeliveredAt == nil || !got.DeliveredAt.Equal(at) {
				t.Errorf("target delivered_at = %v, want %s", got.DeliveredAt, at)
			}
			if status != "delivered" && (got.ReadAt == nil || !got.ReadAt.Equal(at)) {
				t.Errorf("target read_at = %v, want %s", got.ReadAt, at)
			}
			for _, untouched := range []*model.OutboundMessage{other, foreign} {
				got, err := messages.Get(ctx, untouched.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.DeliveredAt != nil || got.ReadAt != nil {
					t.Errorf("foreign instance milestones = %v/%v, want nil", got.DeliveredAt, got.ReadAt)
				}
			}
			pending, err := outbox.ClaimPending(ctx, 10)
			if err != nil || len(pending) != 1 {
				t.Fatalf("pending = %+v, err = %v, want one receipt event", pending, err)
			}
			var envelope events.Envelope
			if err := json.Unmarshal(pending[0].Envelope, &envelope); err != nil {
				t.Fatal(err)
			}
			var payload struct {
				MessageIDs []string `json:"message_ids"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if envelope.InstanceID != first.ID || pending[0].Subject != events.Subjects.Receipt(first.ID) {
				t.Errorf("receipt event has wrong instance/subject: %+v / %s", envelope, pending[0].Subject)
			}
			if len(payload.MessageIDs) != 1 || payload.MessageIDs[0] != "shared-wa-id" {
				t.Errorf("receipt message_ids = %v, want [shared-wa-id]", payload.MessageIDs)
			}
			if err := outbox.MarkPublished(ctx, envelope.EventID); err != nil {
				t.Fatal(err)
			}
			if err := projector.Apply(ctx, session.Receipt{
				InstanceID: first.ID, MessageIDs: []string{"foreign-wa-id", "absent-wa-id"}, Status: status, Timestamp: at,
			}); err != nil {
				t.Fatal(err)
			}
			pending, err = outbox.ClaimPending(ctx, 10)
			if err != nil || len(pending) != 0 {
				t.Errorf("foreign-only receipt pending = %+v, err = %v, want no event", pending, err)
			}
		})
	}
}
