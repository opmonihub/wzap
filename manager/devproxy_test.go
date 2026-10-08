package manager

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDevHandlerForwardsBodyAndTLSHeaders(t *testing.T) {
	type receivedRequest struct {
		Method, Body, URI, Host string
		Headers                 http.Header
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Dev-Response", "upstream")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(receivedRequest{r.Method, string(body), r.RequestURI, r.Host, r.Header})
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	entry := httptest.NewTLSServer(DevHandler(target))
	defer entry.Close()
	req, err := http.NewRequest(http.MethodPost, entry.URL+"/manager/__nuxt?token=a%2Fb", strings.NewReader("dev request body"))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "public.example:8443"
	req.Header.Set("X-Forwarded-For", "192.0.2.99")
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("Connection", "X-Private-Hop")
	req.Header.Set("X-Private-Hop", "remove-me")
	client := entry.Client()
	client.Timeout = 3 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted || resp.Header.Get("X-Dev-Response") != "upstream" {
		t.Fatalf("upstream response = %d, headers = %v; want 202 and X-Dev-Response", resp.StatusCode, resp.Header)
	}
	var got receivedRequest
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Method != "POST" || got.Body != "dev request body" || got.URI != "/manager/__nuxt?token=a%2Fb" || got.Host != "public.example:8443" {
		t.Errorf("upstream request = %+v", got)
	}
	for header, want := range map[string]string{
		"X-Forwarded-For": "127.0.0.1", "X-Forwarded-Host": "public.example:8443",
		"X-Forwarded-Proto": "https", "X-Private-Hop": "",
	} {
		if value := got.Headers.Get(header); value != want {
			t.Errorf("upstream %s = %q, want %q", header, value, want)
		}
	}
}

func TestDevHandlerRedirectPreservesEmptyQuery(t *testing.T) {
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:1"}
	entry := httptest.NewServer(DevHandler(target))
	defer entry.Close()
	client := entry.Client()
	client.Timeout = 3 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(entry.URL + "/manager?")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/manager/?" {
		t.Errorf("status = %d, Location = %q; want 301 /manager/?", resp.StatusCode, resp.Header.Get("Location"))
	}
}
