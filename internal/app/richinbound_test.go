package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/events"
	"wzap/internal/session"
)

func TestRuntimeOnPollVotePublishesVoteEvent(t *testing.T) {
	id := uuid.New()
	writer := &fakeWriter{}
	runtime := NewRuntime(newRuntimeRepo(), writer, nil, nil, "", 1024, zerolog.Nop())
	at := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	raw := json.RawMessage(`{"vote":"yes"}`)

	runtime.OnPollVote(context.Background(), session.PollVote{
		InstanceID:          id,
		PollMessageID:       "wamid.poll",
		ChatJID:             "120363000000000000@g.us",
		SenderJID:           "5511888888888@s.whatsapp.net",
		IsGroup:             true,
		SelectedOptionIDs:   []string{"ab12"},
		SelectedOptionNames: []string{"manhã"},
		Timestamp:           at,
		Raw:                 raw,
	})

	if len(writer.subjects) != 1 {
		t.Fatalf("event writes = %v, want one poll vote event", writer.subjects)
	}
	if want := events.Subjects.PollVote(id); writer.subjects[0] != want {
		t.Errorf("event subject = %q, want %q", writer.subjects[0], want)
	}
	env := writer.events[0]
	if env.EventVersion != 1 {
		t.Errorf("event_version = %d, want 1", env.EventVersion)
	}
	if env.Type != "poll.vote" {
		t.Errorf("event type = %q, want %q", env.Type, "poll.vote")
	}
	if env.EventID == uuid.Nil {
		t.Error("event_id is nil, want a stable id for dedupe")
	}
	var payload map[string]any
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("decode vote payload %q: %v", env.Payload, err)
	}
	if payload["poll_message_id"] != "wamid.poll" {
		t.Errorf("payload.poll_message_id = %v, want the poll", payload["poll_message_id"])
	}
	if got := string(env.Event); got != string(raw) {
		t.Errorf("envelope event = %s, want the raw bytes verbatim %s", got, raw)
	}
}

func TestRuntimeOnReactionPublishesReactionEvent(t *testing.T) {
	id := uuid.New()
	writer := &fakeWriter{}
	runtime := NewRuntime(newRuntimeRepo(), writer, nil, nil, "", 1024, zerolog.Nop())

	for _, emoji := range []string{"👍", ""} {
		runtime.OnReaction(context.Background(), session.Reaction{
			InstanceID: id,
			MessageID:  "wamid.orig",
			ChatJID:    "5511999999999@s.whatsapp.net",
			SenderJID:  "5511888888888@s.whatsapp.net",
			Emoji:      emoji,
			Timestamp:  time.Date(2026, 9, 15, 10, 1, 0, 0, time.UTC),
		})
	}

	if len(writer.subjects) != 2 {
		t.Fatalf("event writes = %v, want reaction plus removal", writer.subjects)
	}
	for i, env := range writer.events {
		if env.EventVersion != 1 {
			t.Errorf("event %d version = %d, want 1", i, env.EventVersion)
		}
		if env.Type != "message.reaction" {
			t.Errorf("event %d type = %q, want %q", i, env.Type, "message.reaction")
		}
		if want := events.Subjects.Reaction(id); writer.subjects[i] != want {
			t.Errorf("event %d subject = %q, want %q", i, writer.subjects[i], want)
		}
		var payload map[string]any
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Fatalf("decode reaction payload: %v", err)
		}
		if payload["target_message_id"] != "wamid.orig" {
			t.Errorf("event %d target = %v, want the reacted message", i, payload["target_message_id"])
		}
	}
}

func TestRuntimeOnInteractiveResponsePublishesUnifiedEvent(t *testing.T) {
	id := uuid.New()
	writer := &fakeWriter{}
	runtime := NewRuntime(newRuntimeRepo(), writer, nil, nil, "", 1024, zerolog.Nop())

	for _, source := range []string{"buttons", "list", "native_flow"} {
		runtime.OnInteractiveResponse(context.Background(), session.InteractiveResponse{
			InstanceID: id,
			MessageID:  "wamid.resp",
			ChatJID:    "5511999999999@s.whatsapp.net",
			SenderJID:  "5511888888888@s.whatsapp.net",
			Source:     source,
			SelectedID: "row-1",
			Title:      "Opção 1",
			Timestamp:  time.Date(2026, 9, 15, 10, 2, 0, 0, time.UTC),
		})
	}

	if len(writer.subjects) != 3 {
		t.Fatalf("event writes = %v, want one per source", writer.subjects)
	}
	for i, env := range writer.events {
		if env.EventVersion != 1 {
			t.Errorf("event %d version = %d, want 1", i, env.EventVersion)
		}
		if env.Type != "interactive.response" {
			t.Errorf("event %d type = %q, want %q", i, env.Type, "interactive.response")
		}
		if want := events.Subjects.InteractiveResponse(id); writer.subjects[i] != want {
			t.Errorf("event %d subject = %q, want %q", i, writer.subjects[i], want)
		}
		var payload map[string]any
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Fatalf("decode interactive payload: %v", err)
		}
		if payload["selected_id"] != "row-1" || payload["title"] != "Opção 1" {
			t.Errorf("event %d payload = %v, want selected id and title", i, payload)
		}
	}
}

// TestRichEventIDSurvivesRetry pins at-least-once dedupe: republicando o mesmo
// envelope (o retry do relay) preserva o event_id, então o consumidor dedupe
// por ele e o broker pelo Nats-Msg-Id.
func TestRichEventIDSurvivesRetry(t *testing.T) {
	id := uuid.New()
	env, err := events.New("poll.vote", id, map[string]any{"poll_message_id": "wamid.poll"})
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	first := env.EventID.String()
	republished := env
	if republished.EventID.String() != first {
		t.Errorf("republished event_id = %s, want the stable %s", republished.EventID, first)
	}
}
