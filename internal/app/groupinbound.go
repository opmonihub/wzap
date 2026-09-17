package app

import (
	"context"
	"time"

	"wzap/internal/events"
	"wzap/internal/session"
	"wzap/internal/webhook"
)

// Event types of the group inbound events.
const (
	groupParticipantsEventType = "group.participants"
	groupInfoEventType         = "group.info"
)

// OnGroupEvent enqueues the group change event: a membership change (members
// joined, left, added or removed) or a metadata change (subject, topic or
// picture). Failures are logged: the sink must not bring the session down.
func (r *Runtime) OnGroupEvent(ctx context.Context, event session.GroupEvent) {
	eventType := groupInfoEventType
	subject := events.Subjects.GroupInfo(event.InstanceID)
	if event.Kind == session.GroupEventParticipants {
		eventType = groupParticipantsEventType
		subject = events.Subjects.GroupParticipants(event.InstanceID)
	}
	payload := groupEventPayload{
		GroupJID:    event.GroupJID,
		ActorJID:    event.ActorJID,
		Affected:    event.Affected,
		Name:        event.Name,
		Description: event.Description,
		Timestamp:   event.Timestamp,
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}
	env, err := events.New(eventType, event.InstanceID, payload)
	if err != nil {
		r.log.Error().Str("instance_id", event.InstanceID.String()).Str("group_jid", event.GroupJID).Err(err).Msg("build group event")
		return
	}
	// The trimmed raw rides the envelope for NATS and webhook alike, like the
	// other inbound events do.
	if trimmed, _ := webhook.CutRawForLimit(event.Raw, r.maxMediaBytes); len(trimmed) > 0 {
		env.Event = trimmed
	}
	if err := r.events.Write(ctx, subject, env); err != nil {
		r.log.Error().Str("instance_id", event.InstanceID.String()).Str("group_jid", event.GroupJID).Err(err).Msg("enqueue group event")
	}
}

// groupEventPayload is the JSON body of an inbound group event: the group,
// the actor of the change (empty when the upstream carries none), the
// affected members on membership changes and the new subject/topic snapshot
// on metadata changes.
type groupEventPayload struct {
	GroupJID    string    `json:"group_jid"`
	ActorJID    string    `json:"actor_jid,omitempty"`
	Affected    []string  `json:"affected,omitempty"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}
