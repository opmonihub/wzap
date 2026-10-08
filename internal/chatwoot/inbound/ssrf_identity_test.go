package inbound

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAttachmentMixedDNSCannotBorrowChatwootTrust(t *testing.T) {
	previous := lookupIP
	t.Cleanup(func() { lookupIP = previous })
	lookupIP = func(host string) ([]net.IP, error) {
		switch host {
		case "chatwoot.invalid":
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		case "attacker.invalid":
			return []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("93.184.216.34")}, nil
		default:
			return nil, fmt.Errorf("unexpected host %s", host)
		}
	}
	var calls atomic.Int32
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte("private attachment"))
	}))
	t.Cleanup(private.Close)
	_, port, err := net.SplitHostPort(private.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	const allow = "https://chatwoot.invalid"
	t.Run("url", func(t *testing.T) {
		d := NewHTTPDownloader(1024)
		d.transportFor(allow).Proxy = nil
		data, _, err := d.Download(context.Background(), "http://attacker.invalid:"+port+"/attachment", allow)
		if err == nil || len(data) != 0 || calls.Load() != 0 {
			t.Fatalf("mixed-DNS download = %q, %v; private calls = %d", data, err, calls.Load())
		}
	})
	t.Run("dial", func(t *testing.T) {
		conn, err := pinnedAttachmentDialContext(allow)(context.Background(), "tcp", "attacker.invalid:"+port)
		if conn != nil {
			_ = conn.Close()
		}
		if err == nil {
			t.Fatal("dial connected to an unrelated private address")
		}
	})
}

func TestAttachmentRedirectCannotBorrowChatwootTrust(t *testing.T) {
	previous := lookupIP
	t.Cleanup(func() { lookupIP = previous })
	lookupIP = func(host string) ([]net.IP, error) {
		if host == "chatwoot.invalid" || host == "attacker.invalid" {
			return []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("93.184.216.34")}, nil
		}
		return nil, fmt.Errorf("unexpected host %s", host)
	}
	var calls atomic.Int32
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte("private attachment"))
	}))
	t.Cleanup(private.Close)
	_, destinationPort, err := net.SplitHostPort(private.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://attacker.invalid:"+destinationPort+"/attachment", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)
	_, originPort, err := net.SplitHostPort(origin.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	allow := "http://chatwoot.invalid:" + originPort
	d := NewHTTPDownloader(1024)
	d.transportFor(allow).Proxy = nil
	data, _, err := d.Download(context.Background(), allow+"/attachment", allow)
	if err == nil || len(data) != 0 || calls.Load() != 0 {
		t.Fatalf("redirect download = %q, %v; private calls = %d", data, err, calls.Load())
	}
}

func TestAttachmentConfiguredPrivateHostRemainsAllowed(t *testing.T) {
	previous := lookupIP
	t.Cleanup(func() { lookupIP = previous })
	lookupIP = func(host string) ([]net.IP, error) {
		if host != "chatwoot.invalid" {
			return nil, fmt.Errorf("unexpected host %s", host)
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("configured private attachment"))
	}))
	t.Cleanup(origin.Close)
	_, port, err := net.SplitHostPort(origin.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	allow := "http://chatwoot.invalid:" + port
	d := NewHTTPDownloader(1024)
	d.transportFor(allow).Proxy = nil
	data, _, err := d.Download(context.Background(), allow+"/attachment", allow)
	if err != nil || string(data) != "configured private attachment" {
		t.Fatalf("configured private host = %q, %v", data, err)
	}
}

func TestAttachmentConfiguredIPIdentityAllowsEquivalentLiterals(t *testing.T) {
	for _, tc := range []struct{ target, configured string }{
		{"http://[0:0:0:0:0:0:0:1]/attachment", "http://[::1]"},
		{"http://[::ffff:127.0.0.1]/attachment", "http://127.0.0.1"},
	} {
		if err := validateAttachmentURL(tc.target, tc.configured); err != nil {
			t.Errorf("same configured IP rejected: %v", err)
		}
	}
}
