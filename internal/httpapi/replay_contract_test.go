package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
)

func TestReplayContractLegacySends(t *testing.T) {
	instanceID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	messageID := "22222222-2222-4222-8222-222222222222"
	for _, suffix := range []string{"messages", "messages/text", "messages/location", "messages/contact", "messages/media"} {
		t.Run(suffix, func(t *testing.T) {
			body := `{"data":{"message_id":"` + messageID + `","status":"queued","owner_user_id":"private","api_key_hash":"secret"}}`
			rec, repo, calls := contractReplay(t, instanceID, suffix, body, http.StatusAccepted)
			if rec.Code != http.StatusAccepted || calls != 0 || len(repo.releases) != 0 || len(repo.completes) != 0 {
				t.Fatalf("replay = %d, calls=%d releases=%d completes=%d: %s", rec.Code, calls, len(repo.releases), len(repo.completes), rec.Body.String())
			}
			var payload struct {
				Data struct {
					Message map[string]any `json:"message"`
				} `json:"data"`
			}
			decodeJSON(t, rec.Body.Bytes(), &payload)
			if payload.Data.Message["id"] != messageID || payload.Data.Message["send_status"] != "queued" {
				t.Errorf("converted message = %v, want same UUID and queued status", payload.Data.Message)
			}
			for _, forbidden := range []string{"message_id", "owner_user_id", "api_key_hash", "secret"} {
				if strings.Contains(rec.Body.String(), forbidden) {
					t.Errorf("replay leaks %s: %s", forbidden, rec.Body.String())
				}
			}
			if rec.Header().Get(idempotentReplayHeader) != "true" || rec.Header().Get(requestIDHeader) != "contract-replay" {
				t.Errorf("missing replay/request headers: %v", rec.Header())
			}
			if string(repo.records[fakeRecordKey(instanceID, "contract")].ResponseBody) != body {
				t.Error("replay mutated the stored response")
			}
		})
	}
}

func TestReplayContractUnconvertibleSend(t *testing.T) {
	for _, body := range []string{
		`{"data":null}`, `{"data":{}}`, `{"data":{"message_id":"not-a-uuid","status":"queued"}}`,
		`{"data":{"message_id":"22222222-2222-4222-8222-222222222222","status":"sent"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			rec, repo, calls := contractReplay(t, uuid.New(), "messages/text", body, http.StatusAccepted)
			if rec.Code != http.StatusGone || calls != 0 || len(repo.releases) != 0 {
				t.Fatalf("replay = %d calls=%d releases=%d: %s", rec.Code, calls, len(repo.releases), rec.Body.String())
			}
			if errorCode(t, rec.Body.Bytes()) != "idempotency_response_expired" {
				t.Fatal(rec.Body.String())
			}
		})
	}
}

func TestReplayContractUnchangedCommands(t *testing.T) {
	for _, suffix := range []string{"status/updates", "status/updates/media", "messages/edit"} {
		t.Run(suffix, func(t *testing.T) {
			body := `{"data":{"message_id":"upstream-wa-id","status":"published"}}`
			rec, _, calls := contractReplay(t, uuid.New(), suffix, body, http.StatusAccepted)
			if rec.Code != http.StatusAccepted || calls != 0 || rec.Body.String() != body {
				t.Fatalf("command changed: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestReplayContractLegacyChannel(t *testing.T) {
	body := `{"data":{"channel":"123@newsletter","title":"News","follower_count":2,"updated_at":"2026-10-06T10:00:00Z","token":"hidden"}}`
	rec, _, calls := contractReplay(t, uuid.New(), "newsletters", body, http.StatusCreated)
	var payload struct {
		Data struct {
			Channel newsletterResponse `json:"channel"`
		} `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if rec.Code != http.StatusCreated || calls != 0 || payload.Data.Channel.Channel != "123@newsletter" || strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("channel replay = %d %s", rec.Code, rec.Body.String())
	}
}

// Runs through the real middleware, so conversion must neither call the effect
// nor release/complete the cache entry. Request IDs use the production wrapper.
func contractReplay(t *testing.T, instanceID uuid.UUID, suffix, body string, status int) (*httptest.ResponseRecorder, *fakeIdempotency, int) {
	t.Helper()
	req := idempotencyRequest(instanceID, "contract", `{}`)
	req.URL.Path = "/instances/" + instanceID.String() + "/" + suffix
	fingerprint, cleanup, err := fingerprintRequest(req, testMultipartLimit)
	if err != nil {
		t.Fatal(err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	repo := newFakeIdempotency()
	repo.putRecord(model.IdempotencyRecord{InstanceID: instanceID, Key: "contract", Fingerprint: fingerprint, Status: "completed", ResponseStatus: status, ResponseBody: json.RawMessage(body)})
	calls := 0
	rec := httptest.NewRecorder()
	req.Header.Set(requestIDHeader, "contract-replay")
	RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Idempotency(repo, zerolog.Nop(), testMultipartBytes)(countingHandler(&calls, http.StatusAccepted, "new")).ServeHTTP(w, r)
	})).ServeHTTP(rec, req)
	return rec, repo, calls
}
