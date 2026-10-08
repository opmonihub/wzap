package httpapi_test

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// A static manager mount or a middleware writer that blocks Hijack breaks this
// contract, even if normal manager HTTP requests still pass.
func TestManagerDevMountForwardsWebSocket(t *testing.T) {
	type receivedRequest struct{ path, query, host string }
	received := make(chan receivedRequest, 1)
	completed := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- receivedRequest{r.URL.Path, r.URL.RawQuery, r.Host}
		completed <- serveManagerTestWebSocket(w, r)
	}))
	defer upstream.Close()
	entry := newManagerDevEntry(t, upstream.URL, zerolog.Nop())
	conn, err := net.DialTimeout("tcp", entry.Listener.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, entry.URL+"/manager/_nuxt/hmr?token=a%2Fb&v=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "public.example:8081"
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Protocol", "vite-hmr")
	req.Header.Set("X-Request-Id", "manager-websocket")
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		t.Fatalf("read WebSocket handshake: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("WebSocket handshake status = %d, want 101", resp.StatusCode)
	}
	for header, want := range map[string]string{
		"Connection": "Upgrade", "Upgrade": "websocket",
		"Sec-WebSocket-Accept":   "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=",
		"Sec-WebSocket-Protocol": "vite-hmr", "X-Request-Id": "manager-websocket",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("handshake %s = %q, want %q", header, got, want)
		}
	}
	select {
	case got := <-received:
		if got.path != "/manager/_nuxt/hmr" || got.query != "token=a%2Fb&v=1" || got.host != "public.example:8081" {
			t.Errorf("upstream WebSocket request = %+v; want original manager path, query and public Host", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upstream did not receive WebSocket request")
	}
	// Read an unsolicited upstream frame before sending, so a proxy that only
	// copies client traffic or stops copying after the 101 cannot pass.
	if got, err := readManagerTestTextFrame(reader, false); err != nil || got != "nuxt-ready" {
		t.Fatalf("upstream greeting = %q, %v; want nuxt-ready", got, err)
	}
	if err := writeManagerTestTextFrame(conn, "hmr-update", true); err != nil {
		t.Fatalf("send client frame: %v", err)
	}
	if got, err := readManagerTestTextFrame(reader, false); err != nil || got != "ack:hmr-update" {
		t.Fatalf("upstream reply = %q, %v; want ack:hmr-update", got, err)
	}
	select {
	case err := <-completed:
		if err != nil {
			t.Fatalf("upstream WebSocket exchange: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upstream WebSocket exchange did not finish")
	}
}

func serveManagerTestWebSocket(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet || !strings.EqualFold(r.Header.Get("Connection"), "Upgrade") ||
		!strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		r.Header.Get("Sec-WebSocket-Version") != "13" || r.Header.Get("Sec-WebSocket-Key") != "dGhlIHNhbXBsZSBub25jZQ==" ||
		r.Header.Get("Sec-WebSocket-Protocol") != "vite-hmr" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("invalid WebSocket upgrade headers: %v", r.Header)
	}
	conn, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	digest := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: %s\r\nSec-WebSocket-Protocol: vite-hmr\r\n\r\n", base64.StdEncoding.EncodeToString(digest[:])); err != nil {
		return err
	}
	if err := writeManagerTestTextFrame(rw, "nuxt-ready", false); err != nil {
		return err
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	payload, err := readManagerTestTextFrame(rw, true)
	if err != nil {
		return err
	}
	if payload != "hmr-update" {
		return fmt.Errorf("client frame = %q, want hmr-update", payload)
	}
	if err := writeManagerTestTextFrame(rw, "ack:"+payload, false); err != nil {
		return err
	}
	return rw.Flush()
}

// The fixture only needs short, unfragmented RFC 6455 text frames. Clients mask
// their frames and servers do not; extended lengths and other opcodes are errors.
func writeManagerTestTextFrame(w io.Writer, payload string, masked bool) error {
	if len(payload) > 125 {
		return fmt.Errorf("test frame payload exceeds 125 bytes")
	}
	frame := []byte{0x81, byte(len(payload))}
	body := []byte(payload)
	if masked {
		frame[1] |= 0x80
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return err
		}
		frame = append(frame, mask[:]...)
		for i := range body {
			body[i] ^= mask[i%len(mask)]
		}
	}
	_, err := w.Write(append(frame, body...))
	return err
}

func readManagerTestTextFrame(r io.Reader, masked bool) (string, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return "", err
	}
	if header[0] != 0x81 || (header[1]&0x80 != 0) != masked || header[1]&0x7f > 125 {
		return "", fmt.Errorf("invalid short text frame header: %x", header)
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return "", err
		}
	}
	payload := make([]byte, int(header[1]&0x7f))
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%len(mask)]
		}
	}
	return string(payload), nil
}
