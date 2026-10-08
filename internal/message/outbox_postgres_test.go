package message

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"wzap/internal/events"
	"wzap/internal/instancelock"
	"wzap/internal/model"
	"wzap/internal/session/sessiontest"
	"wzap/internal/storage/postgres"
	"wzap/internal/storage/postgres/postgrestest"
)

func postgresOutboxFixture(t *testing.T) (*pgxpool.Pool, *postgres.MessageRepository, *Outbox, *fakeWriter, model.OutboundMessage) {
	t.Helper()
	ctx := context.Background()
	pool := postgrestest.NewPool(t)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	owner, err := postgres.NewUserRepository(pool).Create(ctx, model.User{
		ID: uuid.New(), Email: uuid.NewString() + "@test.example", Role: "user", PasswordHash: "synthetic-test-hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := postgres.NewInstanceRepository(pool).Create(ctx, model.Instance{
		ID: uuid.New(), Name: "atomic-terminal", OwnerUserID: &owner.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewMessageRepository(pool)
	if _, err := repo.Create(ctx, textMessage(instance.ID, 0)); err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.ClaimQueued(ctx, 1)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim = %+v, err = %v", claimed, err)
	}
	manager := sessiontest.New(nil)
	manager.Put(instance.ID, sessiontest.NewSession(instance.ID, nil))
	writer := &fakeWriter{inner: events.NewWriter(postgres.NewEventOutboxRepository(pool))}
	outbox := NewOutbox(repo, manager, writer, nil, zerolog.Nop(), 1, instancelock.New(), false)
	return pool, repo, outbox, writer, claimed[0]
}

// Keeping MarkSent/MarkFailed outside the event transaction would persist a
// terminal message despite either the immediate INSERT or deferred COMMIT failure.
func TestOutboxTerminalPersistenceRollsBackStateAndEvent(t *testing.T) {
	for _, status := range []string{StatusSent, StatusFailed} {
		for _, failure := range []string{"insert", "commit"} {
			t.Run(status+"/"+failure, func(t *testing.T) {
				pool, repo, outbox, writer, msg := postgresOutboxFixture(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var ddl []string
				if failure == "insert" {
					ddl = []string{`ALTER TABLE event_outbox ADD CONSTRAINT reject_terminal_events CHECK (envelope->>'type' <> 'message.status')`}
				} else {
					ddl = []string{
						`CREATE FUNCTION reject_terminal_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic commit failure'; END; $$`,
						`CREATE CONSTRAINT TRIGGER reject_terminal_commit AFTER INSERT ON event_outbox DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_terminal_commit()`,
					}
				}
				for _, statement := range ddl {
					if _, err := pool.Exec(ctx, statement); err != nil {
						t.Fatal(err)
					}
				}
				outbox.sleep = func(ctx context.Context, _ time.Duration) error {
					cancel()
					return ctx.Err()
				}
				if status == StatusSent {
					outbox.complete(ctx, msg, "atomic-wa-id")
				} else {
					outbox.fail(ctx, msg, "synthetic definitive failure")
				}
				stored, err := repo.Get(context.Background(), msg.ID)
				if err != nil {
					t.Fatal(err)
				}
				if stored.Status != StatusSending || stored.WhatsAppMessageID != "" || stored.LastError != "" {
					t.Errorf("state after %s failure = %s/%q/%q, want sending with no terminal outcome", failure, stored.Status, stored.WhatsAppMessageID, stored.LastError)
				}
				pending, err := postgres.NewEventOutboxRepository(pool).ClaimPending(context.Background(), 10)
				if err != nil || len(pending) != 0 {
					t.Errorf("pending after failure = %d, err = %v, want none", len(pending), err)
				}
				if got := len(writer.written()); got != 0 {
					t.Errorf("status fan-out before commit = %d, want none", got)
				}
			})
		}
	}
}

func TestOutboxTerminalPersistenceCommitsOneStateAndEvent(t *testing.T) {
	for _, status := range []string{StatusSent, StatusFailed} {
		t.Run(status, func(t *testing.T) {
			pool, repo, outbox, writer, msg := postgresOutboxFixture(t)
			ctx := context.Background()
			persist := func() {
				if status == StatusSent {
					outbox.complete(ctx, msg, "atomic-wa-id")
				} else {
					outbox.fail(ctx, msg, "synthetic definitive failure")
				}
			}
			persist()
			stored, err := repo.Get(ctx, msg.ID)
			if err != nil || stored.Status != status {
				t.Fatalf("terminal state = %+v, err = %v, want %s", stored, err, status)
			}
			eventRepo := postgres.NewEventOutboxRepository(pool)
			pending, err := eventRepo.ClaimPending(ctx, 10)
			if err != nil || len(pending) != 1 {
				t.Fatalf("pending terminal events = %d, err = %v, want 1", len(pending), err)
			}
			var envelope events.Envelope
			if err := json.Unmarshal(pending[0].Envelope, &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.InstanceID != msg.InstanceID || pending[0].Subject != events.Subjects.MessageStatus(msg.InstanceID) {
				t.Errorf("wrong instance/subject for terminal event: %s/%s", envelope.InstanceID, pending[0].Subject)
			}
			if payload := envelopePayload(t, envelope); payload["status"] != status || payload["message_id"] != msg.ID.String() {
				t.Errorf("terminal event payload = %v, want status %s and ID %s", payload, status, msg.ID)
			}
			if written := writer.written(); len(written) != 1 || written[0].envelope.EventID != pending[0].ID {
				t.Fatalf("post-commit events = %+v, want the one durable envelope", written)
			}
			persist()
			if got := len(writer.written()); got != 1 {
				t.Errorf("repeated terminal persistence fan-outs = %d, want 1", got)
			}
			if err := eventRepo.MarkPublished(ctx, pending[0].ID); err != nil {
				t.Fatal(err)
			}
			persist()
			pending, err = eventRepo.ClaimPending(ctx, 10)
			if err != nil || len(pending) != 0 || len(writer.written()) != 1 {
				t.Errorf("repeat after publication recreated event/fan-out: pending=%d, fan-out=%d, err=%v", len(pending), len(writer.written()), err)
			}
		})
	}
}

// lostCommitAck keeps the real database transaction and replaces only the
// missing acknowledgement an interrupted database connection can produce.
type lostCommitAck struct {
	OutboxStore
	loseAck  bool
	attempts []model.OutboxEvent
}

func (s *lostCommitAck) MarkSent(ctx context.Context, id uuid.UUID, waID string, event model.OutboxEvent) (bool, error) {
	s.attempts = append(s.attempts, event)
	changed, err := s.OutboxStore.MarkSent(ctx, id, waID, event)
	return s.acknowledge(changed, err)
}

func (s *lostCommitAck) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, event model.OutboxEvent) (bool, error) {
	s.attempts = append(s.attempts, event)
	changed, err := s.OutboxStore.MarkFailed(ctx, id, errMsg, event)
	return s.acknowledge(changed, err)
}

func (s *lostCommitAck) acknowledge(changed bool, err error) (bool, error) {
	if err == nil && changed && s.loseAck {
		s.loseAck = false
		return false, errors.New("synthetic connection loss after commit")
	}
	return changed, err
}

func TestOutboxAcknowledgesPublishedTerminalCommitWithoutReinserting(t *testing.T) {
	for _, status := range []string{StatusSent, StatusFailed} {
		t.Run(status, func(t *testing.T) {
			pool, repo, outbox, writer, msg := postgresOutboxFixture(t)
			store := &lostCommitAck{OutboxStore: repo, loseAck: true}
			outbox.repo = store
			if status == StatusFailed {
				sess, _ := outbox.manager.Get(msg.InstanceID)
				sess.(*sessiontest.FakeSession).SendErr = errors.New("definitive send failure")
			}
			eventRepo := postgres.NewEventOutboxRepository(pool)
			waits := 0
			outbox.sleep = func(ctx context.Context, _ time.Duration) error {
				waits++
				stored, err := repo.Get(ctx, msg.ID)
				if err != nil || stored.Status != status || len(writer.written()) != 0 {
					t.Fatalf("uncertain commit: stored=%+v, err=%v, fan-out=%d", stored, err, len(writer.written()))
				}
				pending, err := eventRepo.ClaimPending(ctx, 10)
				if err != nil || len(pending) != 1 {
					t.Fatalf("uncertain commit lost durable event: pending=%d, err=%v", len(pending), err)
				}
				// The pending-only relay may publish and delete the event
				// before the message worker acknowledges the commit.
				if err := eventRepo.MarkPublished(ctx, pending[0].ID); err != nil {
					t.Fatal(err)
				}
				return nil
			}
			outbox.process(context.Background(), msg)
			sess, _ := outbox.manager.Get(msg.InstanceID)
			if got := len(sess.(*sessiontest.FakeSession).SendCalls()); got != 1 {
				t.Errorf("WhatsApp sends after uncertain commit = %d, want 1", got)
			}
			if waits != 1 || len(store.attempts) != 2 || store.attempts[0].ID != store.attempts[1].ID {
				t.Fatalf("uncertain commit: waits=%d, attempts=%+v; want two persistence attempts sharing an event ID", waits, store.attempts)
			}
			if written := writer.written(); len(written) != 1 || written[0].envelope.EventID != store.attempts[0].ID {
				t.Errorf("uncertain commit fan-out = %+v, want the one original event", written)
			}
			pending, err := eventRepo.ClaimPending(context.Background(), 10)
			if err != nil || len(pending) != 0 {
				t.Errorf("acknowledgement recreated published event: pending=%d, err=%v", len(pending), err)
			}
		})
	}
}
