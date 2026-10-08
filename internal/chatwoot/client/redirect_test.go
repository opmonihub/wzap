package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAuthenticatedClientRejectsRedirect(t *testing.T) {
	for _, tlsOrigin := range []bool{false, true} {
		name := "another-origin"
		if tlsOrigin {
			name = "https-to-http"
		}
		t.Run(name, func(t *testing.T) {
			var destinationCalls atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				destinationCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"payload":[]}`))
			}))
			t.Cleanup(destination.Close)
			redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("api_access_token") != "synthetic-token" {
					t.Error("configured origin did not receive authentication")
				}
				http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
			})
			origin := httptest.NewServer(redirect)
			if tlsOrigin {
				origin.Close()
				origin = httptest.NewTLSServer(redirect)
			}
			t.Cleanup(origin.Close)
			c := New(origin.URL, "synthetic-token", "1")
			c.http.Transport = origin.Client().Transport
			_, err := c.ListInboxes(context.Background())
			var statusErr *Error
			if !errors.As(err, &statusErr) || statusErr.Status != http.StatusTemporaryRedirect {
				t.Errorf("redirect error = %v, want status 307", err)
			}
			if got := destinationCalls.Load(); got != 0 {
				t.Errorf("secondary origin received %d requests", got)
			}
		})
	}
}
