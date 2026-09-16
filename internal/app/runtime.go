// Package app wires the wzap runtime: the session event sink that projects
// connection state into the database and the outbox.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/events"
	"wzap/internal/message"
	"wzap/internal/session"
	"wzap/internal/storage"
)

// connectionEventType is the event type of the instance connection events.
const connectionEventType = "connection"

// ReceiptApplier projects an inbound receipt into the stored messages and the
// event outbox. *message.Receipts implements it.
type ReceiptApplier interface {
	Apply(ctx context.Context, receipt session.Receipt) error
}

// The concrete projector satisfies the runtime contract; the assertion catches
// signature drift at build time.
var _ ReceiptApplier = (*message.Receipts)(nil)

// Runtime is the session.EventSink of the service. Connection changes are
// projected into the instances row and enqueued as events in the outbox;
// receipts are projected into the message rows; inbound messages have their
// media stored and are enqueued as events.
type Runtime struct {
	instances     storage.InstanceRepository
	events        events.Writer
	receipts      ReceiptApplier
	media         MediaStore
	publicURL     string
	maxMediaBytes int64
	log           zerolog.Logger

	// mu serializes connection projections so the persisted transitions and
	// the events describing them keep a consistent order. The repository
	// update itself is targeted, so it cannot resurrect stale columns.
	mu sync.Mutex
}

var _ session.EventSink = (*Runtime)(nil)

// NewRuntime builds the runtime over the instance repository, the outbox
// writer, the receipt projector and the inbound media store. publicURL is the
// base of the media download URLs and maxMediaBytes the largest inbound media
// stored before it is omitted. A nil receipt projector makes OnReceipt a
// no-op and a nil media store marks every inbound media as omitted.
func NewRuntime(
	instances storage.InstanceRepository,
	writer events.Writer,
	receipts ReceiptApplier,
	mediaStore MediaStore,
	publicURL string,
	maxMediaBytes int64,
	log zerolog.Logger,
) *Runtime {
	return &Runtime{
		instances:     instances,
		events:        writer,
		receipts:      receipts,
		media:         mediaStore,
		publicURL:     publicURL,
		maxMediaBytes: maxMediaBytes,
		log:           log,
	}
}

// OnMessage stores the media of an inbound message when it has one and
// enqueues its event. Failures are logged: the sink must not bring the session
// down and a message without its media is still delivered to the consumers.
func (r *Runtime) OnMessage(ctx context.Context, msg session.InboundMessage) {
	if err := r.handleInbound(ctx, msg); err != nil {
		r.log.Error().Str("instance_id", msg.InstanceID.String()).Str("message_id", msg.MessageID).Err(err).Msg("handle inbound message")
	}
}

// OnReceipt applies a delivery/read receipt to the stored messages and
// enqueues its event. A failure is logged: the receipt is an observation and
// the next one may still apply.
func (r *Runtime) OnReceipt(ctx context.Context, receipt session.Receipt) {
	if r.receipts == nil {
		return
	}
	if err := r.receipts.Apply(ctx, receipt); err != nil {
		r.log.Error().Str("instance_id", receipt.InstanceID.String()).Err(err).Msg("apply receipt")
	}
}

// OnConnection projects a connection change into the instance row and enqueues
// the connection event. When the change cannot be recorded the event is
// skipped, so consumers are not told about a state the status endpoint does
// not report.
func (r *Runtime) OnConnection(ctx context.Context, instanceID uuid.UUID, status session.Status, jid, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// The JID itself is never logged: only whether it is present.
	r.log.Debug().Str("instance_id", instanceID.String()).Str("status", string(status)).Bool("jid_present", jid != "").Str("reason", reason).Msg("connection change received")

	if err := applyConnection(ctx, r.log, r.instances, instanceID, status, jid, reason); err != nil {
		r.log.Error().Str("instance_id", instanceID.String()).Str("status", string(status)).Err(err).Msg("record connection change")
		if errors.Is(err, storage.ErrNotFound) {
			r.log.Warn().Str("instance_id", instanceID.String()).Str("status", string(status)).Msg("skip connection event for unknown instance")
		}
		return
	}

	env, err := events.New(connectionEventType, instanceID, connectionPayload{
		Status:      string(status),
		WhatsAppJID: jid,
		Reason:      reason,
	})
	if err != nil {
		r.log.Error().Str("instance_id", instanceID.String()).Err(err).Msg("build connection event")
		return
	}
	if err := r.events.Write(ctx, events.Subjects.Connection(instanceID), env); err != nil {
		r.log.Error().Str("instance_id", instanceID.String()).Err(err).Msg("enqueue connection event")
	}
}

// applyConnection stores the connection state of instanceID with a targeted
// update, so a concurrent PATCH cannot be overwritten with stale columns. The
// JID is kept while it is unknown so a transient failure keeps the paired
// identity, and last_connected_at is stamped on every transition to connected.
func applyConnection(ctx context.Context, log zerolog.Logger, instances storage.InstanceRepository, instanceID uuid.UUID, status session.Status, jid, reason string) error {
	var connectedAt *time.Time
	if status == session.StatusConnected {
		now := time.Now().UTC()
		connectedAt = &now
	}

	if err := instances.SetConnectionState(ctx, instanceID, string(status), jid, reason, connectedAt); err != nil {
		return fmt.Errorf("set instance connection: %w", err)
	}
	log.Debug().Str("instance_id", instanceID.String()).Str("status", string(status)).Bool("connected_at_set", connectedAt != nil).Msg("connection state recorded")
	return nil
}

// connectionPayload is the JSON body of a connection event.
type connectionPayload struct {
	Status      string `json:"status"`
	WhatsAppJID string `json:"whatsapp_jid,omitempty"`
	Reason      string `json:"reason,omitempty"`
}
