package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/httpapi/representation"
	"wzap/internal/model"
)

// The listing must not truncate an authorized collection or interpret removed
// pagination inputs. The same ordered collection backs every scope.
func TestInstancesListCompleteCollection(t *testing.T) {
	f := newRBACFixture(t)
	rows := make([]model.Instance, 0, 127)
	for range 125 {
		rows = append(rows, model.Instance{ID: uuid.New(), Name: "owned", OwnerUserID: &f.userA})
	}
	rows = append(rows, *f.instB, *f.legacy)
	svc := &fakeInstanceService{listFn: func(context.Context) ([]model.Instance, error) {
		return rows, nil
	}}
	srv := httpapi.New(config.Config{APIKey: f.globalKey, JWTSecret: testJWTSecret}, zerolog.Nop(), httpapi.Deps{
		Instances: svc, Users: f.rbacUsers(), Keys: f.keys, JWTSecret: testJWTSecret,
	})
	tests := []struct {
		name   string
		cookie *http.Cookie
		key    string
		want   int
		status int
	}{
		{name: "global", key: f.globalKey, want: 127, status: http.StatusOK},
		{name: "admin", cookie: rbacSessionCookie(f.adminTok), want: 127, status: http.StatusOK},
		{name: "owner", cookie: rbacSessionCookie(f.userATok), want: 125, status: http.StatusOK},
		{name: "other owner", cookie: rbacSessionCookie(f.userBTok), want: 1, status: http.StatusOK},
		{name: "instance key", key: f.keyA, status: http.StatusForbidden},
		{name: "unauthenticated", status: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, query := range []string{"", "?limit=1", "?limit=abc&cursor=not-a-uuid", "?limit=-1&cursor=obsolete"} {
				t.Run(query, func(t *testing.T) {
					rec := serveRBAC(t, srv, http.MethodGet, "/instances"+query, "", tt.cookie, tt.key, nil)
					if rec.Code != tt.status {
						t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.status, rec.Body.String())
					}
					if rec.Header().Get("X-Request-Id") == "" {
						t.Error("missing response X-Request-Id")
					}
					if tt.status != http.StatusOK {
						return
					}
					var payload struct {
						Data map[string]json.RawMessage `json:"data"`
					}
					decodeJSON(t, rec.Body.Bytes(), &payload)
					if _, ok := payload.Data["next_cursor"]; ok {
						t.Error("data.next_cursor must be absent")
					}
					var items []representation.InstanceResponse
					decodeJSON(t, payload.Data["instances"], &items)
					if len(items) != tt.want {
						t.Fatalf("data.items length = %d, want %d", len(items), tt.want)
					}
					if tt.name == "other owner" {
						if items[0].ID != f.instB.ID.String() {
							t.Errorf("other owner's item = %s, want %s", items[0].ID, f.instB.ID)
						}
						return
					}
					for i, item := range items {
						if item.ID != rows[i].ID.String() {
							t.Errorf("item %d = %s, want %s (preserve repository order)", i, item.ID, rows[i].ID)
						}
					}
					if tt.name == "owner" {

						for i, item := range items {
							raw, _ := json.Marshal(item)
							if strings.Contains(string(raw), "owner_user_id") {
								t.Errorf("item %s leaks owner_user_id", item.ID)
							}
							if item.ID != rows[i].ID.String() {
								t.Errorf("owner item %d = %s, want %s (ownership filter order)", i, item.ID, rows[i].ID)
							}
						}
					}
				})
			}
		})
	}
}
func TestInstancesListEmptyCollection(t *testing.T) {
	rec := serveJSON(t, instancesServer(t, &fakeInstanceService{}), http.MethodGet, "/instances?limit=invalid&cursor=invalid", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var payload struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if string(payload.Data["instances"]) != "[]" {
		t.Errorf("data.items = %s, want []", payload.Data["instances"])
	}
	if _, ok := payload.Data["next_cursor"]; ok {
		t.Error("data.next_cursor must be absent")
	}
}
