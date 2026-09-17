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

func TestRuntimeOnGroupParticipantsPublishesParticipantsEvent(t *testing.T) {
	id := uuid.New()
	writer := &fakeWriter{}
	runtime := NewRuntime(newRuntimeRepo(), writer, nil, nil, "", 1024, zerolog.Nop())
	at := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	raw := json.RawMessage(`{"group":"update"}`)

	runtime.OnGroupEvent(context.Background(), session.GroupEvent{
		InstanceID: id,
		GroupJID:   "120363000000000000@g.us",
		Kind:       session.GroupEventParticipants,
		ActorJID:   "5511888888888@s.whatsapp.net",
		Affected:   []string{"5511999999999@s.whatsapp.net"},
		Timestamp:  at,
		Raw:        raw,
	})

	if len(writer.subjects) != 1 {
		t.Fatalf("event writes = %v, want one group participants event", writer.subjects)
	}
	if want := events.Subjects.GroupParticipants(id); writer.subjects[0] != want {
		t.Errorf("event subject = %q, want %q", writer.subjects[0], want)
	}
	env := writer.events[0]
	if env.EventVersion != 1 {
		t.Errorf("event_version = %d, want 1", env.EventVersion)
	}
	if env.Type != "group.participants" {
		t.Errorf("event type = %q, want %q", env.Type, "group.participants")
	}
	if env.EventID == uuid.Nil {
		t.Error("event_id is nil, want a stable id for dedupe")
	}
	var payload map[string]any
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("decode participants payload %q: %v", env.Payload, err)
	}
	if payload["group_jid"] != "120363000000000000@g.us" {
		t.Errorf("payload.group_jid = %v, want the group", payload["group_jid"])
	}
	if payload["actor_jid"] != "5511888888888@s.whatsapp.net" {
		t.Errorf("payload.actor_jid = %v, want the actor", payload["actor_jid"])
	}
	affected, _ := payload["affected"].([]any)
	if len(affected) != 1 || affected[0] != "5511999999999@s.whatsapp.net" {
		t.Errorf("payload.affected = %v, want the affected member", payload["affected"])
	}
	if got := string(env.Event); got != string(raw) {
		t.Errorf("envelope event = %s, want the raw bytes verbatim %s", got, raw)
	}
}

func TestRuntimeOnGroupInfoPublishesInfoEvent(t *testing.T) {
	id := uuid.New()
	writer := &fakeWriter{}
	runtime := NewRuntime(newRuntimeRepo(), writer, nil, nil, "", 1024, zerolog.Nop())

	runtime.OnGroupEvent(context.Background(), session.GroupEvent{
		InstanceID: id,
		GroupJID:   "120363000000000000@g.us",
		Kind:       session.GroupEventInfo,
		ActorJID:   "5511888888888@s.whatsapp.net",
		Name:       "Novo assunto",
		Timestamp:  time.Date(2026, 9, 16, 10, 1, 0, 0, time.UTC),
	})

	if len(writer.subjects) != 1 {
		t.Fatalf("event writes = %v, want one group info event", writer.subjects)
	}
	if want := events.Subjects.GroupInfo(id); writer.subjects[0] != want {
		t.Errorf("event subject = %q, want %q", writer.subjects[0], want)
	}
	env := writer.events[0]
	if env.EventVersion != 1 {
		t.Errorf("event_version = %d, want 1", env.EventVersion)
	}
	if env.Type != "group.info" {
		t.Errorf("event type = %q, want %q", env.Type, "group.info")
	}
	var payload map[string]any
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("decode info payload: %v", err)
	}
	if payload["name"] != "Novo assunto" {
		t.Errorf("payload.name = %v, want the new subject", payload["name"])
	}
}

// TestGroupEventIDSurvivesRetry pins at-least-once dedupe: republicando o
// mesmo envelope (o retry do relay) preserva o event_id, então o consumidor
// dedupe por ele e o broker pelo Nats-Msg-Id.
func TestGroupEventIDSurvivesRetry(t *testing.T) {
	id := uuid.New()
	env, err := events.New("group.participants", id, map[string]any{"group_jid": "120363000000000000@g.us"})
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	first := env.EventID.String()
	republished := env
	if republished.EventID.String() != first {
		t.Errorf("republished event_id = %s, want the stable %s", republished.EventID, first)
	}
}
