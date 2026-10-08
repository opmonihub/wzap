package message

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Dropping a failed terminal write must leave this test without a persisted
// outcome. Retrying process instead of persistence would add a second send.
func TestOutboxRetriesTerminalPersistenceWithoutResending(t *testing.T) {
	for _, status := range []string{StatusSent, StatusFailed} {
		t.Run(status, func(t *testing.T) {
			fixture := newOutboxFixture()
			msg := textMessage(fixture.instance, 0)
			persistErr := errors.New("synthetic persistence failure")
			if status == StatusSent {
				fixture.repo.markSentErr = persistErr
			} else {
				fixture.session.SendErr = errors.New("definitive send failure")
				fixture.repo.markFailedErr = persistErr
			}
			waits := 0
			fixture.outbox.sleep = func(_ context.Context, _ time.Duration) error {
				waits++
				if len(fixture.writer.written()) != 0 {
					t.Error("status fan-out occurred before terminal persistence")
				}
				if len(fixture.repo.sentIDs()) != 0 || len(fixture.repo.failedMessages()) != 0 {
					t.Error("failed persistence confirmed a terminal state")
				}
				fixture.repo.markSentErr = nil
				fixture.repo.markFailedErr = nil
				return nil
			}
			fixture.outbox.process(context.Background(), msg)
			if waits != 1 {
				t.Errorf("persistence retry waits = %d, want 1", waits)
			}
			if status == StatusSent && fixture.repo.sentIDs()[msg.ID] != "fake-wamid-1" {
				t.Errorf("sent result = %v, want retained WhatsApp ID", fixture.repo.sentIDs())
			}
			if status == StatusFailed && fixture.repo.failedMessages()[msg.ID] != "definitive send failure" {
				t.Errorf("failed result = %v, want retained send failure", fixture.repo.failedMessages())
			}
			if got := len(fixture.session.SendCalls()); got != 1 {
				t.Errorf("WhatsApp sends = %d, want 1", got)
			}
			written := fixture.writer.written()
			if len(written) != 1 {
				t.Fatalf("committed status events = %d, want 1", len(written))
			}
			if payload := envelopePayload(t, written[0].envelope); payload["status"] != status {
				t.Errorf("event status = %v, want %s", payload["status"], status)
			}
			attempts := fixture.repo.terminalAttempts()
			if len(attempts) != 2 || attempts[0].ID == uuid.Nil || attempts[0].ID != attempts[1].ID || string(attempts[0].Envelope) != string(attempts[1].Envelope) {
				t.Errorf("terminal persistence attempts = %+v, want two attempts with identical event ID and envelope", attempts)
			}
			if len(attempts) == 2 && written[0].envelope.EventID != attempts[0].ID {
				t.Error("post-commit fan-out used another event ID")
			}
		})
	}
}

func TestOutboxAcknowledgesUncertainTerminalCommitOnce(t *testing.T) {
	for _, status := range []string{StatusSent, StatusFailed} {
		t.Run(status, func(t *testing.T) {
			fixture := newOutboxFixture()
			msg := textMessage(fixture.instance, 0)
			fixture.repo.markCommitErr = errors.New("connection lost after commit")
			if status == StatusFailed {
				fixture.session.SendErr = errors.New("definitive send failure")
			}
			waits := 0
			fixture.outbox.sleep = func(context.Context, time.Duration) error {
				waits++
				if len(fixture.writer.written()) != 0 {
					t.Error("status fan-out occurred before commit acknowledgement")
				}
				return nil
			}
			fixture.outbox.process(context.Background(), msg)
			if waits != 1 || len(fixture.session.SendCalls()) != 1 || len(fixture.writer.written()) != 1 {
				t.Errorf("uncertain commit: waits=%d, sends=%d, status fan-outs=%d; want 1 each", waits, len(fixture.session.SendCalls()), len(fixture.writer.written()))
			}
			attempts := fixture.repo.terminalAttempts()
			if len(attempts) != 2 || attempts[0].ID == uuid.Nil || attempts[0].ID != attempts[1].ID {
				t.Errorf("uncertain-commit event attempts = %+v, want two with the same ID", attempts)
			}
		})
	}
}

func TestOutboxRecoveryExcludesActiveClaimDuringTerminalRetry(t *testing.T) {
	fixture := newOutboxFixture(textMessage(uuid.Nil, 0))
	msgID := fixture.repo.queue[0].ID
	fixture.repo.markSentErr = errors.New("synthetic persistence failure")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := 0
	fixture.outbox.sleep = func(ctx context.Context, _ time.Duration) error {
		waits++
		if waits == 1 {
			fixture.outbox.StartRecovery(ctx)
			excluded := fixture.repo.recoveryExclusions()
			last := excluded[len(excluded)-1]
			if len(last) != 1 || last[0] != msgID {
				t.Errorf("recovery during terminal retry excluded %v, want active message %s", last, msgID)
			}
			fixture.repo.markSentErr = nil
			return nil
		}
		cancel()
		return ctx.Err()
	}
	fixture.outbox.Run(ctx)
	if len(fixture.session.SendCalls()) != 1 || fixture.repo.sentIDs()[msgID] != "fake-wamid-1" || len(fixture.writer.written()) != 1 {
		t.Errorf("active persistence retry lost/duplicated send outcome: sends=%d, sent=%v, events=%d", len(fixture.session.SendCalls()), fixture.repo.sentIDs(), len(fixture.writer.written()))
	}
}
