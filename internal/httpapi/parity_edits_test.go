package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// editMessageServer wires the edit handler at its future route behind the
// production auth chain so the test exercises the same boundary Task 9 will
// register in server.go.
func editMessageServer(t *testing.T, svc InstanceService) *http.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /instances/{id}/messages/edit", handleEditMessage(svc, zerolog.Nop()))
	return &http.Server{Handler: RequestID(Authenticate(testToken, nil, "")(mux))}
}

func TestEditMessage(t *testing.T) {
	id := uuid.New()
	svc := &fakeInstanceService{
		editMessageFn: func(_ context.Context, instanceID uuid.UUID, chat, msgID, text string) (string, error) {
			if instanceID != id {
				t.Errorf("EditMessage instance = %s, want %s", instanceID, id)
			}
			if chat != "5511999999999@s.whatsapp.net" {
				t.Errorf("EditMessage chat = %q, want the requested chat", chat)
			}
			if msgID != "WAID-1" {
				t.Errorf("EditMessage message_id = %q, want WAID-1", msgID)
			}
			if text != "novo texto" {
				t.Errorf("text = %q, want novo texto", text)
			}
			return "WAID-NEW-1", nil
		},
	}
	rec := serveJSON(t, editMessageServer(t, svc), http.MethodPost,
		"/instances/"+id.String()+"/messages/edit",
		`{"chat":"5511999999999@s.whatsapp.net","message_id":"WAID-1","text":"novo texto"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.MessageID != "WAID-NEW-1" {
		t.Errorf("message_id = %q, want WAID-NEW-1", payload.Data.MessageID)
	}
	if len(svc.editMessageCalls) != 1 {
		t.Fatalf("EditMessage calls = %d, want 1", len(svc.editMessageCalls))
	}
}

func TestEditMessageRejectsEmptyText(t *testing.T) {
	svc := &fakeInstanceService{}
	rec := serveJSON(t, editMessageServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages/edit",
		`{"chat":"5511999999999@s.whatsapp.net","message_id":"WAID-1","text":"   "}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if len(svc.editMessageCalls) != 0 {
		t.Errorf("EditMessage calls = %d, want none on empty text", len(svc.editMessageCalls))
	}
}

func TestEditMessageRejectsTooLong(t *testing.T) {
	svc := &fakeInstanceService{}
	rec := serveJSON(t, editMessageServer(t, svc), http.MethodPost,
		"/instances/"+uuid.NewString()+"/messages/edit",
		`{"chat":"5511999999999@s.whatsapp.net","message_id":"WAID-1","text":"`+strings.Repeat("a", 4097)+`"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if len(svc.editMessageCalls) != 0 {
		t.Errorf("EditMessage calls = %d, want none on oversized text", len(svc.editMessageCalls))
	}
}
