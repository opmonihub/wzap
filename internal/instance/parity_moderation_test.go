package instance

import (
	"context"
	"errors"
	"testing"

	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
)

func TestUpdateGroupSettingsRejectsBadMode(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	mgr.Put(inst.ID, sessiontest.NewSession(inst.ID, nil))
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	bad := "everyone"
	_, err := svc.UpdateGroupSettings(context.Background(), inst.ID, "12036300000001@g.us", nil, nil, &bad, nil)
	if err == nil {
		t.Fatalf("UpdateGroupSettings bad mode err = nil, want invalid input")
	}
}

func TestGetGroupRequests(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	groupJID := "120363000000000021@g.us"
	sess.PutGroup(session.GroupInfo{JID: groupJID, Name: "Requests"})

	if _, err := svc.GetGroupRequests(ctx, inst.ID, groupJID); err != nil {
		t.Fatalf("GetGroupRequests: %v", err)
	}

	if _, err := svc.GetGroupRequests(ctx, inst.ID, "120363099999999999@g.us"); !errors.Is(err, ErrGroupNotFound) {
		t.Errorf("unknown group err = %v, want ErrGroupNotFound", err)
	}

	sess.GroupErr = session.ErrNotConnected
	if _, err := svc.GetGroupRequests(ctx, inst.ID, groupJID); !errors.Is(err, ErrNotConnected) {
		t.Errorf("offline err = %v, want ErrNotConnected", err)
	}
}

func TestUpdateGroupRequestsValidation(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	groupJID := "120363000000000022@g.us"
	sess.PutGroup(session.GroupInfo{JID: groupJID, Name: "Requests"})

	for name, action := range map[string]string{"freeze": "freeze", "blank": "  "} {
		t.Run(name, func(t *testing.T) {
			if err := svc.UpdateGroupRequests(ctx, inst.ID, groupJID, action, []string{"a@s.whatsapp.net"}); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("action %q err = %v, want ErrInvalidInput", action, err)
			}
		})
	}
	if err := svc.UpdateGroupRequests(ctx, inst.ID, groupJID, "approve", nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty participants err = %v, want ErrInvalidInput", err)
	}
	if got := sess.GroupCalls(); len(got) != 0 {
		t.Errorf("GroupCalls = %d, want 0 (validation before session)", len(got))
	}

	if err := svc.UpdateGroupRequests(ctx, inst.ID, groupJID, "approve", []string{"a@s.whatsapp.net"}); err != nil {
		t.Fatalf("UpdateGroupRequests approve: %v", err)
	}
	if err := svc.UpdateGroupRequests(ctx, inst.ID, groupJID, "decline", []string{"b@s.whatsapp.net"}); err != nil {
		t.Fatalf("UpdateGroupRequests decline: %v", err)
	}
	if err := svc.UpdateGroupRequests(ctx, inst.ID, "120363099999999999@g.us", "approve", []string{"a@s.whatsapp.net"}); !errors.Is(err, ErrGroupNotFound) {
		t.Errorf("unknown group err = %v, want ErrGroupNotFound", err)
	}
}

func TestUpdateGroupSettings(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	groupJID := "120363000000000023@g.us"
	sess.PutGroup(session.GroupInfo{JID: groupJID, Name: "Settings"})

	if _, err := svc.UpdateGroupSettings(ctx, inst.ID, groupJID, nil, nil, nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("no fields err = %v, want ErrInvalidInput", err)
	}
	badMember := "moderators"
	if _, err := svc.UpdateGroupSettings(ctx, inst.ID, groupJID, nil, nil, nil, &badMember); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad memberAddMode err = %v, want ErrInvalidInput", err)
	}
	if got := sess.GroupCalls(); len(got) != 0 {
		t.Errorf("GroupCalls = %d, want 0 (validation before session)", len(got))
	}

	announce, locked := true, true
	joinApproval, memberAddMode := "on", "admin_only"
	got, err := svc.UpdateGroupSettings(ctx, inst.ID, groupJID, &announce, &locked, &joinApproval, &memberAddMode)
	if err != nil {
		t.Fatalf("UpdateGroupSettings: %v", err)
	}
	if got.JID != groupJID {
		t.Errorf("group JID = %q, want %q", got.JID, groupJID)
	}
	calls := sess.GroupCalls()
	if len(calls) != 5 {
		t.Fatalf("GroupCalls = %d, want 4 setters plus the re-read", len(calls))
	}
	for i, want := range []string{"announce", "locked", "join-approval", "member-add"} {
		if calls[i].Op != want {
			t.Errorf("call %d op = %q, want %q", i, calls[i].Op, want)
		}
	}
	if calls[4].Op != "get" {
		t.Errorf("call 4 op = %q, want %q (metadata re-read)", calls[4].Op, "get")
	}
}
