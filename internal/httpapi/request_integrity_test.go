package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/httpapi/core"
)

func TestJSONRequestMustEndWithinBodyLimit(t *testing.T) {
	const valid = `{"to":"5547988359190","text":"hello"}`
	for _, keyed := range []bool{false, true} {
		for _, tc := range []struct {
			name, body      string
			status, effects int
		}{
			{"second object", valid + `{}`, 400, 0},
			{"second scalar", valid + ` true`, 400, 0},
			{"trailing junk", valid + `junk`, 400, 0},
			{"oversized padding", valid + strings.Repeat(" ", core.MaxJSONBodyBytes), 413, 0},
			{"exact body limit", valid + strings.Repeat(" ", core.MaxJSONBodyBytes-len(valid)), 202, 1},
			{"trailing whitespace", valid + " \n\t", 202, 1},
			{"unknown field", `{"to":"5547988359190","text":"hello","extra":true}`, 202, 1},
		} {
			name := tc.name
			if keyed {
				name += " with key"
			}
			t.Run(name, func(t *testing.T) {
				svc := &fakeMessageService{}
				repo := newFakeIdempotency()
				headers := map[string]string{}
				if keyed {
					headers[core.IdempotencyKeyHeader] = "json-integrity"
				}
				rec := serveMessages(t, messagesServer(t, svc, repo), http.MethodPost,
					"/instances/"+uuid.NewString()+"/messages/text", tc.body, headers)
				if rec.Code != tc.status {
					t.Errorf("status = %d, want %d, body %q", rec.Code, tc.status, rec.Body.String())
				}
				if len(svc.enqueueCalls) != tc.effects {
					t.Errorf("enqueue calls = %d, want %d", len(svc.enqueueCalls), tc.effects)
				}
				if tc.status == http.StatusRequestEntityTooLarge && len(repo.acquires) != 0 {
					t.Errorf("oversized body acquired %d idempotency keys, want 0", len(repo.acquires))
				}
			})
		}
	}
}

func TestOversizedJSONDoesNotConsultExistingIdempotencyRecord(t *testing.T) {
	const valid = `{"to":"5547988359190","text":"hello"}`
	for _, state := range []string{"completed", "in_progress"} {
		t.Run(state, func(t *testing.T) {
			svc := &fakeMessageService{}
			repo := newFakeIdempotency()
			srv := messagesServer(t, svc, repo)
			id := uuid.New()
			path := "/instances/" + id.String() + "/messages/text"
			headers := map[string]string{core.IdempotencyKeyHeader: "prior-json"}
			first := serveMessages(t, srv, http.MethodPost, path, valid, headers)
			if first.Code != http.StatusAccepted {
				t.Fatalf("first status = %d, body %q", first.Code, first.Body.String())
			}
			record := repo.records[fakeRecordKey(id, "prior-json")]
			record.Status = state
			acquired := len(repo.acquires)
			oversized := serveMessages(t, srv, http.MethodPost, path, valid+strings.Repeat(" ", core.MaxJSONBodyBytes), headers)
			if oversized.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("oversized status = %d, want 413", oversized.Code)
			}
			if len(repo.acquires) != acquired {
				t.Error("oversized request consulted idempotency store")
			}
			if len(svc.enqueueCalls) != 1 {
				t.Error("oversized retry executed a second effect")
			}
			if record.Status != state || len(repo.releases) != 0 {
				t.Error("oversized request changed or released the original key")
			}
		})
	}
}
