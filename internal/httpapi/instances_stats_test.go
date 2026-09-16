package httpapi

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/model"
)

// statsTotals decodes the scoped counts answered by GET /instances/stats.
func statsTotals(t *testing.T, body []byte) (int, map[string]int) {
	t.Helper()
	var payload struct {
		Data struct {
			Total    int            `json:"total"`
			ByStatus map[string]int `json:"by_status"`
		} `json:"data"`
	}
	decodeJSON(t, body, &payload)
	return payload.Data.Total, payload.Data.ByStatus
}

func TestInstanceStatsGlobalCountsAllStatuses(t *testing.T) {
	owner := uuid.New()
	rows := []model.Instance{
		{ID: uuid.New(), Name: "a", Status: "connected", OwnerUserID: &owner},
		{ID: uuid.New(), Name: "b", Status: "disconnected"},
		{ID: uuid.New(), Name: "c", Status: "pairing", OwnerUserID: &owner},
		{ID: uuid.New(), Name: "d", Status: "error"},
	}
	svc := &fakeInstanceService{listFn: func(context.Context, int, string) ([]model.Instance, string, error) {
		return rows, "", nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/stats", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	total, byStatus := statsTotals(t, rec.Body.Bytes())
	if total != 4 {
		t.Errorf("data.total = %d, want 4", total)
	}
	want := map[string]int{"connected": 1, "disconnected": 1, "pairing": 1, "error": 1}
	if !reflect.DeepEqual(byStatus, want) {
		t.Errorf("data.by_status = %v, want %v", byStatus, want)
	}
}

func TestInstanceStatsAdminCountsAll(t *testing.T) {
	f := newRBACFixture(t)
	f.instA.Status = "connected"
	f.instB.Status = "pairing"
	f.legacy.Status = "error"
	srv := f.rbacServer(t)

	rec := serveRBAC(t, srv, http.MethodGet, "/instances/stats", "", rbacSessionCookie(f.adminTok), "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	total, byStatus := statsTotals(t, rec.Body.Bytes())
	if total != 3 {
		t.Errorf("data.total = %d, want 3", total)
	}
	want := map[string]int{"connected": 1, "disconnected": 0, "pairing": 1, "error": 1}
	if !reflect.DeepEqual(byStatus, want) {
		t.Errorf("data.by_status = %v, want %v", byStatus, want)
	}
}

func TestInstanceStatsUserCountsOwnOnly(t *testing.T) {
	f := newRBACFixture(t)
	f.instA.Status = "connected"
	f.instB.Status = "pairing"
	f.legacy.Status = "error"
	srv := f.rbacServer(t)

	rec := serveRBAC(t, srv, http.MethodGet, "/instances/stats", "", rbacSessionCookie(f.userATok), "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	total, byStatus := statsTotals(t, rec.Body.Bytes())
	if total != 1 {
		t.Errorf("data.total = %d, want 1 (only the own instance, legacy NULL-owner excluded)", total)
	}
	want := map[string]int{"connected": 1, "disconnected": 0, "pairing": 0, "error": 0}
	if !reflect.DeepEqual(byStatus, want) {
		t.Errorf("data.by_status = %v, want %v", byStatus, want)
	}
}

func TestInstanceStatsInstanceKeyForbidden(t *testing.T) {
	f := newRBACFixture(t)
	srv := f.rbacServer(t)

	rec := serveRBAC(t, srv, http.MethodGet, "/instances/stats", "", nil, f.keyA, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "forbidden" {
		t.Errorf("error code = %q, want %q", code, "forbidden")
	}
}

func TestInstanceStatsUnknownStatusFoldsIntoDisconnected(t *testing.T) {
	rows := []model.Instance{{ID: uuid.New(), Name: "a", Status: "mysterious"}}
	svc := &fakeInstanceService{listFn: func(context.Context, int, string) ([]model.Instance, string, error) {
		return rows, "", nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/stats", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	total, byStatus := statsTotals(t, rec.Body.Bytes())
	if total != 1 {
		t.Errorf("data.total = %d, want 1", total)
	}
	want := map[string]int{"connected": 0, "disconnected": 1, "pairing": 0, "error": 0}
	if !reflect.DeepEqual(byStatus, want) {
		t.Errorf("data.by_status = %v, want %v", byStatus, want)
	}
}

func TestInstanceStatsServiceErrorIsInternal(t *testing.T) {
	svc := &fakeInstanceService{listFn: func(context.Context, int, string) ([]model.Instance, string, error) {
		return nil, "", errors.New("boom")
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/stats", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "internal_error" {
		t.Errorf("error code = %q, want %q", code, "internal_error")
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("body %q leaks the service cause", rec.Body.String())
	}
}

func TestInstanceStatsAccumulatesPages(t *testing.T) {
	first := []model.Instance{
		{ID: uuid.New(), Name: "a", Status: "connected"},
		{ID: uuid.New(), Name: "b", Status: "disconnected"},
	}
	second := []model.Instance{
		{ID: uuid.New(), Name: "c", Status: "pairing"},
	}
	var limits []int
	var cursors []string
	svc := &fakeInstanceService{listFn: func(_ context.Context, limit int, cursor string) ([]model.Instance, string, error) {
		limits = append(limits, limit)
		cursors = append(cursors, cursor)
		if cursor == "" {
			return first, "cursor-1", nil
		}
		if cursor == "cursor-1" {
			return second, "", nil
		}
		return nil, "", errors.New("unexpected cursor " + cursor)
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/stats", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	total, byStatus := statsTotals(t, rec.Body.Bytes())
	if total != 3 {
		t.Errorf("data.total = %d, want 3 across both pages", total)
	}
	want := map[string]int{"connected": 1, "disconnected": 1, "pairing": 1, "error": 0}
	if !reflect.DeepEqual(byStatus, want) {
		t.Errorf("data.by_status = %v, want %v", byStatus, want)
	}
	// The handler pages at the max page size (100, maxInstancesLimit).
	if !reflect.DeepEqual(limits, []int{100, 100}) {
		t.Errorf("List limits = %v, want [100 100]", limits)
	}
	if !reflect.DeepEqual(cursors, []string{"", "cursor-1"}) {
		t.Errorf("List cursors = %q, want [\"\" \"cursor-1\"]", cursors)
	}
}
