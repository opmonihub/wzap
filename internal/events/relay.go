package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
	"wzap/internal/storage"
)

const (
	defaultRelayBatchSize    = 100
	defaultRelayPollInterval = time.Second
	defaultRelayBackoffBase  = time.Second
	defaultRelayBackoffMax   = 30 * time.Second
)

// Writer enqueues an event into the transactional outbox. Producers call it
// when they record the domain change; the relay publishes the event to the
// broker afterwards.
type Writer interface {
	Write(ctx context.Context, subject string, env Envelope) error
}

// OutboxWriter is the storage-backed Writer.
type OutboxWriter struct {
	outbox storage.EventOutboxRepository
}

var _ Writer = (*OutboxWriter)(nil)

// NewWriter returns a Writer that appends envelopes to outbox.
func NewWriter(outbox storage.EventOutboxRepository) *OutboxWriter {
	return &OutboxWriter{outbox: outbox}
}

// Write serializes env and appends it to the outbox under subject.
func (w *OutboxWriter) Write(ctx context.Context, subject string, env Envelope) error {
	data, err := env.bytes()
	if err != nil {
		return err
	}
	if err := w.outbox.Enqueue(ctx, env.EventID, subject, data); err != nil {
		return fmt.Errorf("enqueue event %s: %w", env.EventID, err)
	}
	return nil
}

// Relay publishes outboxed events to the broker. It is single-worker by
// design: ClaimPending has no claimed state, so a second concurrent relay
// would publish the same rows twice.
type Relay struct {
	outbox    storage.EventOutboxRepository
	publisher Publisher
	log       zerolog.Logger

	batchSize    int
	pollInterval time.Duration
	backoffBase  time.Duration
	backoffMax   time.Duration
	sleep        func(ctx context.Context, d time.Duration) error

	mu        sync.Mutex
	lastWarn  map[string]time.Time
	warnEvery time.Duration // default time.Minute, test-overridable field
}

// NewRelay returns a relay that publishes outbox pending events through
// publisher. The outbox is pending-only: MarkPublished deletes the row on
// success, so pending rows are the only state the relay manages.
func NewRelay(outbox storage.EventOutboxRepository, publisher Publisher, log zerolog.Logger) *Relay {
	return &Relay{
		outbox:       outbox,
		publisher:    publisher,
		log:          log,
		batchSize:    defaultRelayBatchSize,
		pollInterval: defaultRelayPollInterval,
		backoffBase:  defaultRelayBackoffBase,
		backoffMax:   defaultRelayBackoffMax,
		sleep:        sleepContext,
		lastWarn:     make(map[string]time.Time),
		warnEvery:    time.Minute,
	}
}

func (r *Relay) warnThrottled(key, msg string, fields func(*zerolog.Event) *zerolog.Event) {
	r.mu.Lock()
	now := time.Now()
	last, ok := r.lastWarn[key]
	if ok && now.Sub(last) < r.warnEvery {
		r.mu.Unlock()
		return
	}
	r.lastWarn[key] = now
	r.mu.Unlock()
	fields(r.log.Warn()).Msg(msg)
}

// Run publishes pending outbox events until ctx is canceled. The stream is
// ensured on boot and re-checked after publish failures, so the relay recovers
// from a broker that was unavailable at startup.
func (r *Relay) Run(ctx context.Context) {
	failures := 0
	ensured := false

	for {
		if ctx.Err() != nil {
			return
		}

		if !ensured {
			if err := r.publisher.EnsureStream(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				failures++
				r.warnThrottled("stream", "event stream not ready", func(e *zerolog.Event) *zerolog.Event {
					return e.Err(err)
				})
				if r.wait(ctx, r.backoff(failures)) != nil {
					return
				}
				continue
			}
			ensured = true
		}

		pending, err := r.outbox.ClaimPending(ctx, r.batchSize)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			r.warnThrottled("claim", "claim pending events", func(e *zerolog.Event) *zerolog.Event {
				return e.Err(err)
			})
			if r.wait(ctx, r.backoff(failures)) != nil {
				return
			}
			continue
		}

		if len(pending) == 0 {
			failures = 0
			if r.wait(ctx, r.pollInterval) != nil {
				return
			}
			continue
		}

		if err := r.PublishNow(ctx, pending); err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			ensured = false
			r.warnThrottled("publish", "publish pending events", func(e *zerolog.Event) *zerolog.Event {
				return e.Int("pending", len(pending)).Err(err)
			})
			if r.wait(ctx, r.backoff(failures)) != nil {
				return
			}
			continue
		}

		failures = 0
		if len(pending) < r.batchSize {
			if r.wait(ctx, r.pollInterval) != nil {
				return
			}
		}
	}
}

// PublishNow publishes the given pending outbox events. Every event is
// attempted even after a failure. A successful publish deletes the pending
// row (MarkPublished); a failed one stays pending with its attempt recorded,
// so a later claim retries it.
func (r *Relay) PublishNow(ctx context.Context, pending []model.OutboxEvent) error {
	var failures []error
	for _, event := range pending {
		var env Envelope
		if err := json.Unmarshal(event.Envelope, &env); err != nil {
			failures = append(failures, fmt.Errorf("event %s: decode envelope: %w", event.ID, err))
			r.markAttempt(ctx, event.ID, err)
			continue
		}
		if err := r.publisher.Publish(ctx, event.Subject, env); err != nil {
			failures = append(failures, fmt.Errorf("event %s: %w", event.ID, err))
			r.markAttempt(ctx, event.ID, err)
			continue
		}
		if err := r.outbox.MarkPublished(ctx, event.ID); err != nil {
			failures = append(failures, fmt.Errorf("event %s: mark published: %w", event.ID, err))
		}
	}
	return errors.Join(failures...)
}

// markAttempt records a failed publish, leaving the event pending.
func (r *Relay) markAttempt(ctx context.Context, id uuid.UUID, cause error) {
	if err := r.outbox.MarkAttempt(ctx, id, cause.Error()); err != nil {
		r.log.Error().Str("event_id", id.String()).Err(err).Msg("record event attempt")
	}
}

// backoff returns the exponential delay before retry number failures+1,
// capped at backoffMax.
func (r *Relay) backoff(failures int) time.Duration {
	if failures <= 0 {
		return 0
	}
	delay := r.backoffBase
	for i := 1; i < failures; i++ {
		delay *= 2
		if delay >= r.backoffMax {
			return r.backoffMax
		}
	}
	if delay > r.backoffMax {
		return r.backoffMax
	}
	return delay
}

// wait sleeps for d, returning the context error when canceled first.
func (r *Relay) wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	return r.sleep(ctx, d)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
