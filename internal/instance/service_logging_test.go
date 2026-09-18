package instance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/logger"
	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
)

// logRecord is one zerolog event decoded from the test buffer.
type logRecord struct {
	level zerolog.Level
	msg   string
	attrs map[string]any
}

// captureServiceLogs returns a JSON test logger writing into the returned
// buffer, so tests can assert which branch lines Connect/QR emit.
func captureServiceLogs(t *testing.T) (*bytes.Buffer, zerolog.Logger) {
	t.Helper()
	return logger.NewTestLogger()
}

// snapshotServiceLogs decodes every complete JSON event in buf, skipping the
// envelope fields every record carries.
func snapshotServiceLogs(buf *bytes.Buffer) []logRecord {
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

func findServiceRecord(records []logRecord, msg string, branch string) (logRecord, bool) {
	for _, r := range records {
		if r.msg == msg && r.attrs["branch"] == branch {
			return r, true
		}
	}
	return logRecord{}, false
}

// storedCredsSession is a fake session whose Connect reports stored
// credentials: no QR code and no error, like a device pairing without a scan.
type storedCredsSession struct {
	*sessiontest.FakeSession
}

// Connect answers stored credentials without a QR code.
func (s *storedCredsSession) Connect(context.Context) (string, time.Time, error) {
	return "", time.Time{}, nil
}

// storedCredsManager returns a registered session whose device holds stored
// credentials, as the manager does for a paired instance.
type storedCredsManager struct {
	*sessiontest.Fake
	sess *storedCredsSession
}

// Create returns the stored-credentials session.
func (m *storedCredsManager) Create(*model.Instance) (session.Session, error) {
	return m.sess, nil
}

func TestConnectLogsAlreadyConnectedBranch(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: string(session.StatusConnected)})
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(id, nil)
	sess.SetStatus(session.StatusConnected)
	sess.SetConnected(true)
	sessions.Put(id, sess)
	logs, log := captureServiceLogs(t)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, log)

	result, err := svc.Connect(context.Background(), id)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if result.Status != session.StatusConnected {
		t.Fatalf("Status = %q, want %q", result.Status, session.StatusConnected)
	}

	rec, ok := findServiceRecord(snapshotServiceLogs(logs), "connect instance branch", "already-connected")
	if !ok {
		t.Fatal("missing Debug record \"connect instance branch\" with branch already-connected")
	}
	if rec.level != zerolog.DebugLevel {
		t.Errorf("level = %v, want Debug", rec.level)
	}
	if rec.attrs["op"] != "connect" {
		t.Errorf("op = %v, want connect", rec.attrs["op"])
	}
	if rec.attrs["instance_id"] != id.String() {
		t.Errorf("instance_id = %v, want %s", rec.attrs["instance_id"], id)
	}
}

func TestConnectLogsAlreadyPairingBranch(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: string(session.StatusPairing)})
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(id, nil)
	sessions.Put(id, sess)
	if _, _, err := sess.Connect(context.Background()); err != nil {
		t.Fatalf("setup session Connect: %v", err)
	}
	logs, log := captureServiceLogs(t)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, log)

	result, err := svc.Connect(context.Background(), id)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	rec, ok := findServiceRecord(snapshotServiceLogs(logs), "connect instance branch", "already-pairing")
	if !ok {
		t.Fatal("missing Debug record \"connect instance branch\" with branch already-pairing")
	}
	if rec.attrs["qr_present"] != true {
		t.Errorf("qr_present = %v, want true", rec.attrs["qr_present"])
	}
	if _, ok := rec.attrs["expires_at"]; !ok {
		t.Error("record misses expires_at, want the QR validity")
	}
	logger.AssertNoSecret(t, logs, result.QRCode)
}

func TestConnectLogsNewPairingBranch(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: "disconnected"})
	logs, log := captureServiceLogs(t)
	svc := NewService(repo, sessiontest.New(nil), &fakeMedia{}, nil, nil, log)

	result, err := svc.Connect(context.Background(), id)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	rec, ok := findServiceRecord(snapshotServiceLogs(logs), "connect instance branch", "new-pairing")
	if !ok {
		t.Fatal("missing Debug record \"connect instance branch\" with branch new-pairing")
	}
	if rec.attrs["status"] != string(session.StatusPairing) {
		t.Errorf("status = %v, want %q", rec.attrs["status"], session.StatusPairing)
	}
	if rec.attrs["qr_present"] != true {
		t.Errorf("qr_present = %v, want true", rec.attrs["qr_present"])
	}
	if _, ok := rec.attrs["expires_at"]; !ok {
		t.Error("record misses expires_at, want the QR validity")
	}
	logger.AssertNoSecret(t, logs, result.QRCode)
}

func TestConnectLogsStoredCredentialsBranch(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: "disconnected"})
	sess := &storedCredsSession{FakeSession: sessiontest.NewSession(id, nil)}
	sessions := &storedCredsManager{Fake: sessiontest.New(nil), sess: sess}
	logs, log := captureServiceLogs(t)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, log)

	result, err := svc.Connect(context.Background(), id)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if result.Status != session.StatusConnected {
		t.Fatalf("Status = %q, want %q", result.Status, session.StatusConnected)
	}

	if _, ok := findServiceRecord(snapshotServiceLogs(logs), "connect instance branch", "stored-credentials"); !ok {
		t.Error("missing Debug record \"connect instance branch\" with branch stored-credentials")
	}
}

