package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"wzap/internal/chatwoot/inbound"
	"wzap/internal/config"
	"wzap/internal/instance"
	"wzap/internal/model"
)

// aliasFixture keeps name lookups exact and live so renames cannot be hidden by a cache.
func aliasFixture(t *testing.T) (*rbacFixture, *fakeInstanceService, *fakeMessageService, *fakeIdempotency, *http.Server) {
	t.Helper()
	f := newRBACFixture(t)
	f.instA.Name = "Loja_SP-1"
	svc := f.rbacInstances()
	svc.getByNameFn = func(_ context.Context, name string) (*model.Instance, error) {
		for _, row := range f.instancesByID() {
			if row.Name == name {
				return row, nil
			}
		}
		return nil, instance.ErrNotFound
	}
	messages := f.rbacMessages()
	repo := newFakeIdempotency()
	srv := New(config.Config{APIKey: testToken, MaxMediaBytes: testMaxMediaBytes}, zerolog.Nop(), Deps{Instances: svc, Messages: messages, Idempotency: repo, Keys: f.keys, JWTSecret: testJWTSecret})
	return f, svc, messages, repo, srv
}

func TestInstanceReferencesResolveAtRouter(t *testing.T) {
	f, svc, messages, _, srv := aliasFixture(t)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "", "", 200}, {"GET", "/status", "", 200},
		{"PATCH", "", `{"external_ref":"crm"}`, 200},
		{"POST", "/connect", "", 200},
		{"POST", "/messages/text", `{"to":"5547988359190","text":"hello"}`, 202},
		{"GET", "/messages", "", 200},
	} {
		rec := serveRBAC(t, srv, tc.method, "/instances/"+f.instA.Name+tc.path, tc.body, rbacSessionCookie(f.userATok), "", nil)
		if rec.Code != tc.status {
			t.Errorf("%s %s status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	for _, id := range svc.getIDs {
		if id != f.instA.ID {
			t.Errorf("handler received %s", id)
		}
	}
	if len(messages.enqueueCalls) != 1 || messages.enqueueCalls[0].instanceID != f.instA.ID {
		t.Errorf("enqueue calls=%v", messages.enqueueCalls)
	}
}

func TestInstanceReferenceErrorsAndUUIDPrecedence(t *testing.T) {
	f, svc, _, _, srv := aliasFixture(t)
	for _, ref := range []string{"loja_SP-1", "missing", "bad.name", "stats"} {
		suffix := ""
		if ref == "stats" {
			suffix = "/status"
		}
		rec := serveJSON(t, srv, "GET", "/instances/"+ref+suffix, "")
		if rec.Code != 404 {
			t.Errorf("ref %q status=%d", ref, rec.Code)
		}
	}
	svc.getByNameFn = func(context.Context, string) (*model.Instance, error) { return nil, instance.ErrInstanceNameAmbiguous }
	rec := serveJSON(t, srv, "GET", "/instances/duplicate", "")
	if rec.Code != 409 || errorCode(t, rec.Body.Bytes()) != "instance_name_ambiguous" {
		t.Errorf("ambiguous=%d %s", rec.Code, rec.Body.String())
	}
	svc.getNames = nil
	for _, ref := range []string{f.instA.ID.String(), strings.ReplaceAll(f.instA.ID.String(), "-", ""), "x" + f.instA.ID.String() + "y"} {
		rec = serveJSON(t, srv, "GET", "/instances/"+ref, "")
		if rec.Code != 200 {
			t.Errorf("UUID %q=%d", ref, rec.Code)
		}
	}
	if len(svc.getNames) != 0 {
		t.Errorf("UUID namespace queried names: %v", svc.getNames)
	}
}

func TestInstanceKeyNameResolutionDoesNotProbeForeignNames(t *testing.T) {
	f, svc, _, _, srv := aliasFixture(t)
	for _, ref := range []string{f.instB.Name, "missing", f.instB.ID.String()} {
		rec := serveRBAC(t, srv, "GET", "/instances/"+ref, "", nil, f.keyA, nil)
		if rec.Code != 403 {
			t.Errorf("foreign %q=%d body=%s", ref, rec.Code, rec.Body.String())
		}
	}
	rec := serveRBAC(t, srv, "GET", "/instances/"+f.instA.Name, "", nil, f.keyA, nil)
	if rec.Code != 200 {
		t.Errorf("own name=%d %s", rec.Code, rec.Body.String())
	}
	if len(svc.getNames) != 0 {
		t.Errorf("instance key queried foreign namespace: %v", svc.getNames)
	}
	for _, id := range svc.getIDs {
		if id != f.instA.ID {
			t.Errorf("probed foreign row=%s", id)
		}
	}
}

func TestAliasReplayUsesCanonicalUUIDAndCurrentAccess(t *testing.T) {
	for _, firstByName := range []bool{true, false} {
		t.Run(map[bool]string{true: "name-first", false: "uuid-first"}[firstByName], func(t *testing.T) {
			f, _, messages, repo, srv := aliasFixture(t)
			refs := []string{f.instA.ID.String(), f.instA.Name}
			if firstByName {
				refs[0], refs[1] = refs[1], refs[0]
			}
			headers := map[string]string{"Idempotency-Key": "same-send"}
			body := `{"to":"5547988359190","text":"hello"}`
			for i, ref := range refs {
				rec := serveRBAC(t, srv, "POST", "/instances/"+ref+"/messages/text", body, rbacSessionCookie(f.userATok), "", headers)
				if rec.Code != 202 {
					t.Fatalf("send=%d %s", rec.Code, rec.Body.String())
				}
				if i == 1 && rec.Header().Get("X-Idempotent-Replay") != "true" {
					t.Error("alias did not replay")
				}
			}
			if len(messages.enqueueCalls) != 1 {
				t.Errorf("enqueue=%d", len(messages.enqueueCalls))
			}
			acquired := len(repo.acquires)
			for _, ref := range refs {
				for _, credential := range []string{"foreign-user", "foreign-key", "revoked-owner"} {
					cookie := rbacSessionCookie(f.userBTok)
					key := ""
					if credential == "foreign-key" {
						cookie = nil
						key = f.keyB
					}
					if credential == "revoked-owner" {
						f.instA.OwnerUserID = &f.userB
						cookie = rbacSessionCookie(f.userATok)
					}
					rec := serveRBAC(t, srv, "POST", "/instances/"+ref+"/messages/text", body, cookie, key, headers)
					if rec.Code != 403 || rec.Header().Get("X-Idempotent-Replay") != "" {
						t.Errorf("%s %s=%d %s", credential, ref, rec.Code, rec.Body.String())
					}
					f.instA.OwnerUserID = &f.userA
				}
			}
			if len(repo.acquires) != acquired {
				t.Error("denied replay acquired cache")
			}
			old := f.instA.Name
			f.instA.Name = "Renamed"
			rec := serveRBAC(t, srv, "POST", "/instances/"+old+"/messages/text", body, rbacSessionCookie(f.userATok), "", headers)
			if rec.Code != 404 {
				t.Errorf("old name=%d", rec.Code)
			}
			rec = serveRBAC(t, srv, "POST", "/instances/Renamed/messages/text", body, rbacSessionCookie(f.userATok), "", headers)
			if rec.Code != 202 || rec.Header().Get("X-Idempotent-Replay") != "true" {
				t.Errorf("renamed replay=%d %s", rec.Code, rec.Body.String())
			}
			if len(messages.enqueueCalls) != 1 {
				t.Error("rename repeated operation")
			}
		})
	}
}

func TestPublicChatwootNameReference(t *testing.T) {
	f, svc, _, _, _ := aliasFixture(t)
	srv := chatwootTestServer(t, svc, &fakeChatwootConfigs{}, config.Chatwoot{})
	rec := serveRBAC(t, srv, "POST", "/chatwoot/webhook/"+f.instA.Name, `{}`, nil, "", nil)
	if rec.Code != 400 || errorCode(t, rec.Body.Bytes()) != "chatwoot_disabled" {
		t.Errorf("open alias=%d %s", rec.Code, rec.Body.String())
	}
	if len(svc.getIDs) != 1 || svc.getIDs[0] != f.instA.ID {
		t.Errorf("webhook target=%v", svc.getIDs)
	}
}

func TestTargetedInstanceStats(t *testing.T) {
	f, svc, _, _, srv := aliasFixture(t)
	for _, status := range []string{"connected", "disconnected", "pairing", "error", "unknown"} {
		f.instA.Connection.Status = status
		for _, ref := range []string{f.instA.Name, f.instA.ID.String()} {
			rec := serveRBAC(t, srv, "GET", "/instances/stats?instance="+ref, "", rbacSessionCookie(f.adminTok), "", nil)
			if rec.Code != 200 {
				t.Fatalf("target=%d %s", rec.Code, rec.Body.String())
			}
			total, buckets := statsTotals(t, rec.Body.Bytes())
			bucket := status
			if status == "unknown" {
				bucket = "disconnected"
			}
			sum := 0
			for _, count := range buckets {
				sum += count
			}
			if total != 1 || sum != 1 || len(buckets) != 4 || buckets[bucket] != 1 {
				t.Errorf("stats=%d %v", total, buckets)
			}
		}
	}
	for _, tc := range []struct {
		ref, token, key string
		status          int
	}{
		{f.instB.Name, f.userATok, "", 403}, {f.instB.ID.String(), f.userATok, "", 403},
		{"missing", f.userATok, "", 404}, {"bad.name", f.userATok, "", 404},
		{"missing", "", f.keyA, 403}, {f.instA.Name, "", f.keyA, 403},
	} {
		before := len(svc.getIDs) + len(svc.getNames)
		var cookie *http.Cookie
		if tc.token != "" {
			cookie = rbacSessionCookie(tc.token)
		}
		rec := serveRBAC(t, srv, "GET", "/instances/stats?instance="+tc.ref, "", cookie, tc.key, nil)
		if rec.Code != tc.status {
			t.Errorf("ref %s=%d", tc.ref, rec.Code)
		}
		if tc.key != "" && len(svc.getIDs)+len(svc.getNames) != before {
			t.Error("collection denial looked up target")
		}
	}
	if svc.listCalls != 0 {
		t.Error("targeted stats listed collection")
	}
}

func TestInstanceNameErrorsHaveTypedHTTPCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{instance.ErrInvalidInstanceName, 422, "invalid_instance_name"},
		{instance.ErrInstanceNameTaken, 409, "instance_name_taken"},
	} {
		svc := &fakeInstanceService{createFn: func(context.Context, instance.CreateInput) (*model.Instance, string, error) { return nil, "", tc.err }, updateFn: func(context.Context, uuid.UUID, instance.UpdateInput) (*model.Instance, error) { return nil, tc.err }}
		srv := instancesServer(t, svc)
		for _, method := range []string{"POST", "PATCH"} {
			path := "/instances"
			if method == "PATCH" {
				path += "/" + uuid.NewString()
			}
			rec := serveJSON(t, srv, method, path, `{"name":"newname"}`)
			if rec.Code != tc.status || errorCode(t, rec.Body.Bytes()) != tc.code {
				t.Errorf("%s=%d %s", method, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestUUIDReplayRechecksOwnershipBeforeCache(t *testing.T) {
	f, _, messages, repo, srv := aliasFixture(t)
	path := "/instances/" + f.instA.ID.String() + "/messages/text"
	headers := map[string]string{"Idempotency-Key": "warm-uuid"}
	body := `{"to":"5547988359190","text":"hello"}`
	rec := serveRBAC(t, srv, "POST", path, body, rbacSessionCookie(f.userATok), "", headers)
	if rec.Code != 202 {
		t.Fatalf("warm=%d %s", rec.Code, rec.Body.String())
	}
	f.instA.OwnerUserID = &f.userB
	rec = serveRBAC(t, srv, "POST", path, body, rbacSessionCookie(f.userATok), "", headers)
	if rec.Code != 403 || rec.Header().Get("X-Idempotent-Replay") != "" {
		t.Errorf("revoked replay=%d %s", rec.Code, rec.Body.String())
	}
	if len(repo.acquires) != 1 || len(messages.enqueueCalls) != 1 {
		t.Errorf("cache acquires=%d enqueue=%d", len(repo.acquires), len(messages.enqueueCalls))
	}
}

func TestRenameChangesOnlyTheAlias(t *testing.T) {
	f, svc, _, _, srv := aliasFixture(t)
	svc.updateFn = func(_ context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error) {
		if id != f.instA.ID {
			t.Errorf("update target=%s", id)
		}
		if input.Name != nil {
			f.instA.Name = *input.Name
		}
		return f.instA, nil
	}
	old := f.instA.Name
	rec := serveRBAC(t, srv, "PATCH", "/instances/"+old, `{"name":"Renamed"}`, rbacSessionCookie(f.userATok), "", nil)
	if rec.Code != 200 {
		t.Fatalf("rename=%d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		ref    string
		status int
	}{{old, 404}, {"Renamed", 200}, {f.instA.ID.String(), 200}} {
		rec = serveRBAC(t, srv, "GET", "/instances/"+tc.ref, "", rbacSessionCookie(f.userATok), "", nil)
		if rec.Code != tc.status {
			t.Errorf("ref %s=%d %s", tc.ref, rec.Code, rec.Body.String())
		}
		if tc.status == 200 {
			var payload struct {
				Data instanceEnvelope `json:"data"`
			}
			decodeJSON(t, rec.Body.Bytes(), &payload)
			if payload.Data.Instance.ID != f.instA.ID.String() || payload.Data.Instance.Name != "Renamed" {
				t.Errorf("renamed response=%+v", payload.Data)
			}
		}
	}
}

func TestInstanceResolverLeavesStaticAndOtherIDsAlone(t *testing.T) {
	_, svc, _, _, srv := aliasFixture(t)
	for _, tc := range []struct {
		path   string
		status int
	}{{"/healthz", 200}, {"/swagger/doc.json", 200}, {"/instances/stats", 200}, {"/users/Loja_SP-1", 404}, {"/media/Loja_SP-1", 404}} {
		rec := serveJSON(t, srv, "GET", tc.path, "")
		if rec.Code != tc.status {
			t.Errorf("path %s=%d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if len(svc.getNames) != 0 || len(svc.getIDs) != 0 {
		t.Errorf("unrelated paths resolved instances: names=%v ids=%v", svc.getNames, svc.getIDs)
	}
}

// Captures the identity received by the connector rather than resolving aliases itself.
type aliasWebhookInbound struct {
	fakeChatwootInbound
	ids []uuid.UUID
}

func (f *aliasWebhookInbound) Handle(_ context.Context, id uuid.UUID, _ inbound.Payload) (int, error) {
	f.ids = append(f.ids, id)
	return 200, nil
}

func TestPublicWebhookAliasesShareCanonicalLimiterAndConnector(t *testing.T) {
	f, svc, _, _, _ := aliasFixture(t)
	connector := &aliasWebhookInbound{}
	srv := New(config.Config{APIKey: testToken}, zerolog.Nop(), Deps{
		Instances: svc, Chatwoot: chatwootOn(), ChatwootInbound: connector,
		ChatwootWebhookLimiter: NewChatwootRateLimiter(1, 0),
	})
	rec := serveRBAC(t, srv, "POST", "/chatwoot/webhook/"+f.instA.Name, `{}`, nil, "", nil)
	if rec.Code != 200 {
		t.Fatalf("open webhook=%d %s", rec.Code, rec.Body.String())
	}
	rec = serveRBAC(t, srv, "POST", "/chatwoot/webhook/"+f.instA.ID.String(), `{}`, nil, "", nil)
	if rec.Code != 429 {
		t.Errorf("UUID should share name limiter=%d %s", rec.Code, rec.Body.String())
	}
	if len(connector.ids) != 1 || connector.ids[0] != f.instA.ID {
		t.Errorf("connector identities=%v", connector.ids)
	}
}
