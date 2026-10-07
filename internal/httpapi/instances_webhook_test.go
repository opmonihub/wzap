package httpapi

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/instance"
	"wzap/internal/model"
)

// webhookRoundTripFake is a stateful InstanceService fake: Create and Update
// apply the webhook inputs they receive, so the tests can round-trip a
// configuration through POST/PATCH and read it back with GET.
type webhookRoundTripFake struct {
	fakeInstanceService
	stored *model.Instance
}

func newWebhookRoundTripFake() *webhookRoundTripFake {
	f := &webhookRoundTripFake{}
	f.createFn = func(_ context.Context, input instance.CreateInput) (*model.Instance, string, error) {
		stored := &model.Instance{ID: uuid.New(), Name: input.Name, ExternalRef: input.ExternalRef, OwnerUserID: input.OwnerUserID, Connection: model.InstanceConnection{Status: "disconnected"}}
		if input.WebhookURL != nil {
			url := *input.WebhookURL
			stored.Webhook.URL = &url
		}
		if input.WebhookEnabled != nil {
			stored.Webhook.IsEnabled = *input.WebhookEnabled
		}
		if input.WebhookEvents != nil {
			stored.Webhook.Events = append([]string(nil), (*input.WebhookEvents)...)
		} else {
			stored.Webhook.Events = []string{"message", "receipt", "connection", "message.status"}
		}
		f.stored = stored
		return stored, "one-time-key", nil
	}
	f.getFn = func(_ context.Context, id uuid.UUID) (*model.Instance, error) {
		if f.stored == nil || f.stored.ID != id {
			return nil, instance.ErrNotFound
		}
		return f.stored, nil
	}
	f.updateFn = func(_ context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error) {
		if f.stored == nil || f.stored.ID != id {
			return nil, instance.ErrNotFound
		}
		if input.WebhookURL != nil {
			if *input.WebhookURL == "" {
				f.stored.Webhook.URL = nil
			} else {
				url := *input.WebhookURL
				f.stored.Webhook.URL = &url
			}
		}
		if input.WebhookEnabled != nil {
			f.stored.Webhook.IsEnabled = *input.WebhookEnabled
		}
		if input.WebhookEvents != nil {
			f.stored.Webhook.Events = append([]string(nil), (*input.WebhookEvents)...)
		}
		return f.stored, nil
	}
	return f
}

// webhookData decodes data.instance.integration.webhook of rec into a raw
// field map (the webhook block moved under integration in the aggregated
// shape).
func webhookData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload struct {
		Data struct {
			Instance struct {
				ID          string `json:"id"`
				Integration struct {
					Webhook map[string]any `json:"webhook"`
				} `json:"integration"`
			} `json:"instance"`
		} `json:"data"`
	}
	decodeJSON(t, body, &payload)
	return payload.Data.Instance.Integration.Webhook
}

// instanceData decodes data.instance of rec into a raw field map.
func instanceData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload struct {
		Data struct {
			Instance map[string]any `json:"instance"`
		} `json:"data"`
	}
	decodeJSON(t, body, &payload)
	return payload.Data.Instance
}

