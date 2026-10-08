package instance

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
)

// blockedReadRepo captures the first snapshot before letting another update
// start. The mutex protects the storage fake; it does not serialize updates.
type blockedReadRepo struct {
	*fakeRepo
	mu        sync.Mutex
	firstRead chan struct{}
	release   chan struct{}
	reads     int
}

func (r *blockedReadRepo) Get(ctx context.Context, id uuid.UUID) (*model.Instance, error) {
	r.mu.Lock()
	inst, err := r.fakeRepo.Get(ctx, id)
	r.reads++
	first := r.reads == 1
	r.mu.Unlock()
	// A second update announces its attempt here without the service lock;
	// with the lock it announces while acquiring, before it can read storage.
	_ = ctx.Done()
	if first {
		close(r.firstRead)
		select {
		case <-r.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return inst, err
}

func (r *blockedReadRepo) UpdateIdentity(ctx context.Context, id uuid.UUID, name, ref string) (*model.Instance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeRepo.UpdateIdentity(ctx, id, name, ref)
}

func (r *blockedReadRepo) SetWebhook(ctx context.Context, id uuid.UUID, url *string, enabled bool, events []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeRepo.SetWebhook(ctx, id, url, enabled, events)
}

type updateAttemptContext struct {
	context.Context
	attempted chan struct{}
	once      sync.Once
}

func (c *updateAttemptContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.attempted) })
	return c.Context.Done()
}

func TestServiceConcurrentPartialUpdatesPreserveBothPatches(t *testing.T) {
	name, ref, oldURL, newURL := "renamed", "new-ref", "https://old.example", "https://new.example"
	enabled := false
	events := []string{"receipt"}
	for _, tc := range []struct {
		name          string
		first, second UpdateInput
		check         func(*testing.T, model.Instance)
	}{
		{
			name:  "identity",
			first: UpdateInput{Name: &name}, second: UpdateInput{ExternalRef: &ref},
			check: func(t *testing.T, got model.Instance) {
				if got.Name != "renamed" || got.ExternalRef != "new-ref" {
					t.Errorf("identity = %q, %q; a patch restored an omitted field", got.Name, got.ExternalRef)
				}
			},
		},
		{
			name:  "webhook",
			first: UpdateInput{WebhookEnabled: &enabled, WebhookEvents: &events}, second: UpdateInput{WebhookURL: &newURL},
			check: func(t *testing.T, got model.Instance) {
				if got.Webhook.URL == nil || *got.Webhook.URL != "https://new.example" || got.Webhook.IsEnabled || len(got.Webhook.Events) != 1 || got.Webhook.Events[0] != "receipt" {
					t.Errorf("webhook = %+v; a patch restored an omitted field", got.Webhook)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			repo := &blockedReadRepo{
				fakeRepo:  newFakeRepo(model.Instance{ID: id, Name: "original", ExternalRef: "old-ref", Webhook: model.InstanceWebhook{URL: &oldURL, IsEnabled: true, Events: []string{"message"}}}),
				firstRead: make(chan struct{}), release: make(chan struct{}),
			}
			svc := NewService(repo, nil, nil, nil, nil, zerolog.Nop())
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			firstDone, secondDone := make(chan error, 1), make(chan error, 1)
			go func() { _, err := svc.Update(ctx, id, tc.first); firstDone <- err }()
			<-repo.firstRead
			secondCtx := &updateAttemptContext{Context: ctx, attempted: make(chan struct{})}
			go func() { _, err := svc.Update(secondCtx, id, tc.second); secondDone <- err }()
			<-secondCtx.attempted
			close(repo.release)
			if err := <-firstDone; err != nil {
				t.Fatal(err)
			}
			if err := <-secondDone; err != nil {
				t.Fatal(err)
			}
			tc.check(t, repo.instances[id])
		})
	}
}

func TestServiceWebhookOnlyUpdateDoesNotWriteIdentity(t *testing.T) {
	id := uuid.New()
	repo := newFakeRepo(model.Instance{ID: id, Name: "original", ExternalRef: "existing-ref"})
	// An identity write would be an unrelated failure and can overwrite a
	// rename from another satellite writer.
	repo.updateErr = errors.New("identity writer unavailable")
	url := "https://new.example"
	got, err := NewService(repo, nil, nil, nil, nil, zerolog.Nop()).Update(context.Background(), id, UpdateInput{WebhookURL: &url})
	if err != nil {
		t.Fatalf("webhook-only update: %v", err)
	}
	if got.Name != "original" || got.ExternalRef != "existing-ref" || got.Webhook.URL == nil || *got.Webhook.URL != url {
		t.Errorf("updated instance = %+v", got)
	}
}

func TestServiceUpdateWaitingCancellationDoesNotWrite(t *testing.T) {
	id := uuid.New()
	repo := &blockedReadRepo{fakeRepo: newFakeRepo(model.Instance{ID: id, Name: "original"}), firstRead: make(chan struct{}), release: make(chan struct{})}
	svc := NewService(repo, nil, nil, nil, nil, zerolog.Nop())
	firstName, cancelledName := "accepted", "cancelled"
	firstDone := make(chan error, 1)
	go func() {
		_, err := svc.Update(context.Background(), id, UpdateInput{Name: &firstName})
		firstDone <- err
	}()
	<-repo.firstRead
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.Update(ctx, id, UpdateInput{Name: &cancelledName})
	close(repo.release)
	if firstErr := <-firstDone; firstErr != nil {
		t.Fatal(firstErr)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("waiting update error = %v, want canceled", err)
	}
	if got := repo.instances[id].Name; got != "accepted" {
		t.Errorf("stored name = %q, want accepted", got)
	}
}

func TestServiceUpdateReleasesLockAfterFailure(t *testing.T) {
	for _, scenario := range []string{"not-found", "invalid-name", "identity-write", "webhook-write"} {
		t.Run(scenario, func(t *testing.T) {
			id := uuid.New()
			repo := newFakeRepo(model.Instance{ID: id, Name: "original"})
			svc := NewService(repo, nil, nil, nil, nil, zerolog.Nop())
			name, url := "requested", "https://new.example"
			patch := UpdateInput{Name: &name}
			switch scenario {
			case "not-found":
				delete(repo.instances, id)
			case "invalid-name":
				name = "stats"
			case "identity-write":
				repo.updateErr = errors.New("identity write failed")
			case "webhook-write":
				repo.setWebhookErr = errors.New("webhook write failed")
				patch = UpdateInput{WebhookURL: &url}
			}
			if _, err := svc.Update(context.Background(), id, patch); err == nil {
				t.Fatal("failure fixture unexpectedly succeeded")
			}
			repo.instances[id] = model.Instance{ID: id, Name: "original"}
			repo.updateErr, repo.setWebhookErr = nil, nil
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			name = "accepted"
			got, err := svc.Update(ctx, id, UpdateInput{Name: &name})
			if err != nil || got.Name != "accepted" {
				t.Fatalf("retry after failure = %+v, %v", got, err)
			}
		})
	}
}
