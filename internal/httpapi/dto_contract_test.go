package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/httpapi/chatwoot"
	"wzap/internal/httpapi/messages"
	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/storage"
)

func TestDTOContractStructuredErrors(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name, code, text string
		at               *time.Time
		want             any
	}{
		{name: "none"},
		{name: "without timestamp", code: "session_rejected", text: "qr code expired", want: map[string]any{"code": "session_rejected", "message": "qr code expired"}},
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
		{name: "message without code", text: "send timed out", want: nil},
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
					data = map[string]any{"message": data["messages"].([]any)[0]}
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
			listed := payload["data"].(map[string]any)["users"].([]any)[0].(map[string]any)
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

// instanceSettingsServer wires svc plus the Chatwoot config store behind the
// aggregate instance routes, with a public base for the computed webhook URL.
func instanceSettingsServer(t *testing.T, svc httpapi.InstanceService, configs chatwoot.ChatwootConfigStore) *http.Server {
	t.Helper()
	return httpapi.New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken}, zerolog.Nop(),
		httpapi.Deps{
			ReadyChecker:    checkFunc(func(context.Context) error { return nil }),
			Instances:       svc,
			ChatwootConfigs: configs,
			PublicURL:       "https://wzap.example.com",
		})
}

// TestDTOContractInstanceIntegrationAndSettings pins the aggregated instance
// shape (BREAKING): the webhook block travels under integration and the
// settings blocks are omitted while the instance is offline or never
// configured — the aggregate never fetches live blocks for a disconnected
// instance and the persisted default_disappearing echo stays null until the
// first PUT of the default timer.
func TestDTOContractInstanceIntegrationAndSettings(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	stored := &model.Instance{ID: id, Name: "loja",
		Connection: model.InstanceConnection{Status: "disconnected"},
		Webhook:    model.InstanceWebhook{Events: []string{"message", "receipt"}},
		CreatedAt:  at, UpdatedAt: at}
	svc := &fakeInstanceService{getFn: func(context.Context, uuid.UUID) (*model.Instance, error) { return stored, nil }}
	svc.getProfileFn = func(context.Context, uuid.UUID) (session.Profile, error) {
		t.Error("profile must not be fetched while disconnected")
		return session.Profile{}, nil
	}
	svc.getPrivacyFn = func(context.Context, uuid.UUID) (session.Privacy, error) {
		t.Error("privacy must not be fetched while disconnected")
		return session.Privacy{}, nil
	}
	svc.getStatusPrivacyFn = func(context.Context, uuid.UUID) (session.StatusPrivacy, error) {
		t.Error("status privacy must not be fetched while disconnected")
		return session.StatusPrivacy{}, nil
	}
	configs := &fakeChatwootConfigs{getFn: func(context.Context, uuid.UUID) (*model.ChatwootConfig, error) {
		return nil, storage.ErrNotFound
	}}

	rec := serveJSON(t, instanceSettingsServer(t, svc, configs), http.MethodGet, "/instances/"+id.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, rec.Body.Bytes(), &payload)
	inst := payload["data"].(map[string]any)["instance"].(map[string]any)
	requireContractKeys(t, inst, "id,name,connection,integration,settings,created_at,updated_at")

	integration := inst["integration"].(map[string]any)
	requireContractKeys(t, integration, "webhook,chatwoot_config")
	if _, exists := integration["chatwoot_config"]; exists {
		t.Errorf("integration.chatwoot_config = %v, want omitted without a stored config", integration["chatwoot_config"])
	}
	requireContractKeys(t, integration["webhook"].(map[string]any), "enabled,url,events")
	if events := integration["webhook"].(map[string]any)["events"].([]any); !reflect.DeepEqual(events, []any{"message", "receipt"}) {
		t.Errorf("integration.webhook.events = %v, want [message receipt]", events)
	}

	if _, exists := inst["settings"]; exists {
		t.Error("unconfigured settings must be omitted")
	}
	requireContractArraysAndPrivacy(t, payload)
}

