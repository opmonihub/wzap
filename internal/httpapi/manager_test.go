package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/manager"
)

// The manager console is a public surface: browsers load it without a
// credential and log in through it.
func TestManagerMountIsPublic(t *testing.T) {
	for _, path := range []string{"/manager/", "/manager"} {
		rec := serve(t, newTestServer(t), http.MethodGet, path, "")
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("GET %s without credential = 401, want the public console (or its redirect)", path)
		}
	}
}
func TestManagerServesIndexWhenBuilt(t *testing.T) {
	t.Setenv("WZAP_MANAGER_DIR", "")
	if !manager.Built() {
		t.Skip("manager static build is not embedded; run pnpm --dir manager build first")
	}
	srv := newTestServer(t)

	rec := serve(t, srv, http.MethodGet, "/manager/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /manager/ = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	rec = serve(t, srv, http.MethodGet, "/manager/instances", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /manager/instances = %d, want 200 (SPA fallback)", rec.Code)
	}
}

func newManagerDevEntry(t *testing.T, target string, log zerolog.Logger) *httptest.Server {
	t.Helper()
	srv := httpapi.New(config.Config{APIKey: testToken, ManagerDevURL: target}, log, httpapi.Deps{
		ReadyChecker: checkFunc(func(context.Context) error { return nil }),
		Instances:    &fakeInstanceService{},
	})
	entry := httptest.NewServer(srv.Handler)
	t.Cleanup(entry.Close)
	return entry
}

func managerDevRequest(t *testing.T, entry *httptest.Server, path string) (int, http.Header, []byte) {
	t.Helper()
	client := entry.Client()
	client.Timeout = 3 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequest(http.MethodGet, entry.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "public.example:8081"
	req.Header.Set("X-Request-Id", "manager-request")
	req.Header.Set("Cookie", "wzap_session=private-cookie")
	req.Header.Set("apikey", "private-api-key")
	req.Header.Set("Forwarded", "host=spoofed.example;proto=https")
	req.Header.Set("X-Forwarded-For", "192.0.2.99")
	req.Header.Set("X-Forwarded-Host", "spoofed.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, body
}

func TestManagerDevMountForwardsHTTP(t *testing.T) {
	type receivedRequest struct {
		Path, RawPath, Query, URI, Host string
		Headers                         http.Header
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(receivedRequest{
			Path: r.URL.Path, RawPath: r.URL.RawPath, Query: r.URL.RawQuery,
			URI: r.RequestURI, Host: r.Host, Headers: r.Header,
		})
	}))
	defer upstream.Close()
	entry := newManagerDevEntry(t, upstream.URL+"/", zerolog.Nop())
	for _, tt := range []struct{ path, decodedPath, rawPath, query string }{
		{"/manager/", "/manager/", "", ""},
		{"/manager/instances?next=details", "/manager/instances", "", "next=details"},
		{"/manager/_nuxt/app.js?v=1%2F2&raw=a;b", "/manager/_nuxt/app.js", "", "v=1%2F2&raw=a;b"},
		{"/manager/files/a%2Fb?next=%2Finstances", "/manager/files/a/b", "/manager/files/a%2Fb", "next=%2Finstances"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			status, headers, body := managerDevRequest(t, entry, tt.path)
			if status != http.StatusOK {
				t.Fatalf("status = %d, body = %q; want upstream 200", status, body)
			}
			if got := headers.Get("X-Request-Id"); got != "manager-request" {
				t.Errorf("X-Request-Id = %q, want manager-request", got)
			}
			var got receivedRequest
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode upstream response: %v; body = %q", err, body)
			}
			if got.Path != tt.decodedPath || got.RawPath != tt.rawPath || got.Query != tt.query || got.URI != tt.path {
				t.Errorf("upstream URL = path %q raw path %q query %q URI %q; want %q %q %q %q", got.Path, got.RawPath, got.Query, got.URI, tt.decodedPath, tt.rawPath, tt.query, tt.path)
			}
			if got.Host != "public.example:8081" {
				t.Errorf("upstream Host = %q, want public.example:8081", got.Host)
			}
			for header, want := range map[string]string{
				"X-Forwarded-Host": "public.example:8081", "X-Forwarded-Proto": "http",
				"X-Forwarded-For": "127.0.0.1", "Forwarded": "", "X-Request-Id": "manager-request",
				"Cookie": "wzap_session=private-cookie", "apikey": "private-api-key",
			} {
				if value := got.Headers.Get(header); value != want {
					t.Errorf("upstream %s = %q, want %q", header, value, want)
				}
			}
		})
	}
}

