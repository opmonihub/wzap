package instance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
)

func newParityTestInstance(t *testing.T) (*model.Instance, *fakeRepo) {
	t.Helper()
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Connection: model.InstanceConnection{Status: "connected"}}
	return inst, newFakeRepo(*inst)
}

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

func TestEditMessageMapsNotConnected(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sess.EditErr = session.ErrNotConnected
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	_, err := svc.EditMessage(context.Background(), inst.ID, "chat@s.whatsapp.net", "WAID-1", "oi")
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("EditMessage err = %v, want ErrNotConnected", err)
	}
}

func TestEditMessageValidation(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()

	for name, tc := range map[string]struct {
		chat, msg, text string
	}{
		"blank text":      {chat: "a@s.whatsapp.net", msg: "WAID-1", text: "   "},
		"too long":        {chat: "a@s.whatsapp.net", msg: "WAID-1", text: strings.Repeat("x", 4097)},
		"blank chat":      {chat: "  ", msg: "WAID-1", text: "oi"},
		"blank messageID": {chat: "a@s.whatsapp.net", msg: "", text: "oi"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.EditMessage(ctx, inst.ID, tc.chat, tc.msg, tc.text); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("EditMessage err = %v, want ErrInvalidInput", err)
			}
		})
	}
	if got := sess.EditCalls(); len(got) != 0 {
		t.Errorf("EditCalls = %d, want 0 (validation before session)", len(got))
	}
}

func TestEditMessageDelegatesToSession(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())

	out, err := svc.EditMessage(context.Background(), inst.ID, "5511999999999@s.whatsapp.net", "WAID-1", "novo texto")
	if err != nil {
		t.Fatalf("EditMessage: %v", err)
	}
	if out == "" {
		t.Fatal("EditMessage id empty, want the session id")
	}
	calls := sess.EditCalls()
	if len(calls) != 1 || calls[0].Text != "novo texto" {
		t.Errorf("EditCalls = %+v, want one edit with the new text", calls)
	}
}
