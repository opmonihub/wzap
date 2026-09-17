package app

import (
	"context"
	"time"

	"wzap/internal/events"
	"wzap/internal/session"
	"wzap/internal/webhook"
)

// Event types of the call inbound events. The offer, the accept, the reject
// and the end travel unified as call.offer with their state field (Ruling
// D2): the companion never initiates calls, it only observes them.
const callOfferEventType = "call.offer"

// OnCallEvent enqueues the unified call change event. Failures are logged:
// the sink must not bring the session down.
func (r *Runtime) OnCallEvent(ctx context.Context, event session.CallEvent) {
	payload := callEventPayload{
		CallID:    event.CallID,
		FromJID:   event.FromJID,
		State:     event.State,
		IsVideo:   event.IsVideo,
		Timestamp: event.Timestamp,
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}
	env, err := events.New(callOfferEventType, event.InstanceID, payload)
	if err != nil {
		r.log.Error().Str("instance_id", event.InstanceID.String()).Str("call_id", event.CallID).Err(err).Msg("build call event")
		return
	}
	// The trimmed raw rides the envelope for NATS and webhook alike, like the
	// other inbound events do.
	if trimmed, _ := webhook.CutRawForLimit(event.Raw, r.maxMediaBytes); len(trimmed) > 0 {
		env.Event = trimmed
	}
	if err := r.events.Write(ctx, events.Subjects.CallOffer(event.InstanceID), env); err != nil {
		r.log.Error().Str("instance_id", event.InstanceID.String()).Str("call_id", event.CallID).Err(err).Msg("enqueue call event")
	}
}

// callEventPayload is the JSON body of an inbound call event: the upstream
// call id, the caller and the lifecycle state (offer, accept, reject or
// end). IsVideo is best-effort: true only when the wire named video.
type callEventPayload struct {
	CallID    string    `json:"call_id"`
	FromJID   string    `json:"from_jid"`
	State     string    `json:"state"`
	IsVideo   bool      `json:"is_video"`
	Timestamp time.Time `json:"timestamp"`
}
