package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/instance"
	"wzap/internal/model"
)

const (
	testChatDirect = "5511999887766@s.whatsapp.net"
	testChatGroup  = "120363000000000000@g.us"
	testSender     = "5511888888888@s.whatsapp.net"
)

func TestRevoke(t *testing.T) {
	t.Run("connected answers revoked true", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/messages/revoke",
			`{"chat":"`+testChatDirect+`","message_id":"ORIG-1"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data revokeResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Revoked {
			t.Errorf("data.revoked = false, want true")
		}
		if len(svc.revokeCalls) != 1 {
			t.Fatalf("RevokeMessage calls = %d, want 1", len(svc.revokeCalls))
		}
		call := svc.revokeCalls[0]
		if call.InstanceID != id || call.ChatJID != testChatDirect || call.MessageID != "ORIG-1" {
			t.Errorf("RevokeMessage call = %+v, want instance %s chat %s id ORIG-1", call, id, testChatDirect)
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			revokeFn: func(context.Context, uuid.UUID, string, string) error {
				return instance.ErrNotConnected
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/messages/revoke",
			`{"chat":"`+testChatDirect+`","message_id":"ORIG-1"}`)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "conflict" {
			t.Errorf("error code = %q, want %q", code, "conflict")
		}
	})

	t.Run("invalid target answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{
			revokeFn: func(context.Context, uuid.UUID, string, string) error {
				return instance.ErrInvalidInput
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/messages/revoke",
			`{"chat":"not-a-jid","message_id":"ORIG-1"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
			t.Errorf("error code = %q, want %q", code, "unprocessable_entity")
		}
	})

	t.Run("empty fields answer unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		for _, body := range []string{
			`{"chat":"","message_id":"ORIG-1"}`,
			`{"chat":"` + testChatDirect + `","message_id":""}`,
		} {
			rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
				"/instances/"+uuid.NewString()+"/messages/revoke", body)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("body %q: status = %d, want %d", body, rec.Code, http.StatusUnprocessableEntity)
			}
		}
		if len(svc.revokeCalls) != 0 {
			t.Errorf("RevokeMessage calls = %d, want none on empty fields", len(svc.revokeCalls))
		}
	})

	t.Run("unknown instance answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
				return nil, instance.ErrNotFound
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/messages/revoke",
			`{"chat":"`+testChatDirect+`","message_id":"ORIG-1"}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
			t.Errorf("error code = %q, want %q", code, "not_found")
		}
		if len(svc.revokeCalls) != 0 {
			t.Errorf("RevokeMessage calls = %d, want none on unknown instance", len(svc.revokeCalls))
		}
	})
}

func TestMarkRead(t *testing.T) {
	t.Run("direct chat without sender completes with the chat", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/chats/mark-read",
			`{"chat":"`+testChatDirect+`","message_id":"ORIG-9"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data markReadResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.MarkedRead {
			t.Errorf("data.marked_read = false, want true")
		}
		if len(svc.markReadCalls) != 1 {
			t.Fatalf("MarkRead calls = %d, want 1", len(svc.markReadCalls))
		}
		call := svc.markReadCalls[0]
		if call.SenderJID != testChatDirect {
			t.Errorf("sender = %q, want the chat itself %q", call.SenderJID, testChatDirect)
		}
	})

	t.Run("group without sender answers unprocessable", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/chats/mark-read",
			`{"chat":"`+testChatGroup+`","message_id":"ORIG-9"}`)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.markReadCalls) != 0 {
			t.Errorf("MarkRead calls = %d, want none without a group sender", len(svc.markReadCalls))
		}
	})

	t.Run("group with sender is delivered", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/chats/mark-read",
			`{"chat":"`+testChatGroup+`","sender":"`+testSender+`","message_id":"ORIG-9"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(svc.markReadCalls) != 1 || svc.markReadCalls[0].SenderJID != testSender {
			t.Errorf("MarkRead calls = %+v, want one with sender %q", svc.markReadCalls, testSender)
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			markReadFn: func(context.Context, uuid.UUID, string, string, string) error {
				return instance.ErrNotConnected
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/chats/mark-read",
			`{"chat":"`+testChatDirect+`","message_id":"ORIG-9"}`)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})
}
