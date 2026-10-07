package instance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
)

func TestMarkNewsletterViewedRejectsEmpty(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	mgr.Put(inst.ID, sessiontest.NewSession(inst.ID, nil))
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	if err := svc.MarkNewsletterViewed(context.Background(), inst.ID, "123@newsletter", nil); err == nil {
		t.Fatalf("MarkNewsletterViewed empty err = nil, want invalid input")
	}
}

func TestUpdateBlocklist(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	jid := "5511888888888@s.whatsapp.net"

	if err := svc.UpdateBlocklist(ctx, inst.ID, "  ", "block"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank jid err = %v, want ErrInvalidInput", err)
	}
	if err := svc.UpdateBlocklist(ctx, inst.ID, jid, "freeze"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad action err = %v, want ErrInvalidInput", err)
	}
	if got := sess.BlocklistCalls(); len(got) != 0 {
		t.Errorf("BlocklistCalls = %d, want 0 (validation before session)", len(got))
	}

	if err := svc.UpdateBlocklist(ctx, inst.ID, jid, "block"); err != nil {
		t.Fatalf("UpdateBlocklist block: %v", err)
	}
	if err := svc.UpdateBlocklist(ctx, inst.ID, jid, "unblock"); err != nil {
		t.Fatalf("UpdateBlocklist unblock: %v", err)
	}

	sess.BlocklistErr = session.ErrNotConnected
	if err := svc.UpdateBlocklist(ctx, inst.ID, jid, "block"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("offline err = %v, want ErrNotConnected", err)
	}
}

func TestDisappearingTimers(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	chat := "5511999999999@s.whatsapp.net"

	for _, d := range []time.Duration{0, 24 * time.Hour, 7 * 24 * time.Hour, 90 * 24 * time.Hour} {
		if err := svc.SetDisappearingTimer(ctx, inst.ID, chat, d); err != nil {
			t.Fatalf("SetDisappearingTimer %v: %v", d, err)
		}
		if err := svc.SetDefaultDisappearingTimer(ctx, inst.ID, d); err != nil {
			t.Fatalf("SetDefaultDisappearingTimer %v: %v", d, err)
		}
	}
	if err := svc.SetDisappearingTimer(ctx, inst.ID, chat, 5*time.Hour); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad duration err = %v, want ErrInvalidInput", err)
	}
	if err := svc.SetDefaultDisappearingTimer(ctx, inst.ID, 5*time.Hour); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad default duration err = %v, want ErrInvalidInput", err)
	}
	if err := svc.SetDisappearingTimer(ctx, inst.ID, "  ", 24*time.Hour); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank chat err = %v, want ErrInvalidInput", err)
	}

	sess.ChatSettingsErr = session.ErrNotConnected
	if err := svc.SetDisappearingTimer(ctx, inst.ID, chat, 24*time.Hour); !errors.Is(err, ErrNotConnected) {
		t.Errorf("offline err = %v, want ErrNotConnected", err)
	}
}

// The default disappearing PUT keeps a persisted echo of the last applied
// timer: it is written only after the upstream accepted the change, so a
// session failure leaves the stored echo untouched and off (0) stays
// distinguishable from never configured (nil).
func TestSetDefaultDisappearingTimerPersistsEcho(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()

	sess.ChatSettingsErr = session.ErrNotConnected
	if err := svc.SetDefaultDisappearingTimer(ctx, inst.ID, 24*time.Hour); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("offline err = %v, want ErrNotConnected", err)
	}
	if calls := repo.setDefaultDisappearingCalls; len(calls) != 0 {
		t.Fatalf("persisted %+v despite the session failure", calls)
	}

	sess.ChatSettingsErr = nil
	for _, d := range []time.Duration{24 * time.Hour, 0} {
		if err := svc.SetDefaultDisappearingTimer(ctx, inst.ID, d); err != nil {
			t.Fatalf("SetDefaultDisappearingTimer %v: %v", d, err)
		}
	}
	calls := repo.setDefaultDisappearingCalls
	if len(calls) != 2 || calls[0].duration != 24*time.Hour || calls[1].duration != 0 {
		t.Fatalf("persisted echo calls = %+v, want [24h 0]", calls)
	}
	stored, err := repo.Get(ctx, inst.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.DefaultDisappearing == nil || *stored.DefaultDisappearing != 0 {
		t.Errorf("stored echo = %v, want 0 (off), distinct from never configured", stored.DefaultDisappearing)
	}
}

