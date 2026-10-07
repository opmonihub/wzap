package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/httpapi/instances"
	"wzap/internal/instance"
	"wzap/internal/model"
)

func TestPresence(t *testing.T) {
	t.Run("allowlisted states are published", func(t *testing.T) {
		for _, state := range []string{"composing", "paused", "available", "unavailable"} {
			id := uuid.New()
			svc := &fakeInstanceService{}
			rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
				"/instances/"+id.String()+"/presence",
				`{"chat":"`+testChatDirect+`","state":"`+state+`"}`)

			if rec.Code != http.StatusOK {
				t.Fatalf("state %q: status = %d, want %d (body %q)", state, rec.Code, http.StatusOK, rec.Body.String())
			}
			var payload struct {
				Data instances.PresenceResponse `json:"data"`
			}
			decodeJSON(t, rec.Body.Bytes(), &payload)
			if !payload.Data.Sent {
				t.Errorf("state %q: data.sent = false, want true", state)
			}
			if len(svc.presenceCalls) != 1 {
				t.Fatalf("state %q: SendPresence calls = %d, want 1", state, len(svc.presenceCalls))
			}
			call := svc.presenceCalls[0]
			if call.InstanceID != id || call.ChatJID != testChatDirect || call.State != state {
				t.Errorf("state %q: SendPresence call = %+v, want instance %s chat %s", state, call, id, testChatDirect)
			}
		}
	})

	t.Run("unknown state answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/presence",
			`{"chat":"`+testChatDirect+`","state":"typing-forever"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
			t.Errorf("error code = %q, want %q", code, "unprocessable_entity")
		}
		if len(svc.presenceCalls) != 0 {
			t.Errorf("SendPresence calls = %d, want none on unknown state", len(svc.presenceCalls))
		}
	})

	t.Run("empty chat answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/presence",
			`{"chat":"","state":"composing"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.presenceCalls) != 0 {
			t.Errorf("SendPresence calls = %d, want none on empty chat", len(svc.presenceCalls))
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			sendPresenceFn: func(context.Context, uuid.UUID, string, string) error {
				return instance.ErrNotConnected
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/presence",
			`{"chat":"`+testChatDirect+`","state":"composing"}`)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})

	t.Run("unknown instance answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
				return nil, instance.ErrNotFound
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/presence",
			`{"chat":"`+testChatDirect+`","state":"composing"}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if len(svc.presenceCalls) != 0 {
			t.Errorf("SendPresence calls = %d, want none on unknown instance", len(svc.presenceCalls))
		}
	})
}
