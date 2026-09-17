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

func TestCallEventPublishesUnifiedOfferEvent(t *testing.T) {
	id := uuid.New()
	writer := &fakeWriter{}
	runtime := NewRuntime(newRuntimeRepo(), writer, nil, nil, "", 1024, zerolog.Nop())
	at := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	raw := json.RawMessage(`{"call":"offer"}`)

	for _, state := range []string{
		session.CallStateOffer, session.CallStateAccept, session.CallStateReject, session.CallStateEnd,
	} {
		runtime.OnCallEvent(context.Background(), session.CallEvent{
			InstanceID: id,
			CallID:     "call-" + state,
			FromJID:    "5511888888888@s.whatsapp.net",
			State:      state,
			IsVideo:    state == session.CallStateOffer,
			Timestamp:  at,
			Raw:        raw,
		})
	}

	if len(writer.subjects) != 4 {
		t.Fatalf("event writes = %v, want one per call state", writer.subjects)
	}
	for i, state := range []string{
		session.CallStateOffer, session.CallStateAccept, session.CallStateReject, session.CallStateEnd,
	} {
		if want := events.Subjects.CallOffer(id); writer.subjects[i] != want {
			t.Errorf("event %d subject = %q, want %q", i, writer.subjects[i], want)
		}
		env := writer.events[i]
		if env.EventVersion != 1 {
			t.Errorf("event %d version = %d, want 1", i, env.EventVersion)
		}
		if env.Type != "call.offer" {
			t.Errorf("event %d type = %q, want the unified call.offer", i, env.Type)
		}
		if env.EventID == uuid.Nil {
			t.Errorf("event %d id is nil, want a stable id for dedupe", i)
		}
		var payload map[string]any
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Fatalf("decode call payload %q: %v", env.Payload, err)
		}
		if payload["state"] != state {
			t.Errorf("event %d payload.state = %v, want %q", i, payload["state"], state)
		}
		if payload["call_id"] != "call-"+state {
			t.Errorf("event %d payload.call_id = %v, want call-%s", i, payload["call_id"], state)
		}
		if payload["from_jid"] != "5511888888888@s.whatsapp.net" {
			t.Errorf("event %d payload.from_jid = %v, want the caller", i, payload["from_jid"])
		}
		if got := string(env.Event); got != string(raw) {
			t.Errorf("event %d envelope event = %s, want the raw bytes verbatim %s", i, got, raw)
		}
	}
}
