package inbound

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestDownloaderDialsThroughPinnedResolver pins that attachment fetches go
// through the SSRF resolver instead of system DNS: a stub-only hostname
// (unresolvable outside the stub) still downloads when the stub points it at
// the test server.
func TestDownloaderDialsThroughPinnedResolver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("pinned-bytes"))
	}))
	defer srv.Close()
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	old := lookupIP
	defer func() { lookupIP = old }()
	lookupIP = func(host string) ([]net.IP, error) {
		if host == "pinned.test" {
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}
		return nil, errors.New("no such host")
	}

	downloader := NewHTTPDownloader(1 << 20)
	data, mime, err := downloader.Download(context.Background(), "http://pinned.test:"+port+"/file", "http://pinned.test:"+port)
	if err != nil {
		t.Fatalf("Download through pinned resolver: %v", err)
	}
	if string(data) != "pinned-bytes" || mime != "image/jpeg" {
		t.Errorf("Download = (%q, %q), want the file bytes with its mime", data, mime)
	}
}

// TestDownloaderDialRevalidatesAfterRebinding pins the TOCTOU close: when DNS
// flips between validation (loopback) and connect (metadata), the dial is
// rejected on the rebound address instead of fetching it.
func TestDownloaderDialRevalidatesAfterRebinding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("must-not-fetch"))
	}))
	defer srv.Close()
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")

	var calls atomic.Int64
	old := lookupIP
	defer func() { lookupIP = old }()
	lookupIP = func(host string) ([]net.IP, error) {
		if host != "rebind.test" {
			return nil, errors.New("no such host")
		}
		if calls.Add(1) == 1 {
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}
		return []net.IP{net.ParseIP("169.254.169.254")}, nil
	}

	downloader := NewHTTPDownloader(1 << 20)
	_, _, err := downloader.Download(context.Background(), "http://rebind.test:"+port+"/file", "http://rebind.test:"+port)
	if err == nil {
		t.Fatal("Download error = nil, want rejection of the rebound address")
	}
	if !strings.Contains(err.Error(), "blocked address") {
		t.Errorf("Download error = %q, want it to name the blocked address", err.Error())
	}
}