// TestDTOContractInstanceSettingsAggregated pins the connected aggregate: the
// live settings blocks fill in, the Chatwoot config nests without its
// instance_id (already at data.instance.id) and without its write-only token,
// and the persisted default_disappearing echo renders in the textual form the
// PUT accepts.
func TestDTOContractInstanceSettingsAggregated(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		echo time.Duration
		want string
	}{
		{echo: 0, want: "0"},
		{echo: 24 * time.Hour, want: "24h"},
		{echo: 168 * time.Hour, want: "168h"},
		{echo: 2160 * time.Hour, want: "2160h"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			id := uuid.New()
			webhookURL := "https://hooks.example.com/wzap"
			echo := tc.echo
			stored := &model.Instance{ID: id, Name: "loja",
				Connection:          model.InstanceConnection{Status: "connected", LastConnectedAt: &at},
				Webhook:             model.InstanceWebhook{URL: &webhookURL, IsEnabled: true, Events: []string{"message"}},
				DefaultDisappearing: &echo,
				CreatedAt:           at, UpdatedAt: at}
			svc := &fakeInstanceService{
				getFn: func(context.Context, uuid.UUID) (*model.Instance, error) { return stored, nil },
				getProfileFn: func(context.Context, uuid.UUID) (session.Profile, error) {
					return session.Profile{Name: "Loja", StatusText: "Aberta", PhotoURL: "https://cdn/photo.jpg"}, nil
				},
				getPrivacyFn: func(context.Context, uuid.UUID) (session.Privacy, error) {
					return session.Privacy{LastSeen: "contacts", ProfilePhoto: "all", Status: "none", ReadReceipts: "all", GroupsAdd: "contacts"}, nil
				},
				getStatusPrivacyFn: func(context.Context, uuid.UUID) (session.StatusPrivacy, error) {
					return session.StatusPrivacy{Mode: "contacts"}, nil
				},
			}
			configs := &fakeChatwootConfigs{getFn: func(context.Context, uuid.UUID) (*model.ChatwootConfig, error) {
				return &model.ChatwootConfig{InstanceID: id, Enabled: true, URL: "https://chatwoot.example.com", AccountID: "7", Token: "private-token", NameInbox: "Loja", IgnoreJIDs: []string{"5511@spam"}}, nil
			}}

			rec := serveJSON(t, instanceSettingsServer(t, svc, configs), http.MethodGet, "/instances/"+id.String(), "")
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Body.String())
			}
			var payload map[string]any
			decodeJSON(t, rec.Body.Bytes(), &payload)
			inst := payload["data"].(map[string]any)["instance"].(map[string]any)
			integration := inst["integration"].(map[string]any)
			chatwoot, ok := integration["chatwoot_config"].(map[string]any)
			if !ok {
				t.Fatalf("integration.chatwoot_config = %v, want the stored config", integration["chatwoot_config"])
			}
			requireContractKeys(t, chatwoot, "is_enabled,url,account_id,inbox_name,is_sign_enabled,sign_delimiter,is_reopen_enabled,is_pending_enabled,is_merge_enabled,is_import_contacts,is_import_messages,import_days,is_auto_create,organization,logo,ignored_jids,webhook_url")
			if chatwoot["account_id"] != "7" || chatwoot["is_enabled"] != true {
				t.Errorf("integration.chatwoot_config = %v", chatwoot)
			}
			if chatwoot["webhook_url"] != "https://wzap.example.com/chatwoot/webhook/"+id.String() {
				t.Errorf("integration.chatwoot_config.webhook_url = %v", chatwoot["webhook_url"])
			}
			if strings.Contains(rec.Body.String(), `"instance_id"`) || strings.Contains(rec.Body.String(), "private-token") {
				t.Errorf("body %q duplicates instance_id or leaks the chatwoot token", rec.Body.String())
			}

			settings := inst["settings"].(map[string]any)
			if settings["default_disappearing"] != tc.want {
				t.Errorf("settings.default_disappearing = %v, want %q", settings["default_disappearing"], tc.want)
			}
			profile := settings["profile"].(map[string]any)
			requireContractKeys(t, profile, "name,status_text,photo_url")
			if profile["name"] != "Loja" || profile["status_text"] != "Aberta" || profile["photo_url"] != "https://cdn/photo.jpg" {
				t.Errorf("settings.profile = %v", profile)
			}
			privacy := settings["privacy"].(map[string]any)
			requireContractKeys(t, privacy, "last_seen,profile_photo,status,read_receipts,groups_add")
			if privacy["last_seen"] != "contacts" || privacy["read_receipts"] != "all" {
				t.Errorf("settings.privacy = %v", privacy)
			}
			statusPrivacy := settings["status_privacy"].(map[string]any)
			requireContractKeys(t, statusPrivacy, "mode,jids")
			if statusPrivacy["mode"] != "contacts" {
				t.Errorf("settings.status_privacy = %v", statusPrivacy)
			}
			if _, ok := statusPrivacy["jids"].([]any); !ok {
				t.Errorf("settings.status_privacy.jids = %v, want array", statusPrivacy["jids"])
			}
			requireContractArraysAndPrivacy(t, payload)
		})
	}
}

