package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"wzap/internal/config"
	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/manager"
)

// These fixtures record the closed response matrix, independently of Swagger
// and transport DTO types. Each exercises the production router and checks the
// status, success envelope, exact result keys, arrays and secret exclusions.
// Detailed validation/auth cases remain in the existing per-handler suites.
type responseContractFixture struct {
	method, path, body, keys, entity, upload string
	status                                   int
}

var responseContractFixtures = []responseContractFixture{
	{method: "POST", path: "/auth/login", status: 200, body: `{"email":"operator@example.com","password":"contract-password"}`, keys: "me", entity: "me", upload: ""},
	{method: "POST", path: "/auth/logout", status: 200, body: ``, keys: "status", entity: "", upload: ""},
	{method: "GET", path: "/auth/me", status: 200, body: ``, keys: "me", entity: "me", upload: ""},
	{method: "POST", path: "/chatwoot/webhook/{id}", status: 200, body: `{}`, keys: "content", entity: "", upload: ""},
	{method: "GET", path: "/healthz", status: 200, body: ``, keys: "status", entity: "", upload: ""},
	{method: "GET", path: "/instances", status: 200, body: ``, keys: "items", entity: "items.instance", upload: ""},
	{method: "POST", path: "/instances", status: 201, body: `{"name":"contract"}`, keys: "instance,instance_api_key", entity: "instance", upload: ""},
	{method: "GET", path: "/instances/stats", status: 200, body: ``, keys: "stats", entity: "stats", upload: ""},
	{method: "GET", path: "/instances/{id}", status: 200, body: ``, keys: "instance", entity: "instance", upload: ""},
	{method: "DELETE", path: "/instances/{id}", status: 204, body: ``, keys: "", entity: "", upload: ""},
	{method: "PATCH", path: "/instances/{id}", status: 200, body: `{"name":"contract"}`, keys: "instance", entity: "instance", upload: ""},
	{method: "DELETE", path: "/instances/{id}/apikey", status: 204, body: ``, keys: "", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/apikey/rotate", status: 200, body: ``, keys: "id,instance_api_key", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/blocklist", status: 200, body: ``, keys: "items", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/blocklist", status: 200, body: `{"action":"block","jid":"5511999999999@s.whatsapp.net"}`, keys: "updated", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/calls/reject", status: 200, body: `{"call_id":"call-1","from":"5511999999999@s.whatsapp.net"}`, keys: "rejected", entity: "", upload: ""},
	{method: "PUT", path: "/instances/{id}/chats/default-disappearing", status: 200, body: `{"duration":"24h"}`, keys: "updated", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/chats/mark-read", status: 200, body: `{"chat":"5511999999999@s.whatsapp.net","sender":"5511999999999@s.whatsapp.net","message_id":"wamid.1"}`, keys: "marked_read", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/chats/{chat}/disappearing", status: 200, body: ``, keys: "chat,duration_seconds,found", entity: "", upload: ""},
	{method: "PUT", path: "/instances/{id}/chats/{chat}/disappearing", status: 200, body: `{"duration":"24h"}`, keys: "updated", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/chatwoot", status: 200, body: ``, keys: "chatwoot_config", entity: "chatwoot_config", upload: ""},
	{method: "PUT", path: "/instances/{id}/chatwoot", status: 200, body: `{"is_enabled":false,"token":"hidden-token"}`, keys: "chatwoot_config", entity: "chatwoot_config", upload: ""},
	{method: "POST", path: "/instances/{id}/chatwoot/command", status: 200, body: `{"command":"init","conversation_id":1}`, keys: "ok", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/chatwoot/import", status: 202, body: `{}`, keys: "imported", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/connect", status: 200, body: ``, keys: "connection", entity: "pairing", upload: ""},
	{method: "GET", path: "/instances/{id}/contact-link", status: 200, body: ``, keys: "link", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/contacts/check", status: 200, body: `{"phones":["5511999999999"]}`, keys: "items", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/contacts/{jid}/business", status: 200, body: ``, keys: "jid,name,description,verified_name", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/contacts/{jid}/devices", status: 200, body: ``, keys: "jid,devices", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/contacts/{jid}/photo", status: 200, body: ``, keys: "jid,url,version", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/contacts/{jid}/subscribe", status: 200, body: ``, keys: "subscribed", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/disconnect", status: 204, body: ``, keys: "", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/groups", status: 200, body: ``, keys: "items,next_cursor", entity: "items.group", upload: ""},
	{method: "POST", path: "/instances/{id}/groups", status: 201, body: `{"name":"Friends"}`, keys: "group", entity: "group", upload: ""},
	{method: "GET", path: "/instances/{id}/groups/invite-preview", status: 200, body: ``, keys: "group", entity: "group", upload: ""},
	{method: "POST", path: "/instances/{id}/groups/join", status: 200, body: `{"invite_code":"invite-code"}`, keys: "jid", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/groups/{group_id}", status: 200, body: ``, keys: "group", entity: "group", upload: ""},
	{method: "PATCH", path: "/instances/{id}/groups/{group_id}", status: 200, body: `{"name":"Friends"}`, keys: "updated", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/groups/{group_id}/invite", status: 200, body: ``, keys: "invite_code", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/groups/{group_id}/invite/reset", status: 200, body: ``, keys: "invite_code", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/groups/{group_id}/leave", status: 200, body: ``, keys: "left", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/groups/{group_id}/participants", status: 200, body: `{"action":"add","participants":["5511999999999@s.whatsapp.net"]}`, keys: "updated", entity: "", upload: ""},
	{method: "PUT", path: "/instances/{id}/groups/{group_id}/photo", status: 200, body: `image-bytes`, keys: "updated", entity: "", upload: "raw"},
	{method: "GET", path: "/instances/{id}/groups/{group_id}/requests", status: 200, body: ``, keys: "participants", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/groups/{group_id}/requests", status: 200, body: `{"action":"approve","participants":["5511999999999@s.whatsapp.net"]}`, keys: "updated", entity: "", upload: ""},
	{method: "PATCH", path: "/instances/{id}/groups/{group_id}/settings", status: 200, body: `{"announce":true}`, keys: "updated", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/messages", status: 200, body: ``, keys: "items,next_cursor", entity: "items.message", upload: ""},
	{method: "POST", path: "/instances/{id}/messages", status: 202, body: `{"type":"buttons","to":"5511999999999","text":"Choose","buttons":[{"id":"yes","title":"Yes"}]}`, keys: "message", entity: "message", upload: ""},
	{method: "POST", path: "/instances/{id}/messages/contact", status: 202, body: `{"to":"5511999999999","display_name":"Contact","vcard":"BEGIN:VCARD\nEND:VCARD"}`, keys: "message", entity: "message", upload: ""},
	{method: "POST", path: "/instances/{id}/messages/edit", status: 200, body: `{"chat":"5511999999999@s.whatsapp.net","message_id":"wamid.1","text":"Edited"}`, keys: "message_id", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/messages/location", status: 202, body: `{"to":"5511999999999","latitude":1,"longitude":2}`, keys: "message", entity: "message", upload: ""},
	{method: "POST", path: "/instances/{id}/messages/media", status: 202, body: `{"to":"5511999999999","type":"image"}`, keys: "message", entity: "message", upload: "multipart"},
	{method: "POST", path: "/instances/{id}/messages/revoke", status: 200, body: `{"chat":"5511999999999@s.whatsapp.net","message_id":"wamid.1"}`, keys: "revoked", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/messages/text", status: 202, body: `{"to":"5511999999999","text":"Hello"}`, keys: "message", entity: "message", upload: ""},
	{method: "GET", path: "/instances/{id}/messages/{message_id}", status: 200, body: ``, keys: "message", entity: "message", upload: ""},
	{method: "GET", path: "/instances/{id}/newsletters", status: 200, body: ``, keys: "items,next_cursor", entity: "items.channel", upload: ""},
	{method: "POST", path: "/instances/{id}/newsletters", status: 201, body: `{"title":"News"}`, keys: "channel", entity: "channel", upload: ""},
	{method: "POST", path: "/instances/{id}/newsletters/follow", status: 200, body: `{"channel":"123@newsletter"}`, keys: "followed", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/newsletters/unfollow", status: 200, body: `{"channel":"123@newsletter"}`, keys: "followed", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/newsletters/{channel}", status: 200, body: ``, keys: "channel", entity: "channel", upload: ""},
	{method: "GET", path: "/instances/{id}/newsletters/{channel}/messages", status: 200, body: ``, keys: "items,next_cursor", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/newsletters/{channel}/mute", status: 200, body: `{"muted":true}`, keys: "muted", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/newsletters/{channel}/reactions", status: 200, body: `{"server_id":"1","reaction":"ok"}`, keys: "reacted", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/newsletters/{channel}/updates", status: 200, body: ``, keys: "items", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/newsletters/{channel}/viewed", status: 200, body: `{"server_ids":["1"]}`, keys: "viewed", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/numbers/check", status: 200, body: `{"phone":"5511999999999"}`, keys: "exists,jid,normalized", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/pair-phone", status: 200, body: `{"phone":"5511999999999"}`, keys: "pairing_code,expires_at", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/presence", status: 200, body: `{"chat":"5511999999999@s.whatsapp.net","state":"composing"}`, keys: "sent", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/privacy", status: 200, body: ``, keys: "last_seen,profile_photo,status,read_receipts,groups_add", entity: "", upload: ""},
	{method: "PUT", path: "/instances/{id}/privacy", status: 200, body: `{"last_seen":"contacts"}`, keys: "last_seen,profile_photo,status,read_receipts,groups_add", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/profile", status: 200, body: ``, keys: "name,status_text,photo_url", entity: "", upload: ""},
	{method: "PATCH", path: "/instances/{id}/profile", status: 200, body: `{"name":"Shop"}`, keys: "name,status_text,photo_url", entity: "", upload: ""},
	{method: "PUT", path: "/instances/{id}/profile/photo", status: 200, body: `image-bytes`, keys: "updated", entity: "", upload: "raw"},
	{method: "GET", path: "/instances/{id}/qr", status: 200, body: ``, keys: "connection", entity: "pairing", upload: ""},
	{method: "GET", path: "/instances/{id}/status", status: 200, body: ``, keys: "connection", entity: "connection", upload: ""},
	{method: "GET", path: "/instances/{id}/status/privacy", status: 200, body: ``, keys: "mode,jids", entity: "", upload: ""},
	{method: "GET", path: "/instances/{id}/status/updates", status: 200, body: ``, keys: "items", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/status/updates", status: 202, body: `{"type":"text","text":"Hello"}`, keys: "message_id,status", entity: "", upload: ""},
	{method: "POST", path: "/instances/{id}/status/updates/media", status: 202, body: `{"type":"image"}`, keys: "message_id,status", entity: "", upload: "multipart"},
	{method: "DELETE", path: "/instances/{id}/status/updates/{status_id}", status: 200, body: ``, keys: "deleted", entity: "", upload: ""},
	{method: "GET", path: "/manager", status: 301, body: ``, keys: "", entity: "", upload: ""},
	{method: "GET", path: "/manager/", status: 200, body: ``, keys: "", entity: "", upload: ""},
	{method: "GET", path: "/media/{id}", status: 200, body: ``, keys: "", entity: "", upload: ""},
	{method: "GET", path: "/readyz", status: 200, body: ``, keys: "status,checks", entity: "", upload: ""},
	{method: "GET", path: "/users", status: 200, body: ``, keys: "items", entity: "items.user", upload: ""},
	{method: "POST", path: "/users", status: 201, body: `{"email":"created@example.com","password":"new-password","role":"user","instance_limit":0}`, keys: "user", entity: "user", upload: ""},
	{method: "GET", path: "/users/{id}", status: 200, body: ``, keys: "user", entity: "user", upload: ""},
	{method: "DELETE", path: "/users/{id}", status: 204, body: ``, keys: "", entity: "", upload: ""},
	{method: "PATCH", path: "/users/{id}", status: 200, body: `{"instance_limit":0}`, keys: "user", entity: "user", upload: ""},
}

func TestResponseContractMatrix(t *testing.T) {
	t.Setenv("WZAP_MANAGER_DIR", "")
	operator := seedAuthUser(t, "operator@example.com", "contract-password", "admin")
	if len(responseContractFixtures) != 89 {
		t.Fatalf("fixtures = %d, want the 89 closed operations", len(responseContractFixtures))
	}
	seen := map[string]bool{}
	for _, fixture := range responseContractFixtures {
		route := fixture.method + " " + fixture.path
		if seen[route] {
			t.Fatalf("duplicate fixture: %s", route)
		}
		seen[route] = true
		t.Run(route, func(t *testing.T) {
			srv := responseContractServer(t, operator, false)
			rec := runResponseContractFixture(t, srv, operator, fixture)
			wantStatus := fixture.status
			if strings.HasPrefix(fixture.path, "/manager") && !manager.Built() {
				wantStatus = http.StatusServiceUnavailable
			}
			if rec.Code != wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
			}
			if rec.Header().Get(requestIDHeader) != "response-contract" {
				t.Errorf("request id = %q", rec.Header().Get(requestIDHeader))
			}
			if wantStatus == http.StatusNoContent {
				if rec.Body.Len() != 0 {
					t.Errorf("204 has body: %s", rec.Body.String())
				}
				return
			}
			if strings.HasPrefix(fixture.path, "/manager") {
				if strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
					t.Error("console must preserve HTML/plain format")
				}
				if wantStatus == http.StatusMovedPermanently && rec.Header().Get("Location") != "/manager/" {
					t.Error("missing console redirect")
				}
				return
			}
			if fixture.path == "/media/{id}" {
				if rec.Body.String() != "media-bytes" || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Errorf("media exception = %s %v", rec.Body.String(), rec.Header())
				}
				return
			}
			var payload map[string]any
			decodeJSON(t, rec.Body.Bytes(), &payload)
			if fixture.path == "/chatwoot/webhook/{id}" {
				requireContractKeys(t, payload, "content")
				return
			}
			requireContractKeys(t, payload, "data")
			data, ok := payload["data"].(map[string]any)
			if !ok {
				t.Fatalf("data must be an object: %s", rec.Body.String())
			}
			requireContractKeys(t, data, fixture.keys)
			requireContractArraysAndPrivacy(t, payload)
			if fixture.entity != "" {
				requireContractEntity(t, data, fixture.entity)
			}
			if value, ok := data["updated"]; ok && value != true {
				t.Errorf("updated = %v, want true", value)
			}
			if key, ok := data["instance_api_key"]; ok && key == "" {
				t.Error("creation/rotation key is empty")
			}
			if fixture.method == "POST" && fixture.entity == "message" {
				msg := data["message"].(map[string]any)
				if msg["send_status"] != "queued" || msg["wa_id"] != nil {
					t.Errorf("accepted message = %v", msg)
				}
			}
		})
	}
}

func TestResponseContractEmptyCollections(t *testing.T) {
	operator := seedAuthUser(t, "operator@example.com", "contract-password", "admin")
	for _, fixture := range responseContractFixtures {
		if !strings.Contains(fixture.keys, "items") || fixture.method != "GET" {
			continue
		}
		t.Run(fixture.path, func(t *testing.T) {
			rec := runResponseContractFixture(t, responseContractServer(t, operator, true), operator, fixture)
			if rec.Code != http.StatusOK {
				t.Fatalf("empty collection = %d %s", rec.Code, rec.Body.String())
			}
			var payload map[string]any
			decodeJSON(t, rec.Body.Bytes(), &payload)
			data := payload["data"].(map[string]any)
			items, ok := data["items"].([]any)
			if !ok || len(items) != 0 {
				t.Errorf("items = %v, want []", data["items"])
			}
			requireContractKeys(t, data, fixture.keys)
		})
	}
}

func responseContractServer(t *testing.T, operator *model.User, empty bool) *http.Server {
	t.Helper()
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	now := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	private := "private-reference"
	stored := model.Instance{ID: id, Name: "contract", ExternalRef: private, OwnerUserID: &operator.ID, CreatedAt: now, UpdatedAt: now,
		Connection: model.InstanceConnection{Status: "connected", DeviceJID: private}, Webhook: model.InstanceWebhook{}}
	svc := existingInstanceService()
	svc.getFn = func(context.Context, uuid.UUID) (*model.Instance, error) { return &stored, nil }
	svc.createFn = func(context.Context, instance.CreateInput) (*model.Instance, string, error) {
		return &stored, "one-time-instance-key", nil
	}
	svc.updateFn = func(context.Context, uuid.UUID, instance.UpdateInput) (*model.Instance, error) { return &stored, nil }
	svc.listFn = func(context.Context) ([]model.Instance, error) {
		if empty {
			return nil, nil
		}
		return []model.Instance{stored}, nil
	}
	svc.getJoinedGroupsFn = func(context.Context, uuid.UUID, int, string) ([]instance.Group, string, error) {
		if empty {
			return nil, "", nil
		}
		return []instance.Group{{JID: "123@g.us", Name: "Friends"}}, "next-group", nil
	}
	svc.listNewslettersFn = func(context.Context, uuid.UUID, int, string) ([]instance.Newsletter, string, error) {
		if empty {
			return nil, "", nil
		}
		return []instance.Newsletter{{ChannelJID: "123@newsletter", Title: "News"}}, "next-channel", nil
	}
	messages := &fakeMessageService{getFn: func(_ context.Context, instanceID, messageID uuid.UUID) (*model.OutboundMessage, error) {
		return &model.OutboundMessage{ID: messageID, InstanceID: instanceID, Type: "text", Status: "sent", WhatsAppMessageID: "wamid.1", CreatedAt: now, UpdatedAt: now}, nil
	},
		listFn: func(context.Context, uuid.UUID, int, string) ([]model.OutboundMessage, string, error) {
			if empty {
				return nil, "", nil
			}
			return []model.OutboundMessage{{ID: uuid.MustParse("22222222-2222-4222-8222-222222222222"), InstanceID: id, Type: "text", Status: "queued", CreatedAt: now, UpdatedAt: now}}, "next-message", nil
		}}
	users := newFakeUserRepository(operator)
	if empty {
		users = newFakeUserRepository()
	}
	configs := &fakeChatwootConfigs{getFn: func(context.Context, uuid.UUID) (*model.ChatwootConfig, error) {
		return &model.ChatwootConfig{InstanceID: id, Enabled: true, Token: "private-token"}, nil
	}}
	store := &fakeMediaStore{openFn: func(context.Context, uuid.UUID) (io.ReadCloser, *model.Media, error) {
		return io.NopCloser(strings.NewReader("media-bytes")), &model.Media{InstanceID: id, Mimetype: "image/jpeg", SizeBytes: 11}, nil
	}}
	return New(config.Config{APIKey: testToken, JWTSecret: testJWTSecret, MaxMediaBytes: testMaxMediaBytes}, zerolog.Nop(), Deps{Instances: svc, Messages: messages, Users: users, Keys: &fakeAPIKeyRepository{}, JWTSecret: testJWTSecret, Numbers: &fakeNumberResolver{}, Idempotency: newFakeIdempotency(), Media: store,
		ReadyChecker: checkFunc(func(context.Context) error { return nil }), Chatwoot: config.Chatwoot{Enabled: true}, ChatwootConfigs: configs, ChatwootInbound: &fakeChatwootInbound{status: 200}, ChatwootImporter: &fakeImporter{count: 3}, PublicURL: "https://wzap.example.com"})
}

func runResponseContractFixture(t *testing.T, srv *http.Server, operator *model.User, f responseContractFixture) *httptest.ResponseRecorder {
	t.Helper()
	id := "11111111-1111-4111-8111-111111111111"
	if strings.HasPrefix(f.path, "/users/") {
		id = operator.ID.String()
	}
	path := strings.NewReplacer("{id}", id, "{message_id}", "22222222-2222-4222-8222-222222222222", "{jid}", "5511999999999@s.whatsapp.net", "{chat}", "5511999999999@s.whatsapp.net", "{group_id}", "123@g.us", "{channel}", "123@newsletter", "{status_id}", "wamid.status").Replace(f.path)
	if f.path == "/instances/{id}/groups/invite-preview" {
		path += "?invite_code=invite-code"
	}
	body := []byte(f.body)
	contentType := "application/json"
	if f.upload == "raw" {
		contentType = "image/jpeg"
	}
	if f.upload == "multipart" {
		var fields map[string]string
		decodeJSON(t, body, &fields)
		pairs := make([][2]string, 0, len(fields))
		for key, value := range fields {
			pairs = append(pairs, [2]string{key, value})
		}
		body, contentType = multipartBody(t, pairs, "photo.jpg", "image/jpeg", []byte{0xff, 0xd8, 0xff, 0x00, 0x01})
	}
	req := httptest.NewRequest(f.method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set(requestIDHeader, "response-contract")
	if strings.HasPrefix(f.path, "/auth/") {
		req.AddCookie(rbacSessionCookie(mustSessionToken(t, operator.ID, "admin")))
	} else if !strings.HasPrefix(f.path, "/manager") && !strings.HasPrefix(f.path, "/chatwoot/webhook/") {
		req.Header.Set("apikey", testToken)
	}
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

func requireContractKeys(t *testing.T, object map[string]any, csv string) {
	t.Helper()
	want := strings.Split(csv, ",")
	slices.Sort(want)
	got := make([]string, 0, len(object))
	for key := range object {
		got = append(got, key)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func requireContractEntity(t *testing.T, data map[string]any, entity string) {
	t.Helper()
	if strings.HasPrefix(entity, "items.") {
		items, ok := data["items"].([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("resource items missing: %v", data)
		}
		for _, item := range items {
			wrapped, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("resource item is not object: %v", item)
			}
			key := strings.TrimPrefix(entity, "items.")
			requireContractKeys(t, wrapped, key)
			requireContractEntity(t, wrapped, key)
		}
		return
	}
	key := entity
	if entity == "pairing" {
		key = "connection"
	}
	object, ok := data[key].(map[string]any)
	if !ok {
		t.Fatalf("missing entity %s in %v", key, data)
	}
	switch entity {
	case "instance":
		requireContractKeys(t, object, "id,name,connection,webhook,created_at,updated_at")
		requireContractEntity(t, object, "connection")
		hook, ok := object["webhook"].(map[string]any)
		if !ok {
			t.Fatal("missing webhook")
		}
		requireContractKeys(t, hook, "enabled,url,events")
	case "message":
		requireContractKeys(t, object, "id,instance_id,message_type,recipient_jid,send_status,wa_id,media_id,retry_count,last_error,next_attempt_at,delivered_at,read_at,created_at,updated_at")
	case "user":
		requireContractKeys(t, object, "id,email,role,instance_limit,instances_used,created_at,updated_at")
	case "me":
		requireContractKeys(t, object, "id,email,role")
	case "connection":
		requireContractKeys(t, object, "status,last_error,last_connected_at")
	case "pairing":
		requireContractKeys(t, object, "status,qr_code,qr_expires_at")
	case "stats":
		requireContractKeys(t, object, "total,by_status")
		requireContractKeys(t, object["by_status"].(map[string]any), "connected,disconnected,pairing,error")
	case "chatwoot_config":
		requireContractKeys(t, object, "instance_id,is_enabled,url,account_id,inbox_name,is_sign_enabled,sign_delimiter,is_reopen_enabled,is_pending_enabled,is_merge_enabled,is_import_contacts,is_import_messages,import_days,is_auto_create,organization,logo,ignored_jids,webhook_url")
	case "group":
		for _, key := range []string{"jid", "name", "participants", "participant_count", "updated_at"} {
			if _, ok := object[key]; !ok {
				t.Errorf("group missing %s", key)
			}
		}
	case "channel":
		for _, key := range []string{"channel", "title", "follower_count", "updated_at"} {
			if _, ok := object[key]; !ok {
				t.Errorf("channel missing %s", key)
			}
		}
	}
}

func requireContractArraysAndPrivacy(t *testing.T, value any) {
	t.Helper()
	switch object := value.(type) {
	case map[string]any:
		for key, value := range object {
			if slices.Contains([]string{"device_jid", "whatsapp_jid", "external_ref", "owner_user_id", "api_key_hash", "password", "password_hash", "token", "instance_quota"}, key) {
				t.Errorf("public response exposes %s", key)
			}
			if slices.Contains([]string{"items", "participants", "devices", "events", "jids", "ignored_jids"}, key) {
				if _, ok := value.([]any); !ok {
					t.Errorf("%s = %v, want array", key, value)
				}
			}
			requireContractArraysAndPrivacy(t, value)
		}
	case []any:
		for _, value := range object {
			requireContractArraysAndPrivacy(t, value)
		}
	}
}

// Every private operation must retain the same error envelope before any
// resource lookup or command. Public/session exceptions have dedicated tests.
func TestResponseContractUnauthorizedOperations(t *testing.T) {
	srv := New(config.Config{APIKey: testToken}, zerolog.Nop(), Deps{})
	for _, f := range responseContractFixtures {
		if strings.HasPrefix(f.path, "/auth/") || strings.HasPrefix(f.path, "/manager") || strings.HasPrefix(f.path, "/chatwoot/webhook/") || f.path == "/healthz" || f.path == "/readyz" {
			continue
		}
		t.Run(f.method+" "+f.path, func(t *testing.T) {
			req := httptest.NewRequest(f.method, f.path, strings.NewReader(f.body))
			req.Header.Set(requestIDHeader, "response-contract-error")
			rec := httptest.NewRecorder()
			srv.Handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("missing credential = %d %s", rec.Code, rec.Body.String())
			}
			var payload map[string]any
			decodeJSON(t, rec.Body.Bytes(), &payload)
			requireContractKeys(t, payload, "error")
			requireContractKeys(t, payload["error"].(map[string]any), "code,message")
			if rec.Header().Get(requestIDHeader) != "response-contract-error" {
				t.Error("request id lost on error")
			}
		})
	}
}

// A tiny console fixture exercises the preserved HTML/redirect contract even
// on a fresh worktree without a generated Nuxt bundle. It uses the supported
// development directory override and never writes into manager/.
func TestResponseContractConsoleAndSwagger(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<!doctype html><title>Console fixture</title>"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WZAP_MANAGER_DIR", directory)
	srv := New(config.Config{APIKey: testToken}, zerolog.Nop(), Deps{})
	for _, tc := range []struct {
		path   string
		status int
	}{{"/manager", http.StatusMovedPermanently}, {"/manager/", http.StatusOK}, {"/manager/instances", http.StatusOK}, {"/swagger/index.html", http.StatusOK}} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set(requestIDHeader, "public-html")
			rec := httptest.NewRecorder()
			srv.Handler.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("HTML response = %d %s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get(requestIDHeader) != "public-html" || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
				t.Errorf("HTML headers = %v", rec.Header())
			}
			if tc.status == http.StatusMovedPermanently && rec.Header().Get("Location") != "/manager/" {
				t.Error("console redirect lost")
			}
		})
	}
}
