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

func statusService(t *testing.T, sessions *sessiontest.Fake) (*Service, *model.Instance) {
	t.Helper()
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Connection: model.InstanceConnection{Status: "connected"}}
	repo := newFakeRepo(*inst)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())
	return svc, inst
}

func TestServicePublishStatusDelegatesToSession(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := statusService(t, sessions)

	id, err := svc.PublishStatus(context.Background(), inst.ID, session.StatusInput{Text: "bom dia"})
	if err != nil {
		t.Fatalf("PublishStatus: %v", err)
	}
	if id == "" {
		t.Error("PublishStatus id is empty, want the upstream id")
	}

	sess, _ := sessions.Get(inst.ID)
	calls := sess.(*sessiontest.FakeSession).StatusCalls()
	if len(calls) != 1 || calls[0].Op != "publish" {
		t.Errorf("status calls = %+v, want one publish", calls)
	}
}

func TestServiceStatusListDelete(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := statusService(t, sessions)
	ctx := context.Background()

	id, err := svc.PublishStatus(ctx, inst.ID, session.StatusInput{Text: "oi"})
	if err != nil {
		t.Fatalf("PublishStatus: %v", err)
	}

	statuses, err := svc.ListStatuses(ctx, inst.ID)
	if err != nil {
		t.Fatalf("ListStatuses: %v", err)
	}
	if len(statuses) != 1 || statuses[0].ID != id {
		t.Errorf("statuses = %+v, want the published one", statuses)
	}

	if err := svc.DeleteStatus(ctx, inst.ID, id); err != nil {
		t.Fatalf("DeleteStatus: %v", err)
	}
	if err := svc.DeleteStatus(ctx, inst.ID, id); !errors.Is(err, ErrStatusNotFound) {
		t.Errorf("DeleteStatus twice: err = %v, want %v", err, ErrStatusNotFound)
	}
}

func TestServiceStatusMapsDisconnected(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := statusService(t, sessions)

	sess, _ := sessions.Get(inst.ID)
	sess.(*sessiontest.FakeSession).StatusErr = session.ErrNotConnected

	if _, err := svc.PublishStatus(context.Background(), inst.ID, session.StatusInput{Text: "oi"}); !errors.Is(err, ErrNotConnected) {
		t.Errorf("PublishStatus: err = %v, want %v", err, ErrNotConnected)
	}
	if _, err := svc.ListStatuses(context.Background(), inst.ID); !errors.Is(err, ErrNotConnected) {
		t.Errorf("ListStatuses: err = %v, want %v", err, ErrNotConnected)
	}
	if err := svc.DeleteStatus(context.Background(), inst.ID, "wamid.s1"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("DeleteStatus: err = %v, want %v", err, ErrNotConnected)
	}
}
