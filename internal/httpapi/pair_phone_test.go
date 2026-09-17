package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/instance"
	"wzap/internal/model"
)

func TestPairPhone(t *testing.T) {
	t.Run("open channel answers pairing code and expiry", func(t *testing.T) {
		id := uuid.New()
		expiresAt := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)
		svc := &fakeInstanceService{
			pairPhoneFn: func(context.Context, uuid.UUID, string) (instance.PairPhoneResult, error) {
				return instance.PairPhoneResult{Code: "12345678", ExpiresAt: expiresAt}, nil
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+id.String()+"/pair-phone",
			`{"phone":"5547988359190"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Data pairPhoneResponse `json:"data"`
		}
		decodeJSON(t, rec.Body.Bytes(), &payload)
		if payload.Data.PairingCode != "12345678" {
			t.Errorf("data.pairing_code = %q, want %q", payload.Data.PairingCode, "12345678")
		}
		if !payload.Data.ExpiresAt.Equal(expiresAt) {
			t.Errorf("data.expires_at = %q, want %q", payload.Data.ExpiresAt, expiresAt)
		}
		if len(svc.pairPhoneCalls) != 1 {
			t.Fatalf("PairPhone calls = %d, want 1", len(svc.pairPhoneCalls))
		}
		call := svc.pairPhoneCalls[0]
		if call.InstanceID != id || call.Phone != "5547988359190" {
			t.Errorf("PairPhone call = %+v, want instance %s phone 5547988359190", call, id)
		}
	})

	t.Run("no open channel answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			pairPhoneFn: func(context.Context, uuid.UUID, string) (instance.PairPhoneResult, error) {
				return instance.PairPhoneResult{}, instance.ErrNoPairingChannel
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/pair-phone",
			`{"phone":"5547988359190"}`)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
		if code := errorCode(t, rec.Body.Bytes()); code != "conflict" {
			t.Errorf("error code = %q, want %q", code, "conflict")
		}
	})

	t.Run("already connected answers conflict", func(t *testing.T) {
		svc := &fakeInstanceService{
			pairPhoneFn: func(context.Context, uuid.UUID, string) (instance.PairPhoneResult, error) {
				return instance.PairPhoneResult{}, instance.ErrAlreadyConnected
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/pair-phone",
			`{"phone":"5547988359190"}`)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})

	t.Run("invalid number answers generic unprocessable without touching the session", func(t *testing.T) {
		svc := &fakeInstanceService{}
		for _, body := range []string{`{"phone":""}`, `{"phone":"   "}`, `{}`} {
			rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
				"/instances/"+uuid.NewString()+"/pair-phone", body)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("body %q: status = %d, want %d", body, rec.Code, http.StatusUnprocessableEntity)
			}
			if code := errorCode(t, rec.Body.Bytes()); code != "unprocessable_entity" {
				t.Errorf("body %q: error code = %q, want %q", body, code, "unprocessable_entity")
			}
		}
		if len(svc.pairPhoneCalls) != 0 {
			t.Errorf("PairPhone calls = %d, want none on invalid numbers", len(svc.pairPhoneCalls))
		}
	})

	t.Run("unknown instance answers not found", func(t *testing.T) {
		svc := &fakeInstanceService{
			getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
				return nil, instance.ErrNotFound
			},
		}
		rec := serveJSON(t, instancesServer(t, svc), http.MethodPost,
			"/instances/"+uuid.NewString()+"/pair-phone",
			`{"phone":"5547988359190"}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if len(svc.pairPhoneCalls) != 0 {
			t.Errorf("PairPhone calls = %d, want none on unknown instance", len(svc.pairPhoneCalls))
		}
	})
}
