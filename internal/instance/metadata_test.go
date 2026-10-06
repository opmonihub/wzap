package instance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/session/sessiontest"
	"wzap/internal/storage"
)

type fakeGroupMetadataRepo struct {
	rows map[string]model.GroupMetadata
	err  error
}

func (f *fakeGroupMetadataRepo) Upsert(_ context.Context, meta model.GroupMetadata) (model.GroupMetadata, error) {
	if f.err != nil {
		return model.GroupMetadata{}, f.err
	}
	meta.UpdatedAt = time.Now().UTC()
	if f.rows == nil {
		f.rows = make(map[string]model.GroupMetadata)
	}
	f.rows[meta.GroupJID] = meta
	return meta, nil
}

func (f *fakeGroupMetadataRepo) Get(_ context.Context, _ uuid.UUID, groupJID string) (model.GroupMetadata, error) {
	meta, ok := f.rows[groupJID]
	if !ok {
		return model.GroupMetadata{}, storage.ErrNotFound
	}
	return meta, nil
}

func (f *fakeGroupMetadataRepo) DeleteByInstance(_ context.Context, _ uuid.UUID) (int64, error) {
	n := int64(len(f.rows))
	f.rows = nil
	return n, nil
}

type fakeNewsletterMetadataRepo struct {
	rows map[string]model.NewsletterMetadata
	err  error
}

func (f *fakeNewsletterMetadataRepo) Upsert(_ context.Context, meta model.NewsletterMetadata) (model.NewsletterMetadata, error) {
	if f.err != nil {
		return model.NewsletterMetadata{}, f.err
	}
	meta.UpdatedAt = time.Now().UTC()
	if f.rows == nil {
		f.rows = make(map[string]model.NewsletterMetadata)
	}
	f.rows[meta.ChannelJID] = meta
	return meta, nil
}

func (f *fakeNewsletterMetadataRepo) Get(_ context.Context, _ uuid.UUID, channelJID string) (model.NewsletterMetadata, error) {
	meta, ok := f.rows[channelJID]
	if !ok {
		return model.NewsletterMetadata{}, storage.ErrNotFound
	}
	return meta, nil
}

func (f *fakeNewsletterMetadataRepo) ListByInstance(_ context.Context, _ uuid.UUID) ([]model.NewsletterMetadata, error) {
	out := make([]model.NewsletterMetadata, 0, len(f.rows))
	for _, meta := range f.rows {
		out = append(out, meta)
	}
	return out, nil
}

func (f *fakeNewsletterMetadataRepo) DeleteByInstance(_ context.Context, _ uuid.UUID) (int64, error) {
	n := int64(len(f.rows))
	f.rows = nil
	return n, nil
}

func metadataService(t *testing.T) (*Service, *model.Instance, *sessiontest.Fake, *fakeGroupMetadataRepo, *fakeNewsletterMetadataRepo) {
	t.Helper()
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Connection: model.InstanceConnection{Status: "connected"}}
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	groups := &fakeGroupMetadataRepo{}
	newsletters := &fakeNewsletterMetadataRepo{}
	svc := NewService(newFakeRepo(*inst), sessions, &fakeMedia{}, nil, nil, zerolog.Nop(),
		WithMetadataStores(groups, newsletters))
	return svc, inst, sessions, groups, newsletters
}

func TestServiceGetGroupRefreshesCache(t *testing.T) {
	svc, inst, sessions, groups, _ := metadataService(t)
	sess, _ := sessions.Get(inst.ID)
	fake := sess.(*sessiontest.FakeSession)
	code := fake.PutGroup(session.GroupInfo{
		JID:  "120363000000000000@g.us",
		Name: "Time do churrasco",
	})
	_ = code

	got, err := svc.GetGroup(context.Background(), inst.ID, "120363000000000000@g.us")
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("updated_at is zero, want the cache refresh instant")
	}
	cached, err := groups.Get(context.Background(), inst.ID, "120363000000000000@g.us")
	if err != nil {
		t.Fatalf("cached metadata: %v", err)
	}
	if cached.Name != "Time do churrasco" {
		t.Errorf("cached name = %q, want the live subject", cached.Name)
	}
}

func TestServiceGetGroupWithoutCacheLeavesUpdatedAtZero(t *testing.T) {
	inst := &model.Instance{ID: uuid.New(), Name: "loja", Connection: model.InstanceConnection{Status: "connected"}}
	sessions := sessiontest.New(nil)
	sess := sessiontest.NewSession(inst.ID, nil)
	sessions.Put(inst.ID, sess)
	sess.PutGroup(session.GroupInfo{JID: "120363000000000000@g.us", Name: "Time"})
	svc := NewService(newFakeRepo(*inst), sessions, &fakeMedia{}, nil, nil, zerolog.Nop())

	got, err := svc.GetGroup(context.Background(), inst.ID, "120363000000000000@g.us")
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if !got.UpdatedAt.IsZero() {
		t.Errorf("updated_at = %v, want zero without a wired store", got.UpdatedAt)
	}
}

func TestServiceGroupCacheFailureKeepsLiveView(t *testing.T) {
	svc, inst, sessions, _, _ := metadataService(t)
	sess, _ := sessions.Get(inst.ID)
	sess.(*sessiontest.FakeSession).PutGroup(session.GroupInfo{JID: "120363000000000000@g.us", Name: "Time"})
	broken := &fakeGroupMetadataRepo{err: errors.New("cache down")}
	brokenSvc := NewService(newFakeRepo(*inst), sessions, &fakeMedia{}, nil, nil, zerolog.Nop(),
		WithMetadataStores(broken, nil))
	_ = svc

	got, err := brokenSvc.GetGroup(context.Background(), inst.ID, "120363000000000000@g.us")
	if err != nil {
		t.Fatalf("GetGroup: %v, want the live view despite the cache failure", err)
	}
	if got.Name != "Time" {
		t.Errorf("name = %q, want the live subject", got.Name)
	}
}

func TestServiceNewsletterReadsRefreshCache(t *testing.T) {
	svc, inst, sessions, _, newsletters := metadataService(t)
	sess, _ := sessions.Get(inst.ID)
	sess.(*sessiontest.FakeSession).PutNewsletter(session.NewsletterInfo{ChannelJID: "12345@newsletter", Title: "Canal"})
	ctx := context.Background()

	got, err := svc.GetNewsletter(ctx, inst.ID, "12345@newsletter")
	if err != nil {
		t.Fatalf("GetNewsletter: %v", err)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("updated_at is zero, want the cache refresh instant")
	}

	items, _, err := svc.ListNewsletters(ctx, inst.ID, 50, "")
	if err != nil {
		t.Fatalf("ListNewsletters: %v", err)
	}
	// The list refreshes every row again, so its stamp is newer than the get
	// above: both must simply be non-zero refreshes of the same channel.
	if len(items) != 1 || items[0].ChannelJID != "12345@newsletter" || items[0].UpdatedAt.IsZero() {
		t.Errorf("listed = %+v, want the refreshed channel", items)
	}

	cached, err := newsletters.Get(ctx, inst.ID, "12345@newsletter")
	if err != nil {
		t.Fatalf("cached metadata: %v", err)
	}
	if cached.Title != "Canal" {
		t.Errorf("cached title = %q, want the live title", cached.Title)
	}
}
