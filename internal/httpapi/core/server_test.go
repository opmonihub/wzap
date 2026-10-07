package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"wzap/internal/httpapi/core"
)

func TestJSONNoContentHasNoBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/instances", nil)

	core.JSON(rec, req, http.StatusNoContent, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}
func TestJSONEchoesRequestIDFromContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	req = req.WithContext(context.WithValue(req.Context(), core.RequestIDKey, "ctx-id-9"))
	rec := httptest.NewRecorder()

	core.JSON(rec, req, http.StatusOK, map[string]string{"id": "abc"})

	if got := rec.Header().Get("X-Request-Id"); got != "ctx-id-9" {
		t.Errorf("X-Request-Id = %q, want ctx-id-9", got)
	}
}
func TestErrorEchoesRequestIDFromContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	req = req.WithContext(context.WithValue(req.Context(), core.RequestIDKey, "ctx-id-7"))
	rec := httptest.NewRecorder()

	core.Error(rec, req, http.StatusBadRequest, "invalid_request", "bad input")

	if got := rec.Header().Get("X-Request-Id"); got != "ctx-id-7" {
		t.Errorf("X-Request-Id = %q, want %q", got, "ctx-id-7")
	}
}