// A failing live read degrades to a null block: the aggregate still answers
// 200 with the remaining blocks instead of cascading a 500 or 409.
func TestDTOContractInstanceSettingsBlockFailureOmitsAbsent(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	stored := &model.Instance{ID: id, Name: "loja",
		Connection: model.InstanceConnection{Status: "connected"},
		Webhook:    model.InstanceWebhook{Events: []string{}}, CreatedAt: at, UpdatedAt: at}
	svc := &fakeInstanceService{
		getFn: func(context.Context, uuid.UUID) (*model.Instance, error) { return stored, nil },
		getProfileFn: func(context.Context, uuid.UUID) (session.Profile, error) {
			return session.Profile{}, context.DeadlineExceeded
		},
		getPrivacyFn: func(context.Context, uuid.UUID) (session.Privacy, error) {
			return session.Privacy{LastSeen: "all"}, nil
		},
		getStatusPrivacyFn: func(context.Context, uuid.UUID) (session.StatusPrivacy, error) {
			return session.StatusPrivacy{}, context.DeadlineExceeded
		},
	}
	configs := &fakeChatwootConfigs{getFn: func(context.Context, uuid.UUID) (*model.ChatwootConfig, error) {
		return nil, context.DeadlineExceeded
	}}

	rec := serveJSON(t, instanceSettingsServer(t, svc, configs), http.MethodGet, "/instances/"+id.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite block failures: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, rec.Body.Bytes(), &payload)
	settings := payload["data"].(map[string]any)["instance"].(map[string]any)["settings"].(map[string]any)
	if _, exists := settings["profile"]; exists {
		t.Errorf("settings.profile = %v, want omitted after a failed read", settings["profile"])
	}
	if _, exists := settings["status_privacy"]; exists {
		t.Errorf("settings.status_privacy = %v, want omitted after a failed read", settings["status_privacy"])
	}
	if settings["privacy"].(map[string]any)["last_seen"] != "all" {
		t.Errorf("settings.privacy = %v, want the block that succeeded", settings["privacy"])
	}
	integration := payload["data"].(map[string]any)["instance"].(map[string]any)["integration"].(map[string]any)
	if _, exists := integration["chatwoot_config"]; exists {
		t.Errorf("integration.chatwoot_config = %v, want omitted after a failed lookup", integration["chatwoot_config"])
	}
}