func TestInstancesCreateWithWebhookRoundTrip(t *testing.T) {
	svc := newWebhookRoundTripFake()
	srv := instancesServer(t, svc)

	rec := serveJSON(t, srv, http.MethodPost, "/instances",
		`{"name":"loja","webhook":{"url":"https://hooks.example.com/wzap","enabled":true,"events":["message","receipt"]}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(svc.createInputs) != 1 {
		t.Fatalf("Create calls = %d, want 1", len(svc.createInputs))
	}
	sent := svc.createInputs[0]
	if sent.WebhookURL == nil || *sent.WebhookURL != "https://hooks.example.com/wzap" {
		t.Errorf("Create webhook url = %v, want the configured URL", sent.WebhookURL)
	}
	if sent.WebhookEnabled == nil || !*sent.WebhookEnabled {
		t.Errorf("Create webhook enabled = %v, want an explicit true", sent.WebhookEnabled)
	}
	if sent.WebhookEvents == nil || !reflect.DeepEqual(*sent.WebhookEvents, []string{"message", "receipt"}) {
		t.Errorf("Create webhook events = %v, want [message receipt]", sent.WebhookEvents)
	}

	data := webhookData(t, rec.Body.Bytes())
	if data["url"] != "https://hooks.example.com/wzap" {
		t.Errorf("data.instance.integration.webhook.url = %v, want the configured URL", data["url"])
	}
	if data["enabled"] != true {
		t.Errorf("data.instance.integration.webhook.enabled = %v, want true", data["enabled"])
	}
	if !reflect.DeepEqual(data["events"], []any{"message", "receipt"}) {
		t.Errorf("data.instance.integration.webhook.events = %v, want [message receipt]", data["events"])
	}
	id, _ := instanceData(t, rec.Body.Bytes())["id"].(string)

	got := serveJSON(t, srv, http.MethodGet, "/instances/"+id, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d (body %q)", got.Code, http.StatusOK, got.Body.String())
	}
	getData := webhookData(t, got.Body.Bytes())
	if getData["url"] != "https://hooks.example.com/wzap" {
		t.Errorf("get webhook.url = %v, want the configured URL", getData["url"])
	}
	if getData["enabled"] != true {
		t.Errorf("get webhook.enabled = %v, want true", getData["enabled"])
	}
	if !reflect.DeepEqual(getData["events"], []any{"message", "receipt"}) {
		t.Errorf("get webhook.events = %v, want [message receipt]", getData["events"])
	}
}

func TestInstancesUpdateWebhookDisablesAndNarrows(t *testing.T) {
	svc := newWebhookRoundTripFake()
	srv := instancesServer(t, svc)

	created := serveJSON(t, srv, http.MethodPost, "/instances",
		`{"name":"loja","webhook":{"url":"https://hooks.example.com/wzap","enabled":true}}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d (body %q)", created.Code, http.StatusCreated, created.Body.String())
	}
	id, _ := instanceData(t, created.Body.Bytes())["id"].(string)

	updated := serveJSON(t, srv, http.MethodPatch, "/instances/"+id,
		`{"webhook":{"enabled":false,"events":["receipt"]}}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d (body %q)", updated.Code, http.StatusOK, updated.Body.String())
	}
	if len(svc.updateInputs) != 1 {
		t.Fatalf("Update calls = %d, want 1", len(svc.updateInputs))
	}
	sent := svc.updateInputs[0]
	if sent.WebhookEnabled == nil || *sent.WebhookEnabled {
		t.Errorf("Update webhook enabled = %v, want an explicit false", sent.WebhookEnabled)
	}
	if sent.WebhookEvents == nil || !reflect.DeepEqual(*sent.WebhookEvents, []string{"receipt"}) {
		t.Errorf("Update webhook events = %v, want [receipt]", sent.WebhookEvents)
	}
	if sent.WebhookURL != nil {
		t.Errorf("Update webhook url = %v, want nil (absent keeps the stored URL)", sent.WebhookURL)
	}

	data := webhookData(t, updated.Body.Bytes())
	if data["url"] != "https://hooks.example.com/wzap" {
		t.Errorf("data.instance.integration.webhook.url = %v, want the stored URL kept", data["url"])
	}
	if data["enabled"] != false {
		t.Errorf("data.instance.integration.webhook.enabled = %v, want false", data["enabled"])
	}
	if !reflect.DeepEqual(data["events"], []any{"receipt"}) {
		t.Errorf("data.instance.integration.webhook.events = %v, want [receipt]", data["events"])
	}
}

// A flat webhook_url field is ignored by the nested contract: the write goes
// through with the defaults and the stored config stays unset.
func TestInstancesCreateFlatWebhookFieldsIgnored(t *testing.T) {
	svc := &fakeInstanceService{createFn: echoCreateFn("k")}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPost, "/instances",
		`{"name":"loja","webhook_url":"https://hooks.example.com/wzap","webhook_enabled":true}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(svc.createInputs) != 1 {
		t.Fatalf("Create calls = %d, want 1", len(svc.createInputs))
	}
	if svc.createInputs[0].WebhookURL != nil || svc.createInputs[0].WebhookEnabled != nil || svc.createInputs[0].WebhookEvents != nil {
		t.Errorf("Create webhook input = %+v, want all nil (flat fields are ignored)", svc.createInputs[0])
	}
}

func TestInstancesCreateInvalidWebhookUnprocessable(t *testing.T) {
	svc := &fakeInstanceService{createFn: func(context.Context, instance.CreateInput) (*model.Instance, string, error) {
		return nil, "", instance.ErrInvalidWebhook
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPost, "/instances",
		`{"name":"loja","webhook":{"url":"http://example.com/hook"}}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
		t.Errorf("error code = %q, want %q", code, "unprocessable_entity")
	}
}

// A 422 on update keeps the previous configuration: the failed write stores
// nothing, so reading the instance back shows the old webhook.
func TestInstancesUpdateInvalidWebhookKeepsPrevious(t *testing.T) {
	svc := newWebhookRoundTripFake()
	srv := instancesServer(t, svc)

	created := serveJSON(t, srv, http.MethodPost, "/instances",
		`{"name":"loja","webhook":{"url":"https://hooks.example.com/wzap","enabled":true,"events":["message"]}}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d (body %q)", created.Code, http.StatusCreated, created.Body.String())
	}
	id, _ := instanceData(t, created.Body.Bytes())["id"].(string)

	svc.updateFn = func(_ context.Context, _ uuid.UUID, _ instance.UpdateInput) (*model.Instance, error) {
		return nil, instance.ErrInvalidWebhook
	}
	bad := serveJSON(t, srv, http.MethodPatch, "/instances/"+id,
		`{"webhook":{"url":"http://example.com/hook"}}`)
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update status = %d, want %d (body %q)", bad.Code, http.StatusUnprocessableEntity, bad.Body.String())
	}
	if code := errorCode(t, bad.Body.Bytes()); code != "unprocessable_entity" {
		t.Errorf("error code = %q, want %q", code, "unprocessable_entity")
	}

	got := serveJSON(t, srv, http.MethodGet, "/instances/"+id, "")
	if got.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d (body %q)", got.Code, http.StatusOK, got.Body.String())
	}
	data := webhookData(t, got.Body.Bytes())
	if data["url"] != "https://hooks.example.com/wzap" {
		t.Errorf("get webhook.url = %v, want the previous URL kept", data["url"])
	}
	if !reflect.DeepEqual(data["events"], []any{"message"}) {
		t.Errorf("get webhook.events = %v, want the previous [message] kept", data["events"])
	}
}
