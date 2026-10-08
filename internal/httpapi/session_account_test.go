package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/model"
)

func TestSessionAccountAuthorization(t *testing.T) {
	const password = "session-account-password"
	const createBody = `{"email":"created@example.com","password":"new-user-password","role":"user"}`
	for _, state := range []string{"deleted", "demoted", "lookup failure"} {
		t.Run(state, func(t *testing.T) {
			admin := seedAuthUser(t, "session-admin@example.com", password, "admin")
			users := newFakeUserRepository(admin)
			srv := usersCRUDServer(t, users, &fakeAPIKeyRepository{}, 0)
			login := serveAuthForUsers(t, srv, `{"email":"session-admin@example.com","password":"session-account-password"}`)
			if login.Code != http.StatusOK {
				t.Fatalf("login status = %d, body %q", login.Code, login.Body.String())
			}
			cookie := sessionCookie(t, login)
			want := http.StatusUnauthorized
			switch state {
			case "deleted":
				removed := serveRBAC(t, srv, http.MethodDelete, "/users/"+admin.ID.String(), "", cookie, "", nil)
				if removed.Code != http.StatusNoContent {
					t.Fatalf("delete status = %d, body %q", removed.Code, removed.Body.String())
				}
			case "demoted":
				admin.Role = "user"
				want = http.StatusForbidden
			case "lookup failure":
				users.err = errors.New("synthetic repository failure")
				want = http.StatusInternalServerError
			}
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				rec := serveRBAC(t, srv, method, "/users", createBody, cookie, "", nil)
				if rec.Code != want {
					t.Errorf("%s status = %d, want %d, body %q", method, rec.Code, want, rec.Body.String())
				}
			}
			for _, user := range users.byID {
				if user.Email == "created@example.com" {
					t.Fatal("stale session created a user")
				}
			}
		})
	}
}

func TestSessionAccountLookupDoesNotBlockAPIKeys(t *testing.T) {
	f := newRBACFixture(t)
	users := newFakeUserRepository(&model.User{ID: f.admin, Role: "admin"})
	users.err = errors.New("synthetic repository failure")
	srv := httpapi.New(config.Config{APIKey: f.globalKey}, zerolog.Nop(), httpapi.Deps{
		Users: users, Instances: f.rbacInstances(), Keys: f.keys, JWTSecret: testJWTSecret,
	})
	for _, key := range []string{f.globalKey, f.keyA} {
		rec := serveRBAC(t, srv, http.MethodGet, "/instances/"+f.instA.ID.String(), "", nil, key, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("API key status = %d, want 200, body %q", rec.Code, rec.Body.String())
		}
	}
}

func TestSessionAccountLookupFailsBeforePrivateHandler(t *testing.T) {
	f := newRBACFixture(t)
	admin := seedAuthUser(t, "lookup-admin@example.com", "lookup-password", "admin")
	users := newFakeUserRepository(admin)
	calls := 0
	instances := &fakeInstanceService{getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
		calls++
		return f.instA, nil
	}}
	srv := httpapi.New(config.Config{APIKey: f.globalKey}, zerolog.Nop(), httpapi.Deps{
		Users: users, Instances: instances, Keys: f.keys, JWTSecret: testJWTSecret,
	})
	login := serveAuthForUsers(t, srv, `{"email":"lookup-admin@example.com","password":"lookup-password"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body %q", login.Code, login.Body.String())
	}
	users.err = errors.New("synthetic repository failure")
	rec := serveRBAC(t, srv, http.MethodGet, "/instances/"+f.instA.ID.String(), "", sessionCookie(t, login), "", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("lookup failure status = %d, want 500", rec.Code)
	}
	if calls != 0 {
		t.Errorf("instance reads after failed session lookup = %d, want 0", calls)
	}
}
