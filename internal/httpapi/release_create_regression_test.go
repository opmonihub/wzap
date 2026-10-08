package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/instance"
	"wzap/internal/model"
)

func TestReleaseCreateAdminSessionDefaultsToSelf(t *testing.T) {
	oldest, creator := uuid.New(), uuid.New()
	svc := &fakeInstanceService{oldestAdminFn: func(context.Context) (uuid.UUID, error) { return oldest, nil }, createFn: echoCreateFn("admin-key")}
	srv := createOwnerTestServer(t, svc, &model.User{ID: oldest, Role: "admin", CreatedAt: time.Unix(1, 0)}, &model.User{ID: creator, Role: "admin", CreatedAt: time.Unix(2, 0)})
	cookie := rbacSessionCookie(mustSessionToken(t, creator, "admin"))
	rec := serveRBAC(t, srv, http.MethodPost, "/instances", `{"name":"admin-owned"}`, cookie, "", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(svc.createInputs) != 1 || svc.createInputs[0].OwnerUserID == nil || *svc.createInputs[0].OwnerUserID != creator {
		t.Fatalf("created owner=%+v; want session admin %s", svc.createInputs, creator)
	}
}

// releaseCreationStore keeps count queries and persisted creations on the same
// synchronized state. The first persist pauses after quota evaluation, exposing
// the lost reservation if another HTTP request can evaluate quotas meanwhile.
type releaseCreationStore struct {
	*fakeInstanceService
	*fakeAPIKeyRepository
	mu          sync.Mutex
	rows        []model.Instance
	queries     int
	firstCreate chan struct{}
	secondQuery chan struct{}
	release     chan struct{}
	once        sync.Once
}

func (s *releaseCreationStore) count(owner *uuid.UUID) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, row := range s.rows {
		if owner == nil || *row.OwnerUserID == *owner {
			count++
		}
	}
	s.queries++
	if s.queries == 2 {
		close(s.secondQuery)
	}
	return count, nil
}
func (s *releaseCreationStore) CountAll(context.Context) (int, error) { return s.count(nil) }
func (s *releaseCreationStore) CountByOwner(_ context.Context, owner uuid.UUID) (int, error) {
	return s.count(&owner)
}
func (s *releaseCreationStore) Create(ctx context.Context, input instance.CreateInput) (*model.Instance, string, error) {
	s.once.Do(func() { close(s.firstCreate) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	row := model.Instance{ID: uuid.New(), Name: input.Name, OwnerUserID: input.OwnerUserID, Connection: model.InstanceConnection{Status: "disconnected"}}
	s.mu.Lock()
	s.rows = append(s.rows, row)
	s.mu.Unlock()
	return &row, "quota-key", nil
}
func TestReleaseConcurrentCreateEnforcesQuotas(t *testing.T) {
	for _, tc := range []struct {
		name            string
		global          int
		perUser         int
		differentOwners bool
	}{
		{name: "per-user", perUser: 1},
		{name: "global different owners", global: 1, differentOwners: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, second := uuid.New(), uuid.New()
			if !tc.differentOwners {
				second = first
			}
			users := newFakeUserRepository(quotaUser(first, "first@example.com", "user", tc.perUser), quotaUser(second, "second@example.com", "user", tc.perUser))
			store := &releaseCreationStore{fakeInstanceService: &fakeInstanceService{}, fakeAPIKeyRepository: &fakeAPIKeyRepository{}, firstCreate: make(chan struct{}), secondQuery: make(chan struct{}), release: make(chan struct{})}
			srv := httpapi.New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken, JWTSecret: testJWTSecret, MaxInstances: tc.global}, zerolog.Nop(), httpapi.Deps{ReadyChecker: checkFunc(func(context.Context) error { return nil }), Instances: store, Keys: store, Users: users, JWTSecret: testJWTSecret})
			cookies := []*http.Cookie{rbacSessionCookie(mustSessionToken(t, first, "user")), rbacSessionCookie(mustSessionToken(t, second, "user"))}
			results := make(chan *httptest.ResponseRecorder, 2)
			send := func(i int) {
				req := httptest.NewRequest(http.MethodPost, "/instances", strings.NewReader([]string{`{"name":"first"}`, `{"name":"second"}`}[i]))
				req.Header.Set("Content-Type", "application/json")
				req.AddCookie(cookies[i])
				rec := httptest.NewRecorder()
				srv.Handler.ServeHTTP(rec, req)
				results <- rec
			}
			go send(0)
			select {
			case <-store.firstCreate:
			case <-time.After(5 * time.Second):
				t.Fatal("first request did not reach persistence")
			}
			go send(1)
			// With serialized creation the second count waits for persistence. A bounded
			// wait lets that valid implementation proceed without a two-party barrier.
			select {
			case <-store.secondQuery:
			case <-time.After(time.Second):
			}
			close(store.release)
			statuses := map[int]int{}
			for range 2 {
				select {
				case rec := <-results:
					statuses[rec.Code]++
					if rec.Code == http.StatusForbidden && errorCode(t, rec.Body.Bytes()) != "quota_exceeded" {
						t.Errorf("unexpected denial: %s", rec.Body.String())
					}
				case <-time.After(5 * time.Second):
					t.Fatal("create request did not complete")
				}
			}
			if statuses[http.StatusCreated] != 1 || statuses[http.StatusForbidden] != 1 {
				t.Fatalf("statuses=%v; want one 201 and one 403", statuses)
			}
			store.mu.Lock()
			count := len(store.rows)
			store.mu.Unlock()
			if count != 1 {
				t.Fatalf("persisted %d instances; quota is 1", count)
			}
		})
	}
}
