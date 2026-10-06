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

func TestServicePairPhoneReturnsCodeWithChannelExpiry(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: string(session.StatusPairing)}})
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(id, nil)
	sessions.Put(id, sess)
	if _, _, err := sess.Connect(context.Background()); err != nil {
		t.Fatalf("setup session Connect: %v", err)
	}
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())

	result, err := svc.PairPhone(context.Background(), id, "5547988359190")
	if err != nil {
		t.Fatalf("PairPhone: %v", err)
	}

	if result.Code == "" {
		t.Error("PairPhone code is empty, want the session code")
	}
	if result.ExpiresAt.IsZero() {
		t.Error("PairPhone ExpiresAt is the zero time, want the channel expiry")
	}
	calls := sess.PairPhoneCalls()
	if len(calls) != 1 || calls[0].Number != "5547988359190" {
		t.Errorf("session PairPhone calls = %+v, want the dialed number once", calls)
	}
}

func TestServicePairPhoneExpiredChannelAnswersConflictWithoutEmittingCode(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: string(session.StatusPairing)}})
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(id, nil)
	sess.SetStatus(session.StatusPairing)
	sess.QRErr = errors.New("no qr code available")
	sessions.Put(id, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())

	_, err := svc.PairPhone(context.Background(), id, "5547988359190")
	if !errors.Is(err, ErrNoPairingChannel) {
		t.Fatalf("PairPhone error = %v, want ErrNoPairingChannel", err)
	}
	if got := len(sess.PairPhoneCalls()); got != 0 {
		t.Errorf("session PairPhone calls = %d, want 0 (no code may be emitted without a readable channel)", got)
	}
}