// TestDTOContractInstanceListKeepsOrderAndAbsentBlocks pins the listing
// aggregate: a disconnected item keeps its persisted integration and an omitted
// live block without a session read, a failed source omits only that block,
// and parallel assembly preserves item order.
func TestDTOContractInstanceListKeepsOrderAndAbsentBlocks(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	off := model.Instance{ID: uuid.New(), Name: "off", Connection: model.InstanceConnection{Status: "disconnected"}, Webhook: model.InstanceWebhook{Events: []string{"message"}}, CreatedAt: at, UpdatedAt: at}
	slow := model.Instance{ID: uuid.New(), Name: "slow", Connection: model.InstanceConnection{Status: "connected"}, Webhook: model.InstanceWebhook{Events: []string{"receipt"}}, CreatedAt: at, UpdatedAt: at}
	fast := model.Instance{ID: uuid.New(), Name: "fast", Connection: model.InstanceConnection{Status: "connected"}, Webhook: model.InstanceWebhook{Events: []string{"connection"}}, CreatedAt: at, UpdatedAt: at}
	var (
		mu           sync.Mutex
		profileCalls []uuid.UUID
	)
	svc := &fakeInstanceService{
		listFn: func(context.Context) ([]model.Instance, error) {
			return []model.Instance{off, slow, fast}, nil
		},
		getProfileFn: func(_ context.Context, id uuid.UUID) (session.Profile, error) {
			mu.Lock()
			profileCalls = append(profileCalls, id)
			mu.Unlock()
			if id == off.ID {
				t.Error("profile must not be fetched while disconnected")
			}
			if id == slow.ID {
				time.Sleep(30 * time.Millisecond)
				return session.Profile{}, context.DeadlineExceeded
			}
			return session.Profile{Name: "Fast"}, nil
		},
		getPrivacyFn: func(_ context.Context, id uuid.UUID) (session.Privacy, error) {
			if id == off.ID {
				t.Error("privacy must not be fetched while disconnected")
			}
			return session.Privacy{LastSeen: "all"}, nil
		},
		getStatusPrivacyFn: func(_ context.Context, id uuid.UUID) (session.StatusPrivacy, error) {
			if id == off.ID {
				t.Error("status privacy must not be fetched while disconnected")
			}
			return session.StatusPrivacy{Mode: "contacts"}, nil
		},
	}
	configs := &fakeChatwootConfigs{getFn: func(_ context.Context, id uuid.UUID) (*model.ChatwootConfig, error) {
		switch id {
		case off.ID:
			return nil, storage.ErrNotFound
		case slow.ID:
			return nil, context.DeadlineExceeded
		default:
			return &model.ChatwootConfig{InstanceID: id, Enabled: true, URL: "https://chatwoot.example.com", AccountID: "7", Token: "private-token", NameInbox: "Fast"}, nil
		}
	}}

	rec := serveJSON(t, instanceSettingsServer(t, svc, configs), http.MethodGet, "/instances", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, rec.Body.Bytes(), &payload)
	items := payload["data"].(map[string]any)["instances"].([]any)
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	wantOrder := []string{off.ID.String(), slow.ID.String(), fast.ID.String()}
	for i, wantID := range wantOrder {
		inst := items[i].(map[string]any)
		if inst["id"] != wantID {
			t.Errorf("items[%d].instance.id = %v, want %s", i, inst["id"], wantID)
		}
		requireContractKeys(t, inst, "id,name,connection,integration,settings,created_at,updated_at")
		if _, ok := inst["webhook"]; ok {
			t.Errorf("items[%d] still has a root webhook", i)
		}
	}
	if _, exists := items[0].(map[string]any)["settings"]; exists {
		t.Error("disconnected unconfigured settings must be omitted")
	}
	offIntegration := items[0].(map[string]any)["integration"].(map[string]any)
	if _, exists := offIntegration["chatwoot_config"]; exists {
		t.Errorf("disconnected chatwoot_config = %v, want omitted", offIntegration["chatwoot_config"])
	}
	slowSettings := items[1].(map[string]any)["settings"].(map[string]any)
	if _, exists := slowSettings["profile"]; exists {
		t.Errorf("failed profile = %v, want omitted", slowSettings["profile"])
	}
	if slowSettings["privacy"].(map[string]any)["last_seen"] != "all" {
		t.Errorf("privacy = %v, want the block that succeeded", slowSettings["privacy"])
	}
	slowIntegration := items[1].(map[string]any)["integration"].(map[string]any)
	if _, exists := slowIntegration["chatwoot_config"]; exists {
		t.Errorf("failed chatwoot_config = %v, want omitted", slowIntegration["chatwoot_config"])
	}
	fastChatwoot := items[2].(map[string]any)["integration"].(map[string]any)["chatwoot_config"].(map[string]any)
	if fastChatwoot["account_id"] != "7" || strings.Contains(rec.Body.String(), "private-token") {
		t.Errorf("fast chatwoot_config = %v", fastChatwoot)
	}
	requireContractArraysAndPrivacy(t, payload)
}

