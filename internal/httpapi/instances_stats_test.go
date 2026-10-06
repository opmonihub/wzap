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

// statsTotals decodes the scoped counts answered by GET /instances/stats
// (data.stats carries total plus the by_status buckets).
func statsTotals(t *testing.T, body []byte) (int, map[string]int) {
	t.Helper()
	var payload struct {
		Data struct {
			Stats struct {
				Total    int            `json:"total"`
				ByStatus map[string]int `json:"by_status"`
			} `json:"stats"`
		} `json:"data"`
	}
	decodeJSON(t, body, &payload)
	return payload.Data.Stats.Total, payload.Data.Stats.ByStatus
}

func TestInstanceStatsGlobalCountsAllStatuses(t *testing.T) {
	owner := uuid.New()
	rows := []model.Instance{
		{ID: uuid.New(), Name: "a", OwnerUserID: &owner, Connection: model.InstanceConnection{Status: "connected"}},
		{ID: uuid.New(), Name: "b", Connection: model.InstanceConnection{Status: "disconnected"}},
		{ID: uuid.New(), Name: "c", OwnerUserID: &owner, Connection: model.InstanceConnection{Status: "pairing"}},
		{ID: uuid.New(), Name: "d", Connection: model.InstanceConnection{Status: "error"}},
	}
	svc := &fakeInstanceService{listFn: func(context.Context) ([]model.Instance, error) {
		return rows, nil
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
	f.instA.Connection.Status = "connected"
	f.instB.Connection.Status = "pairing"
	f.legacy.Connection.Status = "error"
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
	f.instA.Connection.Status = "connected"
	f.instB.Connection.Status = "pairing"
	f.legacy.Connection.Status = "error"
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
	rows := []model.Instance{{ID: uuid.New(), Name: "a", Connection: model.InstanceConnection{Status: "mysterious"}}}
	svc := &fakeInstanceService{listFn: func(context.Context) ([]model.Instance, error) {
		return rows, nil
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
	svc := &fakeInstanceService{listFn: func(context.Context) ([]model.Instance, error) {
		return nil, errors.New("boom")
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

func TestInstanceStatsCountsCompleteCollection(t *testing.T) {
	rows := make([]model.Instance, 0, 127)
	for range 125 {
		rows = append(rows, model.Instance{ID: uuid.New(), Connection: model.InstanceConnection{Status: "connected"}})
	}
	rows = append(rows, model.Instance{ID: uuid.New(), Connection: model.InstanceConnection{Status: "pairing"}}, model.Instance{ID: uuid.New(), Connection: model.InstanceConnection{Status: "unknown"}})
	svc := &fakeInstanceService{listFn: func(context.Context) ([]model.Instance, error) {
		return rows, nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/stats", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	total, byStatus := statsTotals(t, rec.Body.Bytes())
	if total != 127 {
		t.Errorf("data.total = %d, want 127", total)
	}
	want := map[string]int{"connected": 125, "disconnected": 1, "pairing": 1, "error": 0}
	if !reflect.DeepEqual(byStatus, want) {
		t.Errorf("data.by_status = %v, want %v", byStatus, want)
	}
	if svc.listCalls != 1 {
		t.Errorf("List calls = %d, want 1", svc.listCalls)
	}
}
