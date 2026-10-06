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

// TestServiceHealthNotFound verifica que o health de uma instância
// desconhecida é ErrNotFound, como o Get.
func TestServiceHealthNotFound(t *testing.T) {
	svc := NewService(newFakeRepo(), sessiontest.New(nil), &fakeMedia{}, nil, nil, zerolog.Nop())

	if _, err := svc.Health(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Health error = %v, want ErrNotFound", err)
	}
}

// TestServiceHealthWithoutSession verifica o health sem sessão em memória:
// não há socket vivo e o estado vivo é disconnected, com o DBStatus vindo
// do banco.
func TestServiceHealthWithoutSession(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: string(session.StatusDisconnected)}})
	svc := NewService(repo, sessiontest.New(nil), &fakeMedia{}, nil, nil, zerolog.Nop())

	got, err := svc.Health(context.Background(), id)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.HasSession {
		t.Error("HasSession = true sem sessão registrada")
	}
	if got.SocketConnected {
		t.Error("SocketConnected = true sem sessão registrada")
	}
	if got.LiveStatus != session.StatusDisconnected {
		t.Errorf("LiveStatus = %q, want %q", got.LiveStatus, session.StatusDisconnected)
	}
	if got.DBStatus != string(session.StatusDisconnected) {
		t.Errorf("DBStatus = %q, want %q", got.DBStatus, session.StatusDisconnected)
	}
}

// TestServiceHealthLiveSocket verifica o health com sessão viva: o estado
// vivo e o socket vêm da sessão, o DBStatus do banco.
func TestServiceHealthLiveSocket(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: string(session.StatusConnected)}})
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(id, nil)
	sess.SetStatus(session.StatusConnected)
	sess.SetConnected(true)
	sessions.Put(id, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())

	got, err := svc.Health(context.Background(), id)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !got.HasSession {
		t.Error("HasSession = false com sessão registrada")
	}
	if got.LiveStatus != session.StatusConnected {
		t.Errorf("LiveStatus = %q, want %q", got.LiveStatus, session.StatusConnected)
	}
	if !got.SocketConnected {
		t.Error("SocketConnected = false com o socket vivo")
	}
	if got.DBStatus != string(session.StatusConnected) {
		t.Errorf("DBStatus = %q, want %q", got.DBStatus, session.StatusConnected)
	}
}

// TestServiceHealthDetectsDeadSocket verifica o sintoma clássico do socket
// morto: o banco ainda diz "connected", mas a sessão está disconnected com
// o socket caído. O health distingue os dois lados sem criar sessão.
func TestServiceHealthDetectsDeadSocket(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: string(session.StatusConnected), DeviceJID: "5511999999999@s.whatsapp.net"}})
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(id, nil)
	sess.SetStatus(session.StatusDisconnected)
	sess.SetConnected(false)
	sessions.Put(id, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())

	got, err := svc.Health(context.Background(), id)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.DBStatus != string(session.StatusConnected) {
		t.Errorf("DBStatus = %q, want %q (o banco ainda não viu o drop)", got.DBStatus, session.StatusConnected)
	}
	if got.LiveStatus != session.StatusDisconnected {
		t.Errorf("LiveStatus = %q, want %q", got.LiveStatus, session.StatusDisconnected)
	}
	if got.SocketConnected {
		t.Error("SocketConnected = true com o socket morto")
	}
	if len(sessions.CreateCalls()) != 0 {
		t.Errorf("Health criou %d sessões, want 0 (peek read-only)", len(sessions.CreateCalls()))
	}
}
