package app

import (
	"context"
	"time"

	"wzap/internal/events"
	"wzap/internal/session"
	"wzap/internal/webhook"
)

// Event types of the rich inbound events.
const (
	pollVoteEventType            = "poll.vote"
	reactionEventType            = "message.reaction"
	interactiveResponseEventType = "interactive.response"
)

// OnPollVote enqueues the vote event of a poll message. Failures are logged:
// the sink must not bring the session down.
func (r *Runtime) OnPollVote(ctx context.Context, vote session.PollVote) {
	payload := pollVotePayload{
		FromJID:             vote.SenderJID,
		ChatJID:             vote.ChatJID,
		IsGroup:             vote.IsGroup,
		PollMessageID:       vote.PollMessageID,
		SelectedOptionIDs:   vote.SelectedOptionIDs,
		SelectedOptionNames: vote.SelectedOptionNames,
		Timestamp:           vote.Timestamp,
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}
	env, err := events.New(pollVoteEventType, vote.InstanceID, payload)
	if err != nil {
		r.log.Error().Str("instance_id", vote.InstanceID.String()).Str("poll_message_id", vote.PollMessageID).Err(err).Msg("build poll vote event")
		return
	}
	// The trimmed raw rides the envelope for NATS and webhook alike, like the
	// inbound message events do.
	if trimmed, _ := webhook.CutRawForLimit(vote.Raw, r.maxMediaBytes); len(trimmed) > 0 {
		env.Event = trimmed
	}
	if err := r.events.Write(ctx, events.Subjects.PollVote(vote.InstanceID), env); err != nil {
		r.log.Error().Str("instance_id", vote.InstanceID.String()).Str("poll_message_id", vote.PollMessageID).Err(err).Msg("enqueue poll vote event")
	}
}

// OnReaction enqueues the reaction event of a previous message. An empty
// emoji is the removal on the same target and travels in the same event.
// Failures are logged: the sink must not bring the session down.
func (r *Runtime) OnReaction(ctx context.Context, reaction session.Reaction) {
	payload := reactionPayload{
		FromJID:         reaction.SenderJID,
		ChatJID:         reaction.ChatJID,
		IsGroup:         reaction.IsGroup,
		TargetMessageID: reaction.MessageID,
		Emoji:           reaction.Emoji,
		Timestamp:       reaction.Timestamp,
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}
	env, err := events.New(reactionEventType, reaction.InstanceID, payload)
	if err != nil {
		r.log.Error().Str("instance_id", reaction.InstanceID.String()).Str("target_message_id", reaction.MessageID).Err(err).Msg("build reaction event")
		return
	}
	if trimmed, _ := webhook.CutRawForLimit(reaction.Raw, r.maxMediaBytes); len(trimmed) > 0 {
		env.Event = trimmed
	}
	if err := r.events.Write(ctx, events.Subjects.Reaction(reaction.InstanceID), env); err != nil {
		r.log.Error().Str("instance_id", reaction.InstanceID.String()).Str("target_message_id", reaction.MessageID).Err(err).Msg("enqueue reaction event")
	}
}

// OnInteractiveResponse enqueues the unified answer event of an interactive
// message (buttons, list or native flow). Failures are logged: the sink must
// not bring the session down.
func (r *Runtime) OnInteractiveResponse(ctx context.Context, response session.InteractiveResponse) {
	payload := interactiveResponsePayload{
		FromJID:    response.SenderJID,
		ChatJID:    response.ChatJID,
		IsGroup:    response.IsGroup,
		MessageID:  response.MessageID,
		Source:     response.Source,
		SelectedID: response.SelectedID,
		Title:      response.Title,
		Timestamp:  response.Timestamp,
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}
	env, err := events.New(interactiveResponseEventType, response.InstanceID, payload)
	if err != nil {
		r.log.Error().Str("instance_id", response.InstanceID.String()).Str("message_id", response.MessageID).Err(err).Msg("build interactive response event")
		return
	}
	if trimmed, _ := webhook.CutRawForLimit(response.Raw, r.maxMediaBytes); len(trimmed) > 0 {
		env.Event = trimmed
	}
	if err := r.events.Write(ctx, events.Subjects.InteractiveResponse(response.InstanceID), env); err != nil {
		r.log.Error().Str("instance_id", response.InstanceID.String()).Str("message_id", response.MessageID).Err(err).Msg("enqueue interactive response event")
	}
}

// pollVotePayload is the JSON body of an inbound poll vote event: the poll
// being voted on, the voter and the selected option ids (hex of the wire
// hashes) with best-effort names (empty when the wire carried hashes only).
type pollVotePayload struct {
	FromJID             string    `json:"from_jid"`
	ChatJID             string    `json:"chat_jid"`
	IsGroup             bool      `json:"is_group"`
	PollMessageID       string    `json:"poll_message_id"`
	SelectedOptionIDs   []string  `json:"selected_option_ids,omitempty"`
	SelectedOptionNames []string  `json:"selected_option_names,omitempty"`
	Timestamp           time.Time `json:"timestamp"`
}

// reactionPayload is the JSON body of an inbound reaction event: the reacted
// message id and the emoji, empty when the reaction was removed.
type reactionPayload struct {
	FromJID         string    `json:"from_jid"`
	ChatJID         string    `json:"chat_jid"`
	IsGroup         bool      `json:"is_group"`
	TargetMessageID string    `json:"target_message_id"`
	Emoji           string    `json:"emoji"`
	Timestamp       time.Time `json:"timestamp"`
}

// interactiveResponsePayload is the JSON body of an inbound interactive
// answer event: the unified source (buttons, list or native_flow), the
// selected id (button id, list row id or flow name) and the display title.
type interactiveResponsePayload struct {
	FromJID    string    `json:"from_jid"`
	ChatJID    string    `json:"chat_jid"`
	IsGroup    bool      `json:"is_group"`
	MessageID  string    `json:"message_id"`
	Source     string    `json:"source"`
	SelectedID string    `json:"selected_id"`
	Title      string    `json:"title,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}
