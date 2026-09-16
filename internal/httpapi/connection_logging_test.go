package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/instance"
	"wzap/internal/logger"
	"wzap/internal/model"
	"wzap/internal/session"
)

// capturedRecord is one slog record observed by memoryHandler.
type capturedRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

// memoryHandler is an in-memory slog.Handler that records every log record so
// tests can assert which boundary lines a connection handler emits.
type memoryHandler struct {
	pre  []slog.Attr
	core *memoryCore
}

type memoryCore struct {
	mu      sync.Mutex
	records []capturedRecord
}

func newMemoryHandler() *memoryHandler { return &memoryHandler{core: &memoryCore{}} }

func (h *memoryHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *memoryHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]any, len(h.pre)+r.NumAttrs())
	for _, a := range h.pre {
		attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	h.core.mu.Lock()
	h.core.records = append(h.core.records, capturedRecord{level: r.Level, msg: r.Message, attrs: attrs})
	h.core.mu.Unlock()
	return nil
}

func (h *memoryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &memoryHandler{
		pre:  append(append([]slog.Attr{}, h.pre...), attrs...),
		core: h.core,
	}
}

func (h *memoryHandler) WithGroup(string) slog.Handler { return h }

// snapshot returns a copy of the records captured so far.
func (h *memoryHandler) snapshot() []capturedRecord {
	h.core.mu.Lock()
	defer h.core.mu.Unlock()
	return append([]capturedRecord{}, h.core.records...)
}

// captureBoundaryLogs returns an in-memory handler fed by a zerolog test
// logger: the handlers write JSON into the handler writer, which decodes
// each event back into a record so the boundary assertions keep observing
// without rewriting them here (full helper unification is Task 4.1).
func captureBoundaryLogs(t *testing.T) (*memoryHandler, zerolog.Logger) {
	t.Helper()
	h := newMemoryHandler()
	_, base := logger.NewTestLogger()
	return h, base.Output(boundaryHandlerWriter{h: h})
}

// boundaryHandlerWriter decodes zerolog JSON lines into the memory handler.
type boundaryHandlerWriter struct {
	h *memoryHandler
}

func (w boundaryHandlerWriter) Write(p []byte) (int, error) {
	dec := json.NewDecoder(bytes.NewReader(p))
	for {
		var fields map[string]any
		if err := dec.Decode(&fields); err != nil {
			return len(p), nil
		}
		w.h.addJSONRecord(fields)
	}
}

// addJSONRecord stores one decoded zerolog event as a captured record.
func (h *memoryHandler) addJSONRecord(fields map[string]any) {
	var level slog.Level
	switch fields["level"] {
	case "trace", "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error", "fatal", "panic":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	msg, _ := fields["message"].(string)
	attrs := make(map[string]any, len(fields))
	for k, v := range fields {
		if k == "level" || k == "message" || k == "time" {
			continue
		}
		attrs[k] = v
	}
	h.core.mu.Lock()
	h.core.records = append(h.core.records, capturedRecord{level: level, msg: msg, attrs: attrs})
	h.core.mu.Unlock()
}

// loggedInstancesServer builds the server under test with svc and the given
// logger so boundary tests can observe the injected handler logs.
func loggedInstancesServer(t *testing.T, svc InstanceService, log zerolog.Logger) *http.Server {
	t.Helper()
	if svc == nil {
		svc = &fakeInstanceService{}
	}
	return New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken}, log,
		Deps{
			ReadyChecker: checkFunc(func(context.Context) error { return nil }),
			Instances:    svc,
		})
}

func findRecord(records []capturedRecord, msg string) (capturedRecord, bool) {
	for _, r := range records {
		if r.msg == msg {
			return r, true
		}
	}
	return capturedRecord{}, false
}

// assertNoQRBytes fails when any captured record carries the QR payload: only
// presence and expiry may be logged, never the code bytes.
func assertNoQRBytes(t *testing.T, h *memoryHandler, qr string) {
	t.Helper()
	if qr == "" {
		return
	}
	for _, r := range h.snapshot() {
		if strings.Contains(r.msg, qr) {
			t.Fatalf("log message %q contains secret value", r.msg)
		}
		for k, v := range r.attrs {
			if strings.Contains(fmt.Sprintf("%v", v), qr) {
				t.Fatalf("log record %q attr %q contains secret value", r.msg, k)
			}
		}
	}
}

