package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFinalHEAD(t *testing.T) {
	operator := seedAuthUser(t, "operator@example.com", "contract-password", "admin")
	server := httptest.NewServer(responseContractServer(t, operator, true).Handler)
	defer server.Close()
	for _, tc := range []struct {
		path          string
		authenticated bool
		status        int
	}{{"/healthz", false, 200}, {"/instances", false, 401}, {"/instances", true, 200}} {
		t.Run(tc.path+http.StatusText(tc.status), func(t *testing.T) {
			var getType string
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				req, err := http.NewRequest(method, server.URL+tc.path, nil)
				if err != nil {
					t.Fatal(err)
				}
				if tc.authenticated {
					req.Header.Set("apikey", testToken)
				}
				response, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				if response.StatusCode != tc.status {
					t.Fatalf("%s status=%d", method, response.StatusCode)
				}
				if method == http.MethodGet {
					getType = response.Header.Get("Content-Type")
				} else if len(body) != 0 || response.Header.Get("Content-Type") != getType {
					t.Fatalf("HEAD body=%s headers=%v", body, response.Header)
				}
				if response.Header.Get("X-Request-Id") == "" {
					t.Error("missing request ID")
				}
			}
		})
	}
}
