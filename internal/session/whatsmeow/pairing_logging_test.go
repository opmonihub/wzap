package whatsmeow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

// logRecord is one zerolog event decoded from the test buffer.
type logRecord struct {
	level zerolog.Level
	msg   string
	attrs map[string]any
}

// syncLogBuffer is a mutex-guarded log sink for tests where the session
// goroutine writes while the test goroutine polls. Bytes returns a copy so
// concurrent writes never race the polling reads.
type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *syncLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncLogBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func newSyncTestLogger() (*syncLogBuffer, zerolog.Logger) {
	buf := &syncLogBuffer{}
	return buf, zerolog.New(buf).With().Timestamp().Logger().Level(zerolog.DebugLevel)
}

func assertNoSecret(t *testing.T, buf *syncLogBuffer, secrets ...string) {
	t.Helper()
	out := buf.String()
	for _, s := range secrets {
		if s != "" && strings.Contains(out, s) {
			t.Errorf("log output leaks secret %q: %q", s, out)
		}
	}
}

// snapshotRecords decodes every complete JSON event in buf, skipping the
// envelope fields every record carries.
func snapshotRecords(buf *syncLogBuffer) []logRecord {
	var records []logRecord
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	for {
		var fields map[string]any
		if err := dec.Decode(&fields); err != nil {
			return records
		}
		level := zerolog.InfoLevel
		if name, ok := fields["level"].(string); ok {
			if parsed, err := zerolog.ParseLevel(name); err == nil {
				level = parsed
			}
		}
		msg, _ := fields["message"].(string)
		attrs := make(map[string]any, len(fields))
		for k, v := range fields {
			if k == "level" || k == "message" || k == "time" {
				continue
			}
			attrs[k] = v
		}
		records = append(records, logRecord{level: level, msg: msg, attrs: attrs})
	}
}