func TestConnectLogsPairingError(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: "disconnected"})
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(id, nil)
	sess.ConnectErr = errors.New("dial failed")
	sessions.Put(id, sess)
	logs, log := captureServiceLogs(t)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, log)

	if _, err := svc.Connect(context.Background(), id); err == nil {
		t.Fatal("Connect error = nil, want the session failure")
	}

	rec, ok := findServiceRecord(snapshotServiceLogs(logs), "connect instance failed", "new-pairing")
	if !ok {
		t.Fatal("missing Warn record \"connect instance failed\" with branch new-pairing")
	}
	if rec.level != zerolog.WarnLevel {
		t.Errorf("level = %v, want Warn", rec.level)
	}
	if _, ok := rec.attrs["error"]; !ok {
		t.Error("record misses error, want the session failure")
	}
}

func TestConnectLogsStaleDeviceReset(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{
		ID: id, Name: "loja", Status: string(session.StatusDisconnected),
		WhatsAppJID: "5511999999999@s.whatsapp.net",
	})
	sess := &oneShotNoDeviceSession{FakeSession: sessiontest.NewSession(id, nil), fails: 1}
	sessions := &staleConnectManager{Fake: sessiontest.New(nil), sess: sess}
	logs, log := captureServiceLogs(t)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, log)

	result, err := svc.Connect(context.Background(), id)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if result.Status != session.StatusPairing {
		t.Fatalf("Status = %q, want %q after the reset", result.Status, session.StatusPairing)
	}

	if _, ok := findServiceRecord(snapshotServiceLogs(logs), "connect pairing device gone, resetting stale pairing", "reset-stale-device"); !ok {
		t.Error("missing Debug record for the ErrNoDevice reset path")
	}
	if _, ok := findServiceRecord(snapshotServiceLogs(logs), "reset stale pairing", "reset-stale-device"); !ok {
		t.Error("missing Debug record \"reset stale pairing\"")
	}
	if _, ok := findServiceRecord(snapshotServiceLogs(logs), "connect instance branch", "new-pairing"); !ok {
		t.Error("missing Debug record \"connect instance branch\" with branch new-pairing after the reset")
	}
	logger.AssertNoSecret(t, logs, result.QRCode)
}

func TestConnectLogsPersistPairingError(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: "disconnected"})
	repo.setConnectionErr = errors.New("database down")
	logs, log := captureServiceLogs(t)
	svc := NewService(repo, sessiontest.New(nil), &fakeMedia{}, nil, nil, log)

	if _, err := svc.Connect(context.Background(), id); err == nil {
		t.Fatal("Connect error = nil, want the pairing update failure")
	}

	rec, ok := findServiceRecord(snapshotServiceLogs(logs), "connect instance failed", "persist-pairing")
	if !ok {
		t.Fatal("missing Warn record \"connect instance failed\" with branch persist-pairing")
	}
	if rec.level != zerolog.WarnLevel {
		t.Errorf("level = %v, want Warn", rec.level)
	}
}

func TestQRLogsBranches(t *testing.T) {
	t.Run("already connected", func(t *testing.T) {
		id := uuid.New()
		repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: string(session.StatusConnected)})
		sessions := sessiontest.New(nil)
		sess := sessiontest.NewSession(id, nil)
		sess.SetStatus(session.StatusConnected)
		sess.SetConnected(true)
		sessions.Put(id, sess)
		logs, log := captureServiceLogs(t)
		svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, log)

		if _, err := svc.QR(context.Background(), id); !errors.Is(err, ErrAlreadyConnected) {
			t.Fatalf("QR error = %v, want ErrAlreadyConnected", err)
		}
		if _, ok := findServiceRecord(snapshotServiceLogs(logs), "qr instance branch", "already-connected"); !ok {
			t.Error("missing Debug record \"qr instance branch\" with branch already-connected")
		}
	})

	t.Run("new pairing", func(t *testing.T) {
		id := uuid.New()
		repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: "disconnected"})
		logs, log := captureServiceLogs(t)
		svc := NewService(repo, sessiontest.New(nil), &fakeMedia{}, nil, nil, log)

		result, err := svc.QR(context.Background(), id)
		if err != nil {
			t.Fatalf("QR: %v", err)
		}
		rec, ok := findServiceRecord(snapshotServiceLogs(logs), "qr instance branch", "new-pairing")
		if !ok {
			t.Fatal("missing Debug record \"qr instance branch\" with branch new-pairing")
		}
		if rec.attrs["qr_present"] != true {
			t.Errorf("qr_present = %v, want true", rec.attrs["qr_present"])
		}
		logger.AssertNoSecret(t, logs, result.QRCode)
	})

	t.Run("stored credentials", func(t *testing.T) {
		id := uuid.New()
		repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Status: "disconnected"})
		sess := &storedCredsSession{FakeSession: sessiontest.NewSession(id, nil)}
		sessions := &storedCredsManager{Fake: sessiontest.New(nil), sess: sess}
		logs, log := captureServiceLogs(t)
		svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, log)

		if _, err := svc.QR(context.Background(), id); !errors.Is(err, ErrAlreadyConnected) {
			t.Fatalf("QR error = %v, want ErrAlreadyConnected", err)
		}
		if _, ok := findServiceRecord(snapshotServiceLogs(logs), "qr instance branch", "stored-credentials"); !ok {
			t.Error("missing Debug record \"qr instance branch\" with branch stored-credentials")
		}
	})
}
