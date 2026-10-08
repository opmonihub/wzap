package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/httpapi/core"
)

func TestIdempotencyRegisteredPUTAndPATCH(t *testing.T) {
	for _, tc := range []struct {
		name, method, suffix, body, changed string
		calls                               func(*fakeInstanceService) int
	}{
		{"chat timer", "PUT", "/chats/5511999999999@s.whatsapp.net/disappearing", `{"duration":"24h"}`, `{"duration":"168h"}`, func(s *fakeInstanceService) int { return len(s.setDisappearingTimerCalls) }},
		{"default timer", "PUT", "/chats/default-disappearing", `{"duration":"24h"}`, `{"duration":"168h"}`, func(s *fakeInstanceService) int { return len(s.setDefaultDisappearingCalls) }},
		{"group settings", "PATCH", "/groups/123@g.us/settings", `{"announce":true}`, `{"announce":false}`, func(s *fakeInstanceService) int { return len(s.updateGroupSettingsCalls) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			svc := &fakeInstanceService{}
			repo := newFakeIdempotency()
			srv := httpapi.New(config.Config{APIKey: testToken}, zerolog.Nop(), httpapi.Deps{Instances: svc, Idempotency: repo})
			path := "/instances/" + id.String() + tc.suffix
			headers := map[string]string{core.IdempotencyKeyHeader: "setter"}
			first := serveMessages(t, srv, tc.method, path, tc.body, headers)
			if first.Code != http.StatusOK {
				t.Fatalf("first status = %d, body %q", first.Code, first.Body.String())
			}
			second := serveMessages(t, srv, tc.method, path, tc.body, headers)
			if second.Code != first.Code || second.Body.String() != first.Body.String() || second.Header().Get(core.IdempotentReplayHeader) != "true" {
				t.Errorf("same setter was not replayed: status %d, body %q, replay %q", second.Code, second.Body.String(), second.Header().Get(core.IdempotentReplayHeader))
			}
			divergent := serveMessages(t, srv, tc.method, path, tc.changed, headers)
			if divergent.Code != http.StatusUnprocessableEntity {
				t.Errorf("divergent status = %d, want 422", divergent.Code)
			}
			if tc.calls(svc) != 1 {
				t.Errorf("setter calls = %d, want 1", tc.calls(svc))
			}
			if record, ok := repo.records[fakeRecordKey(id, "setter")]; ok {
				record.Status = "in_progress"
			}
			inFlight := serveMessages(t, srv, tc.method, path, tc.body, headers)
			if inFlight.Code != http.StatusConflict {
				t.Errorf("in-progress status = %d, want 409", inFlight.Code)
			}
			if tc.calls(svc) != 1 {
				t.Errorf("setter calls after in-progress retry = %d, want 1", tc.calls(svc))
			}
		})
	}
}

func TestIdempotencyConcreteResourcesDoNotShareReplay(t *testing.T) {
	for _, tc := range []struct {
		name, method, first, second, body string
		calls                             func(*fakeInstanceService) int
	}{
		{"channels", "POST", "/newsletters/123@newsletter/mute", "/newsletters/456@newsletter/mute", `{"muted":true}`, func(s *fakeInstanceService) int { return len(s.muteNewsletterCalls) }},
		{"group requests", "POST", "/groups/123@g.us/requests", "/groups/456@g.us/requests", `{"action":"approve","participants":["5511999999999@s.whatsapp.net"]}`, func(s *fakeInstanceService) int { return len(s.updateGroupRequestsCalls) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			svc := &fakeInstanceService{}
			srv := httpapi.New(config.Config{APIKey: testToken}, zerolog.Nop(), httpapi.Deps{Instances: svc, Idempotency: newFakeIdempotency()})
			headers := map[string]string{core.IdempotencyKeyHeader: "resource"}
			first := serveMessages(t, srv, tc.method, "/instances/"+id.String()+tc.first, tc.body, headers)
			if first.Code != http.StatusOK {
				t.Fatalf("first status = %d, body %q", first.Code, first.Body.String())
			}
			second := serveMessages(t, srv, tc.method, "/instances/"+id.String()+tc.second, tc.body, headers)
			if second.Code != http.StatusUnprocessableEntity {
				t.Errorf("other resource status = %d, want 422", second.Code)
			}
			if tc.calls(svc) != 1 {
				t.Errorf("resource setter calls = %d, want 1", tc.calls(svc))
			}
		})
	}
}

func TestIdempotencyResourceAliasesSurviveRename(t *testing.T) {
	for _, tc := range []struct {
		name, method, suffix, body string
		calls                      func(*fakeInstanceService) int
	}{
		{"channel", "POST", "/newsletters/123@newsletter/mute", `{"muted":true}`, func(s *fakeInstanceService) int { return len(s.muteNewsletterCalls) }},
		{"chat", "PUT", "/chats/5511999999999@s.whatsapp.net/disappearing", `{"duration":"24h"}`, func(s *fakeInstanceService) int { return len(s.setDisappearingTimerCalls) }},
		{"group", "PATCH", "/groups/123@g.us/settings", `{"announce":true}`, func(s *fakeInstanceService) int { return len(s.updateGroupSettingsCalls) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, svc, _, repo, srv := aliasFixture(t)
			oldName := f.instA.Name
			headers := map[string]string{core.IdempotencyKeyHeader: "alias-resource"}
			first := serveMessages(t, srv, tc.method, "/instances/"+oldName+tc.suffix, tc.body, headers)
			if first.Code != http.StatusOK {
				t.Fatalf("first status = %d, body %q", first.Code, first.Body.String())
			}
			f.instA.Name = "Renamed"
			for _, ref := range []string{f.instA.ID.String(), f.instA.Name} {
				replay := serveMessages(t, srv, tc.method, "/instances/"+ref+tc.suffix, tc.body, headers)
				if replay.Code != first.Code || replay.Body.String() != first.Body.String() || replay.Header().Get(core.IdempotentReplayHeader) != "true" {
					t.Errorf("%s did not replay after rename: status %d, body %q", ref, replay.Code, replay.Body.String())
				}
			}
			acquired := len(repo.acquires)
			old := serveMessages(t, srv, tc.method, "/instances/"+oldName+tc.suffix, tc.body, headers)
			if old.Code != http.StatusNotFound {
				t.Errorf("old alias status = %d, want 404", old.Code)
			}
			foreign := serveRBAC(t, srv, tc.method, "/instances/"+f.instA.Name+tc.suffix, tc.body, nil, f.keyB, headers)
			if foreign.Code != http.StatusForbidden {
				t.Errorf("foreign key status = %d, want 403", foreign.Code)
			}
			if len(repo.acquires) != acquired {
				t.Error("denied reference acquired idempotency key")
			}
			if tc.calls(svc) != 1 {
				t.Errorf("setter calls = %d, want 1", tc.calls(svc))
			}
		})
	}
}
