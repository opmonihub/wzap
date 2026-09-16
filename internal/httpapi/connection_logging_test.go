package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

// logRecord is one zerolog event decoded from the test buffer.
type logRecord struct {
	level zerolog.Level
	msg   string
	attrs map[string]any
}

// captureBoundaryLogs returns a JSON test logger writing into the returned
// buffer, so tests can assert which boundary lines a connection handler
// emits.
func captureBoundaryLogs(t *testing.T) (*bytes.Buffer, zerolog.Logger) {
	t.Helper()
	return logger.NewTestLogger()
}

// snapshotBoundaryLogs decodes every complete JSON event in buf, skipping
// the envelope fields every record carries.
func snapshotBoundaryLogs(buf *bytes.Buffer) []logRecord {
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

func findRecord(records []logRecord, msg string) (logRecord, bool) {
	for _, r := range records {
		if r.msg == msg {
			return r, true
		}
	}
	return logRecord{}, false
}

func TestConnectBoundaryLogs(t *testing.T) {
	id := uuid.New()
	expiresAt := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	svc := &fakeInstanceService{connectFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{Status: session.StatusPairing, QRCode: "qr-123", QRExpiresAt: &expiresAt}, nil
	}}
	logs, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+id.String()+"/connect", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	records := snapshotBoundaryLogs(logs)
	entry, ok := findRecord(records, "connect instance request")
	if !ok {
		t.Fatal("missing Debug record \"connect instance request\"")
	}
	if entry.level != zerolog.DebugLevel {
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
	if result.level != zerolog.DebugLevel {
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
	logger.AssertNoSecret(t, logs, "qr-123")
}

func TestConnectBoundaryLogsServiceError(t *testing.T) {
	svc := &fakeInstanceService{connectFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{}, errors.New("session dial failed")
	}}
	logs, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+uuid.NewString()+"/connect", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	warn, ok := findRecord(snapshotBoundaryLogs(logs), "connect instance failed")
	if !ok {
		t.Fatal("missing Warn record \"connect instance failed\"")
	}
	if warn.level != zerolog.WarnLevel {
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
	logs, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodGet, "/instances/"+id.String()+"/qr", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	records := snapshotBoundaryLogs(logs)
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
	logger.AssertNoSecret(t, logs, "qr-456")
}

func TestQRBoundaryLogsAlreadyConnected(t *testing.T) {
	svc := &fakeInstanceService{qrFn: func(context.Context, uuid.UUID) (instance.ConnectResult, error) {
		return instance.ConnectResult{}, instance.ErrAlreadyConnected
	}}
	logs, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodGet, "/instances/"+uuid.NewString()+"/qr", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}

	if _, ok := findRecord(snapshotBoundaryLogs(logs), "qr instance failed"); !ok {
		t.Error("missing Warn record \"qr instance failed\"")
	}
}

func TestStatusBoundaryLogs(t *testing.T) {
	wantID := uuid.New()
	svc := &fakeInstanceService{getFn: func(_ context.Context, id uuid.UUID) (*model.Instance, error) {
		return &model.Instance{ID: id, Name: "loja", Status: string(session.StatusConnected)}, nil
	}}
	logs, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodGet, "/instances/"+wantID.String()+"/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	records := snapshotBoundaryLogs(logs)
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
	logs, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+uuid.NewString()+"/disconnect", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	records := snapshotBoundaryLogs(logs)
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
	logs, log := captureBoundaryLogs(t)

	rec := serveJSON(t, loggedInstancesServer(t, svc, log), http.MethodPost, "/instances/"+uuid.NewString()+"/disconnect", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	warn, ok := findRecord(snapshotBoundaryLogs(logs), "disconnect instance failed")
	if !ok {
		t.Fatal("missing Warn record \"disconnect instance failed\"")
	}
	if warn.level != zerolog.WarnLevel {
		t.Errorf("level = %v, want Warn", warn.level)
	}
	if warn.attrs["op"] != "disconnect" {
		t.Errorf("op = %v, want disconnect", warn.attrs["op"])
	}
}
