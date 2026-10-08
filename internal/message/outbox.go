package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/events"
	"wzap/internal/instancelock"
	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/storage"
)

const (
	// defaultOutboxBatchSize is how many due messages a worker claims at once.
	defaultOutboxBatchSize = 10
	// defaultOutboxPollInterval is how long an idle worker waits before
	// claiming again.
	defaultOutboxPollInterval = time.Second
	// defaultOutboxRecoveryInterval is how often stuck messages are requeued.
	defaultOutboxRecoveryInterval = 30 * time.Second
	// stuckThreshold is how long a message may stay in sending before the
	// recovery requeues it.
	stuckThreshold = 5 * time.Minute
	// maxSendRetries is how many transient failures a message survives before
	// it is failed definitively.
	maxSendRetries = 5
	// maxRetryDelay caps the exponential retry backoff.
	maxRetryDelay = 2 * time.Minute
	// messageStatusEventType is the event type of the outbound status events.
	messageStatusEventType = "message.status"
)

// OutboxStore is the message queue persistence consumed by the outbox workers.
type OutboxStore interface {
	// ClaimQueued atomically moves due queued messages to sending.
	ClaimQueued(ctx context.Context, limit int) ([]model.OutboundMessage, error)
	// Terminal outcomes and their status event are committed together.
	// False confirms an already persisted, matching terminal outcome.
	MarkSent(ctx context.Context, id uuid.UUID, whatsAppMessageID string, event model.OutboxEvent) (bool, error)
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, event model.OutboxEvent) (bool, error)
	MarkRetrying(ctx context.Context, id uuid.UUID, errMsg string, nextAttemptAt time.Time) error
	// RequeueStuck moves stale sending messages to queued, excluding active claims.
	RequeueStuck(ctx context.Context, olderThan time.Time, activeIDs []uuid.UUID) (int64, error)
}

// The concrete repository satisfies the outbox contract; the interface
// assertion catches signature drift at build time.
var _ OutboxStore = (storage.MessageRepository)(nil)

// Outbox delivers the queued messages of every instance with a pool of
// workers. Messages of the same instance are serialized by the per-instance
// lock; different instances are delivered in parallel.
type Outbox struct {
	repo      OutboxStore
	manager   session.Manager
	notifier  events.CommittedNotifier
	log       zerolog.Logger
	workers   int
	lock      *instancelock.Locker
	senders   map[string]Sender
	humanizer Humanizer

	batchSize        int
	pollInterval     time.Duration
	recoveryInterval time.Duration
	now              func() time.Time
	sleep            func(ctx context.Context, d time.Duration) error

	mu        sync.Mutex
	lastWarn  map[string]time.Time
	warnEvery time.Duration // default time.Minute, test-overridable field

	// Claims and recovery share this guard, preventing recovery from
	// requeuing a live send or a terminal result awaiting persistence.
	queueMu sync.Mutex
	active  map[uuid.UUID]struct{}
}

// NewOutbox builds the outbox over its dependencies. A nil locker falls back
// to a fresh one and a non-positive worker count to a single worker.
// Humanization stays off unless humanize is true; a nil media resolver leaves
// media messages unsupported.
func NewOutbox(
	repo OutboxStore,
	manager session.Manager,
	notifier events.CommittedNotifier,
	media MediaPathResolver,
	log zerolog.Logger,
	workers int,
	lock *instancelock.Locker,
	humanize bool,
) *Outbox {
	if workers <= 0 {
		workers = 1
	}
	if lock == nil {
		lock = instancelock.New()
	}
	return &Outbox{
		repo:             repo,
		manager:          manager,
		notifier:         notifier,
		log:              log,
		workers:          workers,
		lock:             lock,
		senders:          defaultSenders(media),
		humanizer:        Humanizer{Enabled: humanize},
		batchSize:        defaultOutboxBatchSize,
		pollInterval:     defaultOutboxPollInterval,
		recoveryInterval: defaultOutboxRecoveryInterval,
		now:              time.Now,
		sleep:            sleepContext,
		lastWarn:         make(map[string]time.Time),
		warnEvery:        time.Minute,
		active:           make(map[uuid.UUID]struct{}),
	}
}

func (o *Outbox) warnThrottled(key, msg string, fields func(*zerolog.Event) *zerolog.Event) {
	o.mu.Lock()
	now := time.Now()
	last, ok := o.lastWarn[key]
	if ok && now.Sub(last) < o.warnEvery {
		o.mu.Unlock()
		return
	}
	o.lastWarn[key] = now
	o.mu.Unlock()
	fields(o.log.Warn()).Msg(msg)
}