func TestConnectBoundaryLogs(t *testing.T) {
	id := uuid.New()
	expiresAt := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	svc := &fakeInstanceService{connectFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{Status: session.StatusPairing, QRCode: "qr-123", QRExpiresAt: &expiresAt}, nil
	}}
	h, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+id.String()+"/connect", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	records := h.snapshot()
	entry, ok := findRecord(records, "connect instance request")
	if !ok {
		t.Fatal("missing Debug record \"connect instance request\"")
	}
	if entry.level != slog.LevelDebug {
		t.Errorf("entry level = %v, want Debug", entry.level)
	}
	if entry.attrs["instance_id"] != id.String() {
		t.Errorf("entry instance_id = %v, want %s", entry.attrs["instance_id"], id)
	}
	if entry.attrs["op"] != "connect" {
		t.Errorf("entry op = %v, want connect", entry.attrs["op"])
	}

	result, ok := findRecord(records, "connect instance result")
	if !ok {
		t.Fatal("missing Debug record \"connect instance result\"")
	}
	if result.level != slog.LevelDebug {
		t.Errorf("result level = %v, want Debug", result.level)
	}
	if result.attrs["status"] != string(session.StatusPairing) {
		t.Errorf("result status = %v, want %q", result.attrs["status"], session.StatusPairing)
	}
	if result.attrs["qr_present"] != true {
		t.Errorf("result qr_present = %v, want true", result.attrs["qr_present"])
	}
	if _, ok := result.attrs["expires_at"]; !ok {
		t.Error("result misses expires_at, want the QR validity")
	}
	assertNoQRBytes(t, h, "qr-123")
}

func TestConnectBoundaryLogsServiceError(t *testing.T) {
	svc := &fakeInstanceService{connectFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{}, errors.New("session dial failed")
	}}
	h, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+uuid.NewString()+"/connect", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	warn, ok := findRecord(h.snapshot(), "connect instance failed")
	if !ok {
		t.Fatal("missing Warn record \"connect instance failed\"")
	}
	if warn.level != slog.LevelWarn {
		t.Errorf("level = %v, want Warn", warn.level)
	}
	if warn.attrs["op"] != "connect" {
		t.Errorf("op = %v, want connect", warn.attrs["op"])
	}
	if _, ok := warn.attrs["error"]; !ok {
		t.Error("record misses error, want the service failure")
	}
}

func TestQRBoundaryLogs(t *testing.T) {
	id := uuid.New()
	expiresAt := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	svc := &fakeInstanceService{qrFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{Status: session.StatusPairing, QRCode: "qr-456", QRExpiresAt: &expiresAt}, nil
	}}
	h, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodGet, "/instances/"+id.String()+"/qr", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	records := h.snapshot()
	if _, ok := findRecord(records, "qr instance request"); !ok {
		t.Error("missing Debug record \"qr instance request\"")
	}
	result, ok := findRecord(records, "qr instance result")
	if !ok {
		t.Fatal("missing Debug record \"qr instance result\"")
	}
	if result.attrs["qr_present"] != true {
		t.Errorf("result qr_present = %v, want true", result.attrs["qr_present"])
	}
	if _, ok := result.attrs["expires_at"]; !ok {
		t.Error("result misses expires_at, want the QR validity")
	}
	assertNoQRBytes(t, h, "qr-456")
}

func TestQRBoundaryLogsAlreadyConnected(t *testing.T) {
	svc := &fakeInstanceService{qrFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{}, instance.ErrAlreadyConnected
	}}
	h, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodGet, "/instances/"+uuid.NewString()+"/qr", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}

	if _, ok := findRecord(h.snapshot(), "qr instance failed"); !ok {
		t.Error("missing Warn record \"qr instance failed\"")
	}
}

func TestStatusBoundaryLogs(t *testing.T) {
	wantID := uuid.New()
	svc := &fakeInstanceService{getFn: func(_ context.Context, id uuid.UUID) (*model.Instance, error) {
		return &model.Instance{ID: id, Name: "loja", Status: string(session.StatusConnected)}, nil
	}}
	h, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodGet, "/instances/"+wantID.String()+"/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	records := h.snapshot()
	if _, ok := findRecord(records, "instance status request"); !ok {
		t.Error("missing Debug record \"instance status request\"")
	}
	result, ok := findRecord(records, "instance status result")
	if !ok {
		t.Fatal("missing Debug record \"instance status result\"")
	}
	if result.attrs["status"] != string(session.StatusConnected) {
		t.Errorf("result status = %v, want %q", result.attrs["status"], session.StatusConnected)
	}
}

func TestDisconnectBoundaryLogs(t *testing.T) {
	svc := &fakeInstanceService{}
	h, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+uuid.NewString()+"/disconnect", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	records := h.snapshot()
	if _, ok := findRecord(records, "disconnect instance request"); !ok {
		t.Error("missing Debug record \"disconnect instance request\"")
	}
	if _, ok := findRecord(records, "disconnect instance result"); !ok {
		t.Error("missing Debug record \"disconnect instance result\"")
	}
}

func TestDisconnectBoundaryLogsServiceError(t *testing.T) {
	svc := &fakeInstanceService{disconnectFn: func(context.Context, uuid.UUID) error {
		return errors.New("session still connected")
	}}
	h, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+uuid.NewString()+"/disconnect", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	warn, ok := findRecord(h.snapshot(), "disconnect instance failed")
	if !ok {
		t.Fatal("missing Warn record \"disconnect instance failed\"")
	}
	if warn.level != slog.LevelWarn {
		t.Errorf("level = %v, want Warn", warn.level)
	}
	if warn.attrs["op"] != "disconnect" {
		t.Errorf("op = %v, want disconnect", warn.attrs["op"])
	}
}
