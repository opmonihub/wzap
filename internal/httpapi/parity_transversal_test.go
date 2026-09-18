package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestParityRequiresAuth(t *testing.T) {
	srv := instancesServer(t, &fakeInstanceService{})
	req := httptest.NewRequest(http.MethodGet, "/instances/"+uuid.NewString()+"/blocklist", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %q)", rec.Code, rec.Body.String())
	}
}

func TestParityEnvelopeAndRequestID(t *testing.T) {
	id := uuid.New()
	svc := &fakeInstanceService{
		getBlocklistFn: func(context.Context, uuid.UUID) ([]string, error) { return []string{}, nil },
	}
	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/"+id.String()+"/blocklist", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatalf("X-Request-Id missing")
	}
	var env struct {
		Data any `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &env)
}