// Creation and update answer the same aggregated instance shape. A fresh
// instance is disconnected, so the live blocks and the timer echo are omitted.
func TestDTOContractInstanceWritesReturnAggregatedShape(t *testing.T) {
	created := &model.Instance{ID: uuid.New(), Name: "loja", Connection: model.InstanceConnection{Status: "disconnected"}, Webhook: model.InstanceWebhook{Events: []string{"message"}}}
	svc := &fakeInstanceService{
		oldestAdminFn: func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
		createFn: func(context.Context, instance.CreateInput) (*model.Instance, string, error) {
			return created, "one-time-key", nil
		},
		getProfileFn: func(context.Context, uuid.UUID) (session.Profile, error) {
			t.Error("profile must not be fetched for a fresh disconnected instance")
			return session.Profile{}, nil
		},
	}
	configs := &fakeChatwootConfigs{}

	rec := serveJSON(t, instanceSettingsServer(t, svc, configs), http.MethodPost, "/instances", `{"name":"loja"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var createdPayload map[string]any
	decodeJSON(t, rec.Body.Bytes(), &createdPayload)
	createdInst := createdPayload["data"].(map[string]any)["instance"].(map[string]any)
	requireContractKeys(t, createdInst, "id,name,connection,integration,settings,created_at,updated_at")
	_, hasSettings := createdInst["settings"]
	_, hasChatwoot := createdInst["integration"].(map[string]any)["chatwoot_config"]
	if hasSettings || hasChatwoot {
		t.Errorf("create instance = %v, want omitted live/chatwoot blocks", createdInst)
	}

	updated := &model.Instance{ID: created.ID, Name: "loja-2", Connection: model.InstanceConnection{Status: "disconnected"}, Webhook: model.InstanceWebhook{Events: []string{"message"}}}
	svc.updateFn = func(context.Context, uuid.UUID, instance.UpdateInput) (*model.Instance, error) {
		return updated, nil
	}
	rec = serveJSON(t, instanceSettingsServer(t, svc, configs), http.MethodPatch, "/instances/"+created.ID.String(), `{"name":"loja-2"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var updatedPayload map[string]any
	decodeJSON(t, rec.Body.Bytes(), &updatedPayload)
	updatedInst := updatedPayload["data"].(map[string]any)["instance"].(map[string]any)
	requireContractKeys(t, updatedInst, "id,name,connection,integration,settings,created_at,updated_at")
	if _, ok := updatedInst["webhook"]; ok {
		t.Error("update response still has a root webhook")
	}
}

// The accepted body carries only fields known at accept time: no fabricated
// zero values, and media_id only for media uploads.
func TestDTOContractMessageAcceptedFields(t *testing.T) {
	messageID, instanceID, mediaID := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		name    string
		mediaID *uuid.UUID
		want    any
	}{
		{name: "text", want: nil},
		{name: "media", mediaID: &mediaID, want: mediaID.String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(messages.NewMessageAcceptedResponse(messageID, instanceID, tc.mediaID))
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]map[string]any
			decodeJSON(t, raw, &payload)
			msg := payload["message"]
			requireContractKeys(t, msg, "id,instance_id,send_status,media_id")
			if msg["id"] != messageID.String() || msg["instance_id"] != instanceID.String() || msg["send_status"] != "queued" || msg["media_id"] != tc.want {
				t.Errorf("accepted message = %v", msg)
			}
			if strings.Contains(string(raw), "0001-01-01") {
				t.Errorf("accepted body carries a zero timestamp: %s", raw)
			}
		})
	}
}