// Run recovers the messages stuck in sending, then delivers claimed messages
// with workers goroutines until ctx is canceled. It returns after every worker
// stopped; a message being sent when the context is canceled stays in sending
// and is requeued by the recovery once it is older than stuckThreshold.
func (o *Outbox) Run(ctx context.Context) {
	o.StartRecovery(ctx)

	var wg sync.WaitGroup
	for i := 0; i < o.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.worker(ctx)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.recoveryLoop(ctx)
	}()

	<-ctx.Done()
	wg.Wait()
}

// StartRecovery requeues the messages stuck in sending past stuckThreshold.
func (o *Outbox) StartRecovery(ctx context.Context) {
	o.queueMu.Lock()
	activeIDs := make([]uuid.UUID, 0, len(o.active))
	for id := range o.active {
		activeIDs = append(activeIDs, id)
	}
	recovered, err := o.repo.RequeueStuck(ctx, o.now().Add(-stuckThreshold), activeIDs)
	o.queueMu.Unlock()
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.warnThrottled("requeue", "requeue stuck messages", func(e *zerolog.Event) *zerolog.Event {
			return e.Err(err)
		})
		return
	}
	if recovered > 0 {
		o.log.Info().Int64("count", recovered).Msg("requeued stuck messages")
	}
}

// recoveryLoop requeues stuck messages periodically, so a worker that died
// mid-send does not leave the message stuck until the next restart.
func (o *Outbox) recoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(o.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.StartRecovery(ctx)
		}
	}
}

// worker claims and delivers batches until ctx is canceled.
func (o *Outbox) worker(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		claimed, err := o.claimQueued(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			o.warnThrottled("claim", "claim queued messages", func(e *zerolog.Event) *zerolog.Event {
				return e.Err(err)
			})
			if o.wait(ctx, o.pollInterval) != nil {
				return
			}
			continue
		}

		for i := range claimed {
			if ctx.Err() != nil {
				o.releaseClaims(claimed[i:])
				return
			}
			o.process(ctx, claimed[i])
			o.releaseClaims(claimed[i : i+1])
		}

		if len(claimed) < o.batchSize {
			if o.wait(ctx, o.pollInterval) != nil {
				return
			}
		}
	}
}

func (o *Outbox) claimQueued(ctx context.Context) ([]model.OutboundMessage, error) {
	o.queueMu.Lock()
	defer o.queueMu.Unlock()
	claimed, err := o.repo.ClaimQueued(ctx, o.batchSize)
	if err != nil {
		return nil, err
	}
	for _, msg := range claimed {
		o.active[msg.ID] = struct{}{}
	}
	return claimed, nil
}

func (o *Outbox) releaseClaims(claimed []model.OutboundMessage) {
	o.queueMu.Lock()
	defer o.queueMu.Unlock()
	for _, msg := range claimed {
		delete(o.active, msg.ID)
	}
}

// process delivers one claimed message and records its outcome.
func (o *Outbox) process(ctx context.Context, msg model.OutboundMessage) {
	release, err := o.lock.Acquire(ctx, msg.InstanceID)
	if err != nil {
		// Shutdown while waiting for the instance: leave the message in
		// sending for the recovery to requeue.
		return
	}
	defer release()

	sess, ok := o.manager.Get(msg.InstanceID)
	if !ok {
		o.fail(ctx, msg, "session not found for instance "+msg.InstanceID.String())
		return
	}

	sender, ok := o.senders[msg.Type]
	if !ok {
		o.fail(ctx, msg, fmt.Sprintf("unsupported message type %q", msg.Type))
		return
	}

	if err := o.simulatePresence(ctx, sess, msg); err != nil {
		o.handleSendError(ctx, msg, err)
		return
	}

	whatsappID, err := sender.Send(ctx, sess, msg)
	if err != nil {
		o.handleSendError(ctx, msg, err)
		return
	}

	o.complete(ctx, msg, whatsappID)
}

// simulatePresence types before the send when humanization is enabled. Presence
// is best effort: a typing-indicator hiccup must not retry or fail a message
// that was never sent, so it is logged and the send proceeds. A canceled
// context still aborts, leaving the message in sending for the recovery. v1
// keeps no inbound history, so a send cannot be told apart from an open
// conversation; the delay always uses the open-conversation profile, and first
// contact stays available for when that signal exists. The media size arrives
// with the media pipeline, so media messages currently use the base delay only.
func (o *Outbox) simulatePresence(ctx context.Context, sess session.Session, msg model.OutboundMessage) error {
	delay := o.humanizer.PresenceFor(msg.Type, contentTextLen(msg.Type, msg.Payload), 0, false)
	if err := o.humanizer.BeforeSend(ctx, sess, msg.RecipientJID, delay); err != nil {
		if ctx.Err() != nil {
			// Shutdown or timeout: leave the message in sending for the
			// recovery instead of sending it on a dead context.
			return err
		}
		o.log.Warn().Str("message_id", msg.ID.String()).Err(err).Msg("simulate send presence")
	}
	return nil
}