func TestSubscribePresence(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()

	if err := svc.SubscribePresence(ctx, inst.ID, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank jid err = %v, want ErrInvalidInput", err)
	}
	if got := sess.ChatSettingsCalls(); len(got) != 0 {
		t.Errorf("ChatSettingsCalls = %d, want 0 (validation before session)", len(got))
	}
	if err := svc.SubscribePresence(ctx, inst.ID, "5511999999999@s.whatsapp.net"); err != nil {
		t.Fatalf("SubscribePresence: %v", err)
	}
}

func TestGetContactQRLink(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()

	link, err := svc.GetContactQRLink(ctx, inst.ID, false)
	if err != nil {
		t.Fatalf("GetContactQRLink: %v", err)
	}
	if link == "" {
		t.Error("link empty, want the fake contact link")
	}

	sess.DirectoryErr = session.ErrNotConnected
	if _, err := svc.GetContactQRLink(ctx, inst.ID, false); !errors.Is(err, ErrNotConnected) {
		t.Errorf("offline err = %v, want ErrNotConnected", err)
	}
}

func TestCreateNewsletter(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()

	if _, err := svc.CreateNewsletter(ctx, inst.ID, "  ", "desc"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateNewsletter(ctx, inst.ID, strings.Repeat("x", 101), ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("long title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateNewsletter(ctx, inst.ID, "Canal", strings.Repeat("x", 501)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("long description err = %v, want ErrInvalidInput", err)
	}

	got, err := svc.CreateNewsletter(ctx, inst.ID, "Canal", "sobre")
	if err != nil {
		t.Fatalf("CreateNewsletter: %v", err)
	}
	if got.Title != "Canal" || got.ChannelJID == "" {
		t.Errorf("newsletter = %+v, want the created channel", got)
	}

	if _, err := svc.CreateNewsletter(ctx, inst.ID, "Canal", "outra"); !errors.Is(err, ErrForbidden) {
		t.Errorf("duplicate title err = %v, want ErrForbidden", err)
	}
}

func TestMuteNewsletter(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	channel := "12036300000001@newsletter"
	sess.PutNewsletter(session.NewsletterInfo{ChannelJID: channel, Title: "Canal"})

	if err := svc.MuteNewsletter(ctx, inst.ID, "  ", true); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank channel err = %v, want ErrInvalidInput", err)
	}
	if err := svc.MuteNewsletter(ctx, inst.ID, channel, true); err != nil {
		t.Fatalf("MuteNewsletter: %v", err)
	}
	if err := svc.MuteNewsletter(ctx, inst.ID, "99999@newsletter", true); !errors.Is(err, ErrNewsletterNotFound) {
		t.Errorf("unknown channel err = %v, want ErrNewsletterNotFound", err)
	}
}

func TestMarkNewsletterViewed(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	channel := "12036300000002@newsletter"
	sess.PutNewsletter(session.NewsletterInfo{ChannelJID: channel, Title: "Canal"})

	many := make([]string, 101)
	for i := range many {
		many[i] = "1"
	}
	if err := svc.MarkNewsletterViewed(ctx, inst.ID, channel, many); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("oversize batch err = %v, want ErrInvalidInput", err)
	}
	if err := svc.MarkNewsletterViewed(ctx, inst.ID, channel, []string{"1", "2"}); err != nil {
		t.Fatalf("MarkNewsletterViewed: %v", err)
	}
	if err := svc.MarkNewsletterViewed(ctx, inst.ID, "99999@newsletter", []string{"1"}); !errors.Is(err, ErrNewsletterNotFound) {
		t.Errorf("unknown channel err = %v, want ErrNewsletterNotFound", err)
	}
}

func TestReactNewsletter(t *testing.T) {
	inst, repo := newParityTestInstance(t)
	mgr := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	mgr.Put(inst.ID, sess)
	svc := NewService(repo, mgr, nil, nil, nil, testLogger())
	ctx := context.Background()
	channel := "12036300000003@newsletter"
	sess.PutNewsletter(session.NewsletterInfo{ChannelJID: channel, Title: "Canal"})

	if err := svc.ReactNewsletter(ctx, inst.ID, channel, "", "👍"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty serverID err = %v, want ErrInvalidInput", err)
	}
	// An empty reaction removes the reaction: it must reach the session.
	if err := svc.ReactNewsletter(ctx, inst.ID, channel, "42", ""); err != nil {
		t.Fatalf("ReactNewsletter remove: %v", err)
	}
	if err := svc.ReactNewsletter(ctx, inst.ID, channel, "42", "👍"); err != nil {
		t.Fatalf("ReactNewsletter: %v", err)
	}
	if err := svc.ReactNewsletter(ctx, inst.ID, "99999@newsletter", "42", "👍"); !errors.Is(err, ErrNewsletterNotFound) {
		t.Errorf("unknown channel err = %v, want ErrNewsletterNotFound", err)
	}
}