func TestManagerDevRedirectPreservesQuery(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer upstream.Close()
	entry := newManagerDevEntry(t, upstream.URL, zerolog.Nop())
	status, headers, _ := managerDevRequest(t, entry, "/manager?next=instances")
	if status != http.StatusMovedPermanently || headers.Get("Location") != "/manager/?next=instances" {
		t.Errorf("status = %d, Location = %q; want 301 /manager/?next=instances", status, headers.Get("Location"))
	}
	if requests.Load() != 0 {
		t.Error("bare manager redirect reached the upstream")
	}
}

func TestManagerDevFailureHasNoStaticFallbackAndRecovers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("stale manager bundle"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WZAP_MANAGER_DIR", dir)
	if !manager.Built() {
		t.Fatal("static fallback fixture is not usable")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("live manager")) }))
	target, addr := upstream.URL, upstream.Listener.Addr().String()
	upstream.Close()
	var logs bytes.Buffer
	entry := newManagerDevEntry(t, target, zerolog.New(&logs))
	for _, path := range []string{"/manager/?token=private-query", "/manager/instances", "/manager/_nuxt/app.js"} {
		status, headers, body := managerDevRequest(t, entry, path)
		if status != http.StatusServiceUnavailable {
			t.Errorf("GET %s status = %d, body = %q; want 503", path, status, body)
		}
		if !strings.HasPrefix(headers.Get("Content-Type"), "text/plain") || len(body) == 0 {
			t.Errorf("GET %s must return a nonempty plain-text error", path)
		}
		for _, secret := range []string{target, addr, "private-query", "private-cookie", "private-api-key", "stale manager bundle", "dial tcp"} {
			if strings.Contains(string(body), secret) || strings.Contains(logs.String(), secret) {
				t.Errorf("response or logs disclose %q", secret)
			}
		}
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	recovered := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("recovered manager")) }))
	if err := recovered.Listener.Close(); err != nil {
		t.Fatal(err)
	}
	recovered.Listener = listener
	recovered.Start()
	defer recovered.Close()
	status, _, body := managerDevRequest(t, entry, "/manager/")
	if status != http.StatusOK || string(body) != "recovered manager" {
		t.Fatalf("recovered upstream = %d %q, want 200 recovered manager", status, body)
	}
}

func TestManagerDevKeepsBackendRoutes(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer upstream.Close()
	entry := newManagerDevEntry(t, upstream.URL, zerolog.Nop())
	for _, tt := range []struct {
		path string
		want int
	}{
		{"/auth", 404}, {"/auth/me", 401}, {"/instances", 401}, {"/users", 401}, {"/media", 401},
		{"/healthz", 200}, {"/readyz", 200}, {"/swagger/index.html", 200},
	} {
		t.Run(tt.path, func(t *testing.T) {
			status, headers, _ := managerDevRequest(t, entry, tt.path)
			if status != tt.want {
				t.Errorf("status = %d, want %d", status, tt.want)
			}
			if headers.Get("X-Request-Id") != "manager-request" {
				t.Error("request id middleware was lost")
			}
		})
	}
	if requests.Load() != 0 {
		t.Errorf("backend routes reached manager upstream %d times", requests.Load())
	}
}
