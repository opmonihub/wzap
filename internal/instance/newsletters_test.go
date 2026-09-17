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

func newsletterService(t *testing.T, sessions *sessiontest.Fake) (*Service, *model.Instance) {
	t.Helper()
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Status: "connected"}
	svc := NewService(newFakeRepo(*inst), sessions, &fakeMedia{}, nil, nil, zerolog.Nop())
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	return svc, inst
}

func seedNewsletter(t *testing.T, sessions *sessiontest.Fake, inst *model.Instance, channel, title string) {
	t.Helper()
	sess, _ := sessions.Get(inst.ID)
	sess.(*sessiontest.FakeSession).PutNewsletter(session.NewsletterInfo{ChannelJID: channel, Title: title})
}

func TestServiceNewsletterFollowUnfollow(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := newsletterService(t, sessions)
	ctx := context.Background()
	seedNewsletter(t, sessions, inst, "12345@newsletter", "Canal da loja")

	if err := svc.FollowNewsletter(ctx, inst.ID, "12345@newsletter"); err != nil {
		t.Fatalf("FollowNewsletter: %v", err)
	}
	if err := svc.UnfollowNewsletter(ctx, inst.ID, "12345@newsletter"); err != nil {
		t.Fatalf("UnfollowNewsletter: %v", err)
	}

	sess, _ := sessions.Get(inst.ID)
	calls := sess.(*sessiontest.FakeSession).NewsletterCalls()
	// Follow refreshes the cache from the live view, hence the get between
	// the follow and the unfollow.
	if len(calls) != 3 || calls[0].Op != "follow" || calls[1].Op != "get" || calls[2].Op != "unfollow" {
		t.Errorf("newsletter calls = %+v, want follow, refresh get, then unfollow", calls)
	}
}

func TestServiceNewsletterUnknownChannel(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := newsletterService(t, sessions)
	ctx := context.Background()

	if err := svc.FollowNewsletter(ctx, inst.ID, "99999@newsletter"); !errors.Is(err, ErrNewsletterNotFound) {
		t.Errorf("FollowNewsletter error = %v, want %v", err, ErrNewsletterNotFound)
	}
	if _, err := svc.GetNewsletter(ctx, inst.ID, "99999@newsletter"); !errors.Is(err, ErrNewsletterNotFound) {
		t.Errorf("GetNewsletter error = %v, want %v", err, ErrNewsletterNotFound)
	}
}

func TestServiceNewsletterGet(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := newsletterService(t, sessions)
	seedNewsletter(t, sessions, inst, "12345@newsletter", "Canal da loja")

	got, err := svc.GetNewsletter(context.Background(), inst.ID, "12345@newsletter")
	if err != nil {
		t.Fatalf("GetNewsletter: %v", err)
	}
	if got.Title != "Canal da loja" {
		t.Errorf("title = %q, want Canal da loja", got.Title)
	}
}

func TestServiceNewsletterListPages(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := newsletterService(t, sessions)
	ctx := context.Background()
	for _, ch := range []string{"111@newsletter", "222@newsletter", "333@newsletter"} {
		seedNewsletter(t, sessions, inst, ch, "Canal "+ch)
	}

	first, next, err := svc.ListNewsletters(ctx, inst.ID, 2, "")
	if err != nil {
		t.Fatalf("ListNewsletters: %v", err)
	}
	if len(first) != 2 || first[0].ChannelJID != "111@newsletter" || first[1].ChannelJID != "222@newsletter" {
		t.Fatalf("first page = %+v, want 111 then 222", first)
	}
	if next != "222@newsletter" {
		t.Fatalf("next cursor = %q, want 222@newsletter", next)
	}

	second, next, err := svc.ListNewsletters(ctx, inst.ID, 2, next)
	if err != nil {
		t.Fatalf("ListNewsletters second: %v", err)
	}
	if len(second) != 1 || second[0].ChannelJID != "333@newsletter" {
		t.Errorf("second page = %+v, want 333 only", second)
	}
	if next != "" {
		t.Errorf("next cursor = %q, want empty on the last page", next)
	}
}

func TestServiceNewsletterDisconnected(t *testing.T) {
	sessions := sessiontest.New(nil)
	svc, inst := newsletterService(t, sessions)
	sess, _ := sessions.Get(inst.ID)
	sess.(*sessiontest.FakeSession).NewsletterErr = session.ErrNotConnected

	if err := svc.FollowNewsletter(context.Background(), inst.ID, "12345@newsletter"); !errors.Is(err, ErrNotConnected) {
		t.Errorf("FollowNewsletter error = %v, want %v", err, ErrNotConnected)
	}
}
