package instance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
)

func callService(t *testing.T, sessions *sessiontest.Fake) (*Service, *model.Instance) {
	t.Helper()
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Connection: model.InstanceConnection{Status: "connected"}}
	repo := newFakeRepo(*inst)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())
	return svc, inst
}

func TestServiceRejectCallDelegatesToSession(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := callService(t, sessions)

	if err := svc.RejectCall(context.Background(), inst.ID, "5511888888888@s.whatsapp.net", "call-1"); err != nil {
		t.Fatalf("RejectCall: %v", err)
	}

	sess, _ := sessions.Get(inst.ID)
	calls := sess.(*sessiontest.FakeSession).RejectCalls()
	if len(calls) != 1 || calls[0].FromJID != "5511888888888@s.whatsapp.net" || calls[0].CallID != "call-1" {
		t.Errorf("reject calls = %+v, want the caller and call id", calls)
	}
}

func TestServiceRejectCallMapsErrors(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := callService(t, sessions)

	sess, _ := sessions.Get(inst.ID)
	fake := sess.(*sessiontest.FakeSession)

	fake.CallErr = session.ErrNotConnected
	if err := svc.RejectCall(context.Background(), inst.ID, "5511888888888@s.whatsapp.net", "call-1"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("RejectCall disconnected: err = %v, want %v", err, ErrNotConnected)
	}

	fake.CallErr = session.ErrUnsupported
	if err := svc.RejectCall(context.Background(), inst.ID, "5511888888888@s.whatsapp.net", "call-1"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("RejectCall unsupported: err = %v, want %v", err, ErrUnsupported)
	}
}
