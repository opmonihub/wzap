package httpapi

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"wzap/internal/instance"
	"wzap/internal/model"
)

func TestDTOContractStructuredErrors(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name, code, text string
		at               *time.Time
		want             any
	}{
		{name: "none"},
		{name: "legacy", code: "legacy_error", text: "qr code expired", want: map[string]any{"code": "legacy_error", "message": "qr code expired", "occurred_at": nil}},
		{name: "recorded", code: "session_rejected", text: "session rejected", at: &at, want: map[string]any{"code": "session_rejected", "message": "session rejected", "occurred_at": "2026-10-06T10:00:00Z"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			stored := &model.Instance{ID: id, Name: "private", Connection: model.InstanceConnection{Status: "error", LastConnectedAt: &at, DeviceJID: "private-jid"}, CreatedAt: at, UpdatedAt: at}
			if tc.text != "" {
				stored.Connection.LastError = &model.InstanceError{Code: tc.code, Message: tc.text, At: tc.at}
			}
			svc := &fakeInstanceService{getFn: func(context.Context, uuid.UUID) (*model.Instance, error) { return stored, nil }}
			for _, suffix := range []string{"", "/status"} {
				rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/"+id.String()+suffix, "")
				if rec.Code != http.StatusOK {
					t.Fatal(rec.Body.String())
				}
				var payload map[string]any
				decodeJSON(t, rec.Body.Bytes(), &payload)
				data := payload["data"].(map[string]any)
				if suffix == "" {
					data = data["instance"].(map[string]any)
				}
				connection := data["connection"].(map[string]any)
				if !reflect.DeepEqual(connection["last_error"], tc.want) || connection["last_connected_at"] != "2026-10-06T10:00:00Z" {
					t.Errorf("connection = %v, want error %v and persisted timestamp", connection, tc.want)
				}
				requireContractArraysAndPrivacy(t, payload)
			}
		})
	}
}

func TestDTOContractMessageReceiptAndErrorFields(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	instanceID, messageID, mediaID := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		name, code, text, waID string
		at                     *time.Time
		want                   any
	}{
		{name: "queued"},
		{name: "legacy", text: "send timed out", want: map[string]any{"code": "legacy_error", "message": "send timed out", "occurred_at": nil}},
		{name: "recorded", code: "send_retry", text: "retry", waID: "wamid.1", at: &at, want: map[string]any{"code": "send_retry", "message": "retry", "occurred_at": "2026-10-06T10:00:00Z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := model.OutboundMessage{ID: messageID, InstanceID: instanceID, Type: "image", RecipientJID: "5511999999999@s.whatsapp.net", Status: "queued", Attempts: 2, WhatsAppMessageID: tc.waID, MediaID: &mediaID, LastError: tc.text, LastErrorCode: tc.code, LastErrorAt: tc.at, NextAttemptAt: &at, DeliveredAt: &at, ReadAt: &at, CreatedAt: at, UpdatedAt: at}
			svc := &fakeMessageService{getFn: func(context.Context, uuid.UUID, uuid.UUID) (*model.OutboundMessage, error) { return &message, nil }, listFn: func(context.Context, uuid.UUID, int, string) ([]model.OutboundMessage, string, error) {
				return []model.OutboundMessage{message}, "next-message", nil
			}}
			for _, suffix := range []string{"", "/" + messageID.String()} {
				rec := serveJSON(t, messagesServer(t, svc, nil), http.MethodGet, "/instances/"+instanceID.String()+"/messages"+suffix, "")
				if rec.Code != http.StatusOK {
					t.Fatal(rec.Body.String())
				}
				var payload map[string]any
				decodeJSON(t, rec.Body.Bytes(), &payload)
				data := payload["data"].(map[string]any)
				if suffix == "" {
					if data["next_cursor"] != "next-message" {
						t.Error("cursor lost")
					}
					data = data["items"].([]any)[0].(map[string]any)
				}
				object := data["message"].(map[string]any)
				requireContractEntity(t, data, "message")
				if !reflect.DeepEqual(object["last_error"], tc.want) || object["retry_count"] != float64(2) || object["media_id"] != mediaID.String() {
					t.Errorf("message = %v", object)
				}
				for _, key := range []string{"next_attempt_at", "delivered_at", "read_at", "created_at", "updated_at"} {
					if object[key] != "2026-10-06T10:00:00Z" {
						t.Errorf("%s = %v", key, object[key])
					}
				}
				if tc.waID == "" {
					if object["wa_id"] != nil {
						t.Errorf("unconfirmed wa_id = %v", object["wa_id"])
					}
				} else if object["wa_id"] != tc.waID {
					t.Error("upstream WA ID lost")
				}
			}
		})
	}
}