func waitForRecord(t *testing.T, buf *syncLogBuffer, msg string) logRecord {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, r := range snapshotRecords(buf) {
			if r.msg == msg {
				return r
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for log record %q", msg)
	return logRecord{}
}

func newLoggedSession(t *testing.T, device *store.Device, log zerolog.Logger, sink session.EventSink) *instanceSession {
	t.Helper()
	sess, err := newSession(uuid.New(), device, log, sink, testMediaLimit)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	return sess
}

// TestMonitorQRLogsSuccess verifies the success terminal emits its boundary
// log while the session still reaches connected with the paired JID.
func TestMonitorQRLogsSuccess(t *testing.T) {
	logs, log := newSyncTestLogger()
	jid := types.NewJID("5511999999999", types.DefaultUserServer)
	sink := &recordingSink{}
	sess := newLoggedSession(t, &store.Device{ID: &jid}, log, sink)

	pairing := make(chan whatsmeow.QRChannelItem, 4)
	go sess.monitorQR(pairing)
	pairing <- qrCodeItem("QR-SUCCESS-LOG")
	pairing <- whatsmeow.QRChannelSuccess

	rec := waitForRecord(t, logs, "pairing succeeded")
	if rec.level != zerolog.InfoLevel {
		t.Errorf("pairing succeeded level = %v, want info", rec.level)
	}
	if rec.attrs["instance_id"] != sess.instanceID.String() {
		t.Errorf("pairing succeeded instance_id = %v, want %v", rec.attrs["instance_id"], sess.instanceID)
	}
	assertNoSecret(t, logs, "QR-SUCCESS-LOG")

	waitForPairing(t, "pairing success event", func() bool { return sink.count() > 0 })
	if sess.Status() != session.StatusConnected {
		t.Fatalf("session status = %q, want connected", sess.Status())
	}
}

// TestMonitorQRLogsTimeout verifies the QR expiry terminal emits its boundary
// log with the stable reason while the session still disconnects.
func TestMonitorQRLogsTimeout(t *testing.T) {
	logs, log := newSyncTestLogger()
	sink := &recordingSink{}
	sess := newLoggedSession(t, &store.Device{}, log, sink)

	pairing := make(chan whatsmeow.QRChannelItem, 4)
	go sess.monitorQR(pairing)
	pairing <- qrCodeItem("QR-TIMEOUT-LOG")
	pairing <- whatsmeow.QRChannelTimeout

	rec := waitForRecord(t, logs, "pairing timed out")
	if rec.level != zerolog.WarnLevel {
		t.Errorf("pairing timed out level = %v, want warn", rec.level)
	}
	if rec.attrs["reason"] != "qr code expired" {
		t.Errorf("pairing timed out reason = %v, want %q", rec.attrs["reason"], "qr code expired")
	}
	assertNoSecret(t, logs, "QR-TIMEOUT-LOG")

	waitForPairing(t, "expiry disconnect", func() bool {
		return sess.Status() == session.StatusDisconnected
	})
}

// TestMonitorQRLogsError verifies the error terminal emits its boundary log
// and the error-status transition additionally warns, without changing the
// resulting error state.
func TestMonitorQRLogsError(t *testing.T) {
	logs, log := newSyncTestLogger()
	sink := &recordingSink{}
	sess := newLoggedSession(t, &store.Device{}, log, sink)

	pairing := make(chan whatsmeow.QRChannelItem, 4)
	go sess.monitorQR(pairing)
	pairing <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventError, Error: fmt.Errorf("boom")}

	rec := waitForRecord(t, logs, "pairing failed")
	if rec.level != zerolog.WarnLevel {
		t.Errorf("pairing failed level = %v, want warn", rec.level)
	}
	if rec.attrs["instance_id"] != sess.instanceID.String() {
		t.Errorf("pairing failed instance_id = %v, want %v", rec.attrs["instance_id"], sess.instanceID)
	}
	warn := waitForRecord(t, logs, "session entered error status")
	if warn.level != zerolog.WarnLevel {
		t.Errorf("session entered error status level = %v, want warn", warn.level)
	}

	waitForPairing(t, "error status", func() bool {
		return sess.Status() == session.StatusError
	})
}

// TestMonitorQRLogsChannelClosed verifies an abruptly closed QR channel emits
// its boundary log while the session still disconnects.
func TestMonitorQRLogsChannelClosed(t *testing.T) {
	logs, log := newSyncTestLogger()
	sess := newLoggedSession(t, &store.Device{}, log, nil)

	pairing := make(chan whatsmeow.QRChannelItem, 4)
	go sess.monitorQR(pairing)
	close(pairing)

	rec := waitForRecord(t, logs, "pairing qr channel closed")
	if rec.level != zerolog.WarnLevel {
		t.Errorf("pairing qr channel closed level = %v, want warn", rec.level)
	}
	if rec.attrs["reason"] != "qr channel closed" {
		t.Errorf("pairing qr channel closed reason = %v, want %q", rec.attrs["reason"], "qr channel closed")
	}
	waitForPairing(t, "closed-channel disconnect", func() bool {
		return sess.Status() == session.StatusDisconnected
	})
}

// TestQRCodeRotationLogsExpiryWithoutBytes verifies a code rotation emits a
// debug line carrying only the expiry, never the code bytes.
func TestQRCodeRotationLogsExpiryWithoutBytes(t *testing.T) {
	logs, log := newSyncTestLogger()
	sess := newLoggedSession(t, &store.Device{}, log, nil)

	pairing := make(chan whatsmeow.QRChannelItem, 4)
	go sess.monitorQR(pairing)
	pairing <- qrCodeItem("ROTATION-SECRET-CODE")

	rec := waitForRecord(t, logs, "qr code rotated")
	if rec.level != zerolog.DebugLevel {
		t.Errorf("qr code rotated level = %v, want debug", rec.level)
	}
	if _, ok := rec.attrs["expires_at"]; !ok {
		t.Errorf("qr code rotated record is missing expires_at: %v", rec.attrs)
	}
	assertNoSecret(t, logs, "ROTATION-SECRET-CODE")
	close(pairing)
}

// TestSetStatusLogsTransition verifies every status transition emits a debug
// line with from/to, jid presence and reason, that repeats stay silent, and
// that the error status additionally warns.
func TestSetStatusLogsTransition(t *testing.T) {
	logs, log := newSyncTestLogger()
	sess := newLoggedSession(t, &store.Device{}, log, nil)

	sess.setStatus(session.StatusPairing, "", "")
	rec := waitForRecord(t, logs, "session status changed")
	if rec.level != zerolog.DebugLevel {
		t.Errorf("session status changed level = %v, want debug", rec.level)
	}
	if rec.attrs["from"] != string(session.StatusDisconnected) || rec.attrs["to"] != string(session.StatusPairing) {
		t.Errorf("session status changed from/to = %v/%v, want disconnected/pairing",
			rec.attrs["from"], rec.attrs["to"])
	}
	if rec.attrs["jid_present"] != false {
		t.Errorf("session status changed jid_present = %v, want false", rec.attrs["jid_present"])
	}
	before := len(snapshotRecords(logs))
	sess.setStatus(session.StatusPairing, "", "")
	time.Sleep(50 * time.Millisecond)
	if got := len(snapshotRecords(logs)); got != before {
		t.Fatalf("repeated setStatus emitted %d extra records, want none", got-before)
	}

	sess.setStatus(session.StatusError, "", "boom")
	warn := waitForRecord(t, logs, "session entered error status")
	if warn.attrs["reason"] != "boom" {
		t.Errorf("session entered error status reason = %v, want %q", warn.attrs["reason"], "boom")
	}
	if sess.Status() != session.StatusError {
		t.Fatalf("session status = %q, want error", sess.Status())
	}
}
