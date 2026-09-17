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

func profileService(t *testing.T, sessions *sessiontest.Fake) (*Service, *model.Instance) {
	t.Helper()
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Status: "connected"}
	repo := newFakeRepo(*inst)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	svc := NewService(repo, sessions, &fakeMedia{}, nil, nil, zerolog.Nop())
	return svc, inst
}

func TestServiceProfileRoundTrip(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := profileService(t, sessions)
	ctx := context.Background()

	if err := svc.SetProfileName(ctx, inst.ID, "Loja nova"); err != nil {
		t.Fatalf("SetProfileName: %v", err)
	}
	if err := svc.SetProfileStatusText(ctx, inst.ID, "aberto"); err != nil {
		t.Fatalf("SetProfileStatusText: %v", err)
	}
	if err := svc.SetProfilePhoto(ctx, inst.ID, []byte("bytes")); err != nil {
		t.Fatalf("SetProfilePhoto: %v", err)
	}

	profile, err := svc.GetProfile(ctx, inst.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.Name != "Loja nova" || profile.StatusText != "aberto" {
		t.Errorf("profile = %+v, want the updated name and recado", profile)
	}

	sess, _ := sessions.Get(inst.ID)
	calls := sess.(*sessiontest.FakeSession).ProfileCalls()
	if len(calls) != 4 {
		t.Errorf("profile calls = %+v, want set-name, set-status, set-photo and get", calls)
	}
}

func TestServiceProfileMapsErrors(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := profileService(t, sessions)
	ctx := context.Background()

	sess, _ := sessions.Get(inst.ID)
	fake := sess.(*sessiontest.FakeSession)
	fake.ProfileErr = session.ErrNotConnected

	if _, err := svc.GetProfile(ctx, inst.ID); !errors.Is(err, ErrNotConnected) {
		t.Errorf("GetProfile: err = %v, want %v", err, ErrNotConnected)
	}
	if err := svc.SetProfileName(ctx, inst.ID, "Loja"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("SetProfileName: err = %v, want %v", err, ErrNotConnected)
	}

	fake.ProfileErr = session.ErrUnsupported
	if err := svc.SetProfileName(ctx, inst.ID, "Loja"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("SetProfileName unsupported: err = %v, want %v", err, ErrUnsupported)
	}
	if err := svc.SetProfilePhoto(ctx, inst.ID, []byte("bytes")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("SetProfilePhoto unsupported: err = %v, want %v", err, ErrUnsupported)
	}
}

func TestServicePrivacyRoundTrip(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := profileService(t, sessions)
	ctx := context.Background()

	applied, err := svc.SetPrivacy(ctx, inst.ID, session.Privacy{LastSeen: "contacts", ReadReceipts: "none"})
	if err != nil {
		t.Fatalf("SetPrivacy: %v", err)
	}
	if applied.LastSeen != "contacts" || applied.ReadReceipts != "none" {
		t.Errorf("applied = %+v, want contacts and none", applied)
	}

	privacy, err := svc.GetPrivacy(ctx, inst.ID)
	if err != nil {
		t.Fatalf("GetPrivacy: %v", err)
	}
	if privacy.LastSeen != "contacts" || privacy.ReadReceipts != "none" {
		t.Errorf("privacy = %+v, want the applied settings", privacy)
	}

	sess, _ := sessions.Get(inst.ID)
	calls := sess.(*sessiontest.FakeSession).PrivacyCalls()
	if len(calls) != 2 || calls[0].Op != "set" || calls[1].Op != "get" {
		t.Errorf("privacy calls = %+v, want set then get", calls)
	}
	if calls[0].Input.LastSeen != "contacts" {
		t.Errorf("set input = %+v, want contacts last_seen", calls[0].Input)
	}
}

func TestServicePrivacyMapsErrors(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := profileService(t, sessions)
	ctx := context.Background()

	sess, _ := sessions.Get(inst.ID)
	sess.(*sessiontest.FakeSession).PrivacyErr = session.ErrNotConnected

	if _, err := svc.GetPrivacy(ctx, inst.ID); !errors.Is(err, ErrNotConnected) {
		t.Errorf("GetPrivacy: err = %v, want %v", err, ErrNotConnected)
	}
	if _, err := svc.SetPrivacy(ctx, inst.ID, session.Privacy{LastSeen: "all"}); !errors.Is(err, ErrNotConnected) {
		t.Errorf("SetPrivacy: err = %v, want %v", err, ErrNotConnected)
	}
}
