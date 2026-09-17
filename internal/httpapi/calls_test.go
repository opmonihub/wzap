package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/instance"
)

// callServer builds the API server with the instance fake for the call
// routes.
func callServer(t *testing.T, svc *fakeInstanceService) *http.Server {
	t.Helper()
	return New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken},
		zerolog.Nop(),
		Deps{
			ReadyChecker: checkFunc(func(context.Context) error { return nil }),
			Instances:    svc,
		})
}

func TestCallReject(t *testing.T) {
	t.Run("active call answers rejected true", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeInstanceService{
			rejectCallFn: func(_ context.Context, instanceID uuid.UUID, fromJID, callID string) error {
				if instanceID != id {
					t.Errorf("RejectCall instance = %s, want %s", instanceID, id)
				}
				if fromJID != "5511888888888@s.whatsapp.net" || callID != "call-1" {
					t.Errorf("RejectCall = (%q, %q), want the caller and call id", fromJID, callID)
				}
				return nil
			},
		}
		rec := serveStatus(t, callServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/calls/reject",
			`{"call_id":"call-1","from":"5511888888888@s.whatsapp.net"}`, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data struct {
				Rejected bool `json:"rejected"`
			} `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if !payload.Data.Rejected {
			t.Error("data.rejected = false, want true")
		}
		if len(svc.rejectCallCalls) != 1 {
			t.Errorf("RejectCall calls = %d, want 1", len(svc.rejectCallCalls))
		}
	})

	t.Run("missing call id answers unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		rec := serveStatus(t, callServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/calls/reject",
			`{"from":"5511888888888@s.whatsapp.net"}`, nil)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
		if len(svc.rejectCallCalls) != 0 {
			t.Errorf("RejectCall calls = %d, want none on missing call id", len(svc.rejectCallCalls))
		}
	})

	t.Run("unsupported upstream answers not implemented", func(t *testing.T) {
		svc := &fakeInstanceService{
			rejectCallFn: func(context.Context, uuid.UUID, string, string) error {
				return instance.ErrUnsupported
			},
		}
		rec := serveStatus(t, callServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/calls/reject",
			`{"call_id":"call-1","from":"5511888888888@s.whatsapp.net"}`, nil)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotImplemented, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "not_supported" {
			t.Errorf("error code = %q, want not_supported", code)
		}
	})

	t.Run("disconnected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			rejectCallFn: func(context.Context, uuid.UUID, string, string) error {
				return instance.ErrNotConnected
			},
		}
		rec := serveStatus(t, callServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/calls/reject",
			`{"call_id":"call-1","from":"5511888888888@s.whatsapp.net"}`, nil)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})
}