// contentTextLen returns the typing budget of a stored payload: the length of a
// text message, zero for the other types.
func contentTextLen(msgType string, payload []byte) int {
	if msgType != TypeText {
		return 0
	}
	var body textPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return 0
	}
	return len(body.Text)
}

// handleSendError retries a transient failure with exponential backoff while
// fewer than maxSendRetries retries happened, and fails the message
// definitively otherwise. The backoff is computed from the attempts before
// this call because MarkRetrying increments the stored counter itself.
func (o *Outbox) handleSendError(ctx context.Context, msg model.OutboundMessage, cause error) {
	if ctx.Err() != nil {
		// Shutdown: leave the message in sending for the recovery.
		return
	}

	if errors.Is(cause, session.ErrTransient) && msg.Attempts < maxSendRetries {
		nextAttemptAt := o.now().Add(retryDelay(msg.Attempts))
		if err := o.repo.MarkRetrying(ctx, msg.ID, cause.Error(), nextAttemptAt); err != nil {
			o.log.Error().Str("message_id", msg.ID.String()).Err(err).Msg("schedule message retry")
		}
		return
	}

	o.fail(ctx, msg, cause.Error())
}

// complete persists the successful send and its status event together.
func (o *Outbox) complete(ctx context.Context, msg model.OutboundMessage, whatsappID string) {
	o.persistTerminal(ctx, msg, StatusSent, whatsappID, "")
}

// fail persists the definitive failure and its status event together.
func (o *Outbox) fail(ctx context.Context, msg model.OutboundMessage, errMsg string) {
	o.persistTerminal(ctx, msg, StatusFailed, "", errMsg)
}

// persistTerminal retains the send outcome and one envelope while retrying
// only database persistence. The active claim keeps recovery from sending
// it again in this process. Cancellation before confirmation still leaves a
// crash window: a later process can recover a sending message whose external
// send already succeeded.
func (o *Outbox) persistTerminal(ctx context.Context, msg model.OutboundMessage, status, whatsappID, errMsg string) {
	env, err := events.New(messageStatusEventType, msg.InstanceID, messageStatusPayload{
		MessageID:  msg.ID,
		Status:     status,
		WhatsAppID: whatsappID,
		Error:      errMsg,
	})
	if err != nil {
		o.log.Error().Str("message_id", msg.ID.String()).Err(err).Msg("build message status event")
		return
	}
	data, err := json.Marshal(env)
	if err != nil {
		o.log.Error().Str("message_id", msg.ID.String()).Err(err).Msg("marshal message status event")
		return
	}
	subject := events.Subjects.MessageStatus(msg.InstanceID)
	event := model.OutboxEvent{ID: env.EventID, Subject: subject, Envelope: data}

	for attempt := 0; ctx.Err() == nil; attempt++ {
		var changed bool
		if status == StatusSent {
			changed, err = o.repo.MarkSent(ctx, msg.ID, whatsappID, event)
		} else {
			changed, err = o.repo.MarkFailed(ctx, msg.ID, errMsg, event)
		}
		if err == nil {
			// A retry can acknowledge a commit whose response was lost.
			// The store confirms the matching outcome before returning
			// false. Normal repeated calls skip fan-out; a local retry
			// acknowledges and notifies exactly once in this execution.
			if changed || attempt > 0 {
				o.notifier.NotifyCommitted(subject, env)
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrMessageOutcomeConflict) {
			o.log.Error().Str("message_id", msg.ID.String()).Err(err).Msg("persist terminal message outcome")
			return
		}
		o.warnThrottled("terminal", "persist terminal message outcome", func(e *zerolog.Event) *zerolog.Event {
			return e.Str("message_id", msg.ID.String()).Str("event_id", env.EventID.String()).Err(err)
		})
		if o.wait(ctx, retryDelay(attempt)) != nil {
			return
		}
	}
}

// messageStatusPayload is the JSON body of a message.status event. WhatsAppID
// is set on sent and Error on failed.
type messageStatusPayload struct {
	MessageID  uuid.UUID `json:"message_id"`
	Status     string    `json:"status"`
	WhatsAppID string    `json:"whatsapp_id,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// retryDelay returns the delay before the attempt that follows retries
// transient failures: 2^retries seconds, capped at maxRetryDelay.
func retryDelay(retries int) time.Duration {
	delay := time.Second
	for i := 0; i < retries; i++ {
		delay *= 2
		if delay >= maxRetryDelay {
			return maxRetryDelay
		}
	}
	return delay
}

// wait sleeps for d, returning the context error when canceled first.
func (o *Outbox) wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	return o.sleep(ctx, d)
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