func TestDTOContractUserUsage(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	user := &model.User{ID: uuid.New(), Email: "operator@example.com", Role: "user", InstanceQuota: 0, PasswordHash: "private-hash", CreatedAt: at, UpdatedAt: at}
	other := uuid.New()
	keys := &countingKeys{instances: []model.Instance{ownedInstance(user.ID, "connected"), ownedInstance(user.ID, "disconnected"), ownedInstance(user.ID, "error"), ownedInstance(other, "connected"), {ID: uuid.New()}}}
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			srv := usersCRUDServer(t, newFakeUserRepository(user), keys, 4)
			rec := serveJSON(t, srv, method, "/users/"+user.ID.String(), `{"instance_limit":0}`)
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body.String())
			}
			var payload map[string]any
			decodeJSON(t, rec.Body.Bytes(), &payload)
			data := payload["data"].(map[string]any)
			requireContractEntity(t, data, "user")
			requireContractArraysAndPrivacy(t, payload)
			object := data["user"].(map[string]any)
			if object["instances_used"] != float64(3) || object["instance_limit"] != float64(0) || object["created_at"] != "2026-10-06T10:00:00Z" {
				t.Errorf("user = %v, want unlimited and all three owned states", object)
			}
			rec = serveJSON(t, srv, http.MethodGet, "/users", "")
			decodeJSON(t, rec.Body.Bytes(), &payload)
			listed := payload["data"].(map[string]any)["items"].([]any)[0].(map[string]any)["user"].(map[string]any)
			if listed["instances_used"] != float64(3) {
				t.Errorf("listed usage = %v", listed["instances_used"])
			}
		})
	}
}

func TestDTOContractNullUserLimit(t *testing.T) {
	user := &model.User{ID: uuid.New(), Email: "operator@example.com", Role: "user", InstanceQuota: 7}
	srv := usersCRUDServer(t, newFakeUserRepository(user), &fakeAPIKeyRepository{}, 4)
	rec := serveJSON(t, srv, http.MethodPatch, "/users/"+user.ID.String(), `{"instance_limit":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("null limit must clear to unlimited: %d %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload["data"].(map[string]any)["user"].(map[string]any)["instance_limit"] != float64(0) {
		t.Error("null did not become unlimited")
	}
}

func TestDTOContractQRFieldsOnlyWhenPairing(t *testing.T) {
	at := time.Now()
	svc := &fakeInstanceService{connectFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{Status: "connected", QRCode: "stale-qr", QRExpiresAt: &at}, nil
	}, qrFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{Status: "disconnected", QRCode: "stale-qr", QRExpiresAt: &at}, nil
	}}
	for _, tc := range []struct{ method, suffix string }{{http.MethodPost, "connect"}, {http.MethodGet, "qr"}} {
		rec := serveJSON(t, instancesServer(t, svc), tc.method, "/instances/"+uuid.NewString()+"/"+tc.suffix, "")
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		var payload map[string]any
		decodeJSON(t, rec.Body.Bytes(), &payload)
		connection := payload["data"].(map[string]any)["connection"].(map[string]any)
		requireContractKeys(t, connection, "status")
	}
}
