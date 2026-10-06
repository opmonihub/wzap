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

func groupService(t *testing.T, sessions *sessiontest.Fake) (*Service, *model.Instance) {
	t.Helper()
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Connection: model.InstanceConnection{Status: "connected"}}
	repo := newFakeRepo(*inst)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())
	return svc, inst
}

func TestServiceCreateGroupDelegatesToSession(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := groupService(t, sessions)

	group, err := svc.CreateGroup(context.Background(), inst.ID, CreateGroupInput{
		Name:         "Time do churrasco",
		Participants: []string{"5511888888888@s.whatsapp.net"},
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if group.Name != "Time do churrasco" || group.JID == "" {
		t.Errorf("group = %+v, want the created subject and a JID", group)
	}

	sess, _ := sessions.Get(inst.ID)
	calls := sess.(*sessiontest.FakeSession).GroupCalls()
	if len(calls) != 1 || calls[0].Op != "create" || calls[0].Name != "Time do churrasco" {
		t.Errorf("group calls = %+v, want one create", calls)
	}
}

func TestServiceGroupLifecycle(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := groupService(t, sessions)
	ctx := context.Background()

	created, err := svc.CreateGroup(ctx, inst.ID, CreateGroupInput{Name: "Time"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	got, err := svc.GetGroup(ctx, inst.ID, created.JID)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if got.Name != "Time" {
		t.Errorf("GetGroup name = %q, want Time", got.Name)
	}

	name := "Novo assunto"
	desc := "Nova descrição"
	updated, err := svc.UpdateGroup(ctx, inst.ID, created.JID, UpdateGroupInput{Name: &name, Description: &desc})
	if err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	if updated.Name != name || updated.Description != desc {
		t.Errorf("updated = %+v, want the new subject and topic", updated)
	}

	if err := svc.UpdateGroupParticipants(ctx, inst.ID, created.JID, "promote", []string{"5511888888888@s.whatsapp.net"}); err != nil {
		t.Fatalf("UpdateGroupParticipants: %v", err)
	}

	code, err := svc.GetGroupInvite(ctx, inst.ID, created.JID)
	if err != nil {
		t.Fatalf("GetGroupInvite: %v", err)
	}
	joined, err := svc.JoinGroup(ctx, inst.ID, code)
	if err != nil {
		t.Fatalf("JoinGroup: %v", err)
	}
	if joined != created.JID {
		t.Errorf("joined = %q, want %q", joined, created.JID)
	}

	fresh, err := svc.ResetGroupInvite(ctx, inst.ID, created.JID)
	if err != nil {
		t.Fatalf("ResetGroupInvite: %v", err)
	}
	if fresh == code {
		t.Errorf("reset code = %q, want a fresh code unlike %q", fresh, code)
	}
	if _, err := svc.JoinGroup(ctx, inst.ID, code); !errors.Is(err, ErrGroupNotFound) {
		t.Errorf("JoinGroup with revoked code error = %v, want %v", err, ErrGroupNotFound)
	}

	if err := svc.SetGroupPhoto(ctx, inst.ID, created.JID, []byte{0xff, 0xd8}); err != nil {
		t.Fatalf("SetGroupPhoto: %v", err)
	}
	if err := svc.LeaveGroup(ctx, inst.ID, created.JID); err != nil {
		t.Fatalf("LeaveGroup: %v", err)
	}
}

func TestServiceGroupErrorMapping(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := groupService(t, sessions)
	ctx := context.Background()

	if _, err := svc.GetGroup(ctx, inst.ID, "120363099999999999@g.us"); !errors.Is(err, ErrGroupNotFound) {
		t.Errorf("GetGroup unknown error = %v, want %v", err, ErrGroupNotFound)
	}

	sess, _ := sessions.Get(inst.ID)
	fake := sess.(*sessiontest.FakeSession)
	fake.GroupErr = session.ErrForbidden
	if err := svc.UpdateGroupParticipants(ctx, inst.ID, "120363000000000000@g.us", "promote", []string{"x"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("UpdateGroupParticipants error = %v, want %v", err, ErrForbidden)
	}

	fake.GroupErr = session.ErrNotConnected
	if _, err := svc.GetGroup(ctx, inst.ID, "120363000000000000@g.us"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("GetGroup offline error = %v, want %v", err, ErrNotConnected)
	}

	if _, err := svc.GetGroup(ctx, uuid.New(), "120363000000000000@g.us"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGroup unknown instance error = %v, want %v", err, ErrNotFound)
	}
}
