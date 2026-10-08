package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDeliverRejectsCredentialRedirect(t *testing.T) {
	for _, tlsOrigin := range []bool{false, true} {
		name := "another-origin"
		if tlsOrigin {
			name = "https-to-http"
		}
		t.Run(name, func(t *testing.T) {
			var destinationCalls atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				destinationCalls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(destination.Close)
			redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("apikey") != "synthetic-token" {
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
			previous := webhookClient
			copyClient := *previous
			transport := previous.Transport.(*http.Transport).Clone()
			transport.TLSClientConfig = origin.Client().Transport.(*http.Transport).TLSClientConfig
			copyClient.Transport = transport
			webhookClient = &copyClient
			t.Cleanup(func() { webhookClient = previous; transport.CloseIdleConnections() })
			err := Deliver(context.Background(), origin.URL, "synthetic-token", []byte(`{}`))
			if err == nil || !strings.Contains(err.Error(), "307") {
				t.Errorf("redirect error = %v, want status 307", err)
			}
			if got := destinationCalls.Load(); got != 0 {
				t.Errorf("secondary origin received %d requests", got)
			}
		})
	}
}
