package inbound

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
	"wzap/internal/storage"
)

type commandInstanceRepo struct {
	storage.InstanceRepository
	inst *model.Instance
}

func (f *fakeInstances) Connect(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error) {
	return f.lifecycle.Connect(ctx, id)
}

func (f *fakeInstances) PairPhone(ctx context.Context, id uuid.UUID, number string) (instance.PairPhoneResult, error) {
	return f.lifecycle.PairPhone(ctx, id, number)
}

func (r *commandInstanceRepo) Get(context.Context, uuid.UUID) (*model.Instance, error) {
	copy := *r.inst
	return &copy, nil
}

func (r *commandInstanceRepo) SetConnection(_ context.Context, _ uuid.UUID, status, jid string) error {
	r.inst.Connection.Status, r.inst.Connection.DeviceJID = status, jid
	return nil
}

func TestCommandInitUsesFreshInstanceLifecycle(t *testing.T) {
	for _, command := range []string{"init", "init:5511999999999"} {
		t.Run(command, func(t *testing.T) {
			fx := newFixture(t, enabledConnector(), globalOn())
			_ = fx.sessions.Remove(context.Background(), fx.instance)
			fx.instances.inst.Connection.Status = "disconnected"
			repo := &commandInstanceRepo{inst: fx.instances.inst}
			fx.handler.instances = instance.NewService(repo, fx.sessions, nil, nil, nil, zerolog.Nop())
			status, err := fx.handler.HandleCommand(context.Background(), fx.instance, command, 7)
			if err != nil || status != 200 {
				t.Fatalf("command = %d, %v", status, err)
			}
			sess, ok := fx.sessions.Get(fx.instance)
			if !ok || sess.Status() != session.StatusPairing || repo.inst.Connection.Status != "pairing" {
				t.Fatalf("fresh session not created/started: session = %v, exists = %v, stored = %s", sess, ok, repo.inst.Connection.Status)
			}
			if len(fx.chats.creates) != 1 {
				t.Fatalf("confirmations = %d", len(fx.chats.creates))
			}
			confirmation := fx.chats.creates[0].content
			if command == "init" {
				if !strings.Contains(confirmation, "QR:") {
					t.Errorf("QR confirmation missing: %s", confirmation)
				}
			} else {
				if len(sess.(*sessiontest.FakeSession).PairPhoneCalls()) != 1 || !strings.Contains(confirmation, "Código de pareamento") {
					t.Errorf("phone pairing not requested after connect: %s", confirmation)
				}
			}
		})
	}
}

func TestCommandStatusHidesDeviceJID(t *testing.T) {
	fx := newFixture(t, enabledConnector(), globalOn())
	fx.instances.inst.Connection.DeviceJID = "synthetic-device-identity@s.whatsapp.net"
	status, err := fx.handler.HandleCommand(context.Background(), fx.instance, "status", 7)
	if err != nil || status != 200 || len(fx.chats.creates) != 1 {
		t.Fatalf("status failed: %d, %v", status, err)
	}
	if got := fx.chats.creates[0].content; strings.Contains(got, "@s.whatsapp.net") || !strings.Contains(got, "loja") || !strings.Contains(got, "connected") {
		t.Errorf("unsafe/incomplete status: %s", got)
	}
}

func TestCommandInitFailureConfirmationHidesUpstreamDevice(t *testing.T) {
	for _, command := range []string{"init", "init:5511999999999"} {
		t.Run(command, func(t *testing.T) {
			fx := newFixture(t, enabledConnector(), globalOn())
			sess, _ := fx.sessions.Get(fx.instance)
			cause := errors.New("upstream refused synthetic-device@s.whatsapp.net")
			if command == "init" {
				sess.(*sessiontest.FakeSession).ConnectErr = cause
			} else {
				sess.(*sessiontest.FakeSession).PairPhoneErr = cause
			}
			status, err := fx.handler.HandleCommand(context.Background(), fx.instance, command, 7)
			if err != nil || status != 200 || len(fx.chats.creates) != 1 {
				t.Fatalf("init failure = %d, %v", status, err)
			}
			if got := fx.chats.creates[0].content; strings.Contains(got, "@s.whatsapp.net") || !strings.Contains(got, "Falha") {
				t.Errorf("failure exposed upstream device: %s", got)
			}
		})
	}
}

func TestCommandInitKeepsConnectedSession(t *testing.T) {
	fx := newFixture(t, enabledConnector(), globalOn())
	sess, _ := fx.sessions.Get(fx.instance)
	sess.(*sessiontest.FakeSession).SetConnected(true)
	status, err := fx.handler.HandleCommand(context.Background(), fx.instance, "init:5511999999999", 7)
	if err != nil || status != 200 || len(fx.chats.creates) != 1 {
		t.Fatalf("connected init = %d, %v", status, err)
	}
	if sess.Status() != session.StatusConnected || sess.(*sessiontest.FakeSession).ConnectCalls() != 0 || len(sess.(*sessiontest.FakeSession).PairPhoneCalls()) != 0 || !strings.Contains(fx.chats.creates[0].content, "já conectado") {
		t.Errorf("connected instance restarted pairing: %s", fx.chats.creates[0].content)
	}
}
