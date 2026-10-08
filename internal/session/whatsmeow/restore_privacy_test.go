package whatsmeow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/model"
	"wzap/internal/session"
)

type bindingFailureRepo struct {
	*fakeInstanceRepo
	err error
}

func (r *bindingFailureRepo) GetByDeviceJID(context.Context, string) (*model.Instance, error) {
	return nil, r.err
}

func TestRestoreParseFailureHidesBoundDevice(t *testing.T) {
	const bound = "5511999999999:synthetic@s.whatsapp.net"
	var logs bytes.Buffer
	sink := &recordingSink{}
	id := uuid.New()
	m := &Manager{log: zerolog.New(&logs), sink: sink, instances: &fakeInstanceRepo{instances: []model.Instance{{ID: id, Connection: model.InstanceConnection{DeviceJID: bound}}}}}
	if err := m.RestoreAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertRestorePrivacy(t, logs.String(), sink.last(t).reason)
}

func TestLoadBoundDeviceHidesUpstreamAndPreservesCause(t *testing.T) {
	cause := errors.Join(context.DeadlineExceeded, errors.New("query for 5511999999999@s.whatsapp.net failed"))
	m := &Manager{instances: &bindingFailureRepo{err: cause}}
	_, err := m.loadBoundDevice(context.Background(), &model.Instance{ID: uuid.New(), Connection: model.InstanceConnection{DeviceJID: "5511999999999@s.whatsapp.net"}})
	if !errors.Is(err, cause) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("device lookup lost error classification: %v", err)
	}
	if err == nil {
		t.Fatal("lookup unexpectedly succeeded")
	}
	assertRestorePrivacy(t, err.Error())
}

func TestClassifySessionErrorPreservesUpstreamContextPrivately(t *testing.T) {
	cause := fmt.Errorf("device 5511999999999@s.whatsapp.net: %w", context.DeadlineExceeded)
	err := classifySessionError(cause)
	if !errors.Is(err, session.ErrTransient) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("classification discarded the upstream context: %v", err)
	}
	assertRestorePrivacy(t, err.Error())
}

func TestRestoreStoredDevicePrivacy(t *testing.T) {
	for _, scenario := range []string{"missing", "mismatch", "upstream", "success"} {
		t.Run(scenario, func(t *testing.T) {
			m := newTestManager(t)
			var logs bytes.Buffer
			m.log = zerolog.New(&logs)
			sink := &recordingSink{}
			m.sink = sink
			id := uuid.New()
			bound := "5511888888888@s.whatsapp.net"
			if scenario != "missing" {
				bound = saveTestDevice(t, m.devices, "5511999999999").String()
			}
			if scenario == "mismatch" {
				// ParseJID normalizes a zero device component away. The store
				// returns the plain JID, which disagrees with this binding.
				bound = strings.Replace(bound, "@", ":0@", 1)
			}
			upstream := fmt.Errorf("upstream refused device %s: %w", bound, session.ErrNoDevice)
			m.restoreConnect = func(_ context.Context, sess *instanceSession) error {
				if scenario == "upstream" {
					return upstream
				}
				sess.setStatus(session.StatusConnected, sess.JID(), "")
				return nil
			}
			inst := model.Instance{ID: id, Connection: model.InstanceConnection{DeviceJID: bound}}
			m.instances = &fakeInstanceRepo{instances: []model.Instance{inst}}
			if scenario == "missing" || scenario == "mismatch" {
				_, err := m.loadBoundDevice(context.Background(), &inst)
				if !errors.Is(err, session.ErrNoDevice) {
					t.Fatalf("loadBoundDevice = %v, want missing/mismatched device", err)
				}
				assertRestorePrivacy(t, err.Error())
			}
			if err := m.RestoreAll(context.Background()); err != nil {
				t.Fatal(err)
			}
			event := sink.last(t)
			wantStatus := session.StatusError
			if scenario == "success" {
				wantStatus = session.StatusConnected
			}
			if event.status != wantStatus {
				t.Errorf("restore status = %s, want %s", event.status, wantStatus)
			}
			assertRestorePrivacy(t, logs.String(), event.reason)
			if !strings.Contains(logs.String(), id.String()) {
				t.Error("restore log omitted the opaque instance id")
			}
		})
	}
}

func assertRestorePrivacy(t *testing.T, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(value, "@"+types.DefaultUserServer) || strings.Contains(value, "5511999999999") || strings.Contains(value, "5511888888888") {
			t.Errorf("restore exposed device identity: %s", value)
		}
	}
}
