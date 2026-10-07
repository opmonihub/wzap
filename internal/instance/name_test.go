package instance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
	"wzap/internal/storage"
)

func TestServiceCreateRejectsInvalidNameBeforeDependencies(t *testing.T) {
	for _, name := range []string{"", " has-space", "has-space ", "a/b", "a.b", "é", "_a", "a-", "stats", "550e8400-e29b-41d4-a716-446655440000", "550e8400e29b41d4a716446655440000", "urn:uuid:550e8400-e29b-41d4-a716-446655440000", "{550e8400-e29b-41d4-a716-446655440000}", strings.Repeat("a", 65)} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo()
			keys := newFakeKeyRepo()
			svc := NewService(repo, nil, nil, nil, keys, zerolog.Nop())
			_, key, err := svc.Create(context.Background(), CreateInput{Name: name, OwnerUserID: ownerPtr(uuid.New())})
			if !errors.Is(err, ErrInvalidInstanceName) {
				t.Fatalf("Create error = %v, want invalid instance name before dependencies", err)
			}
			if key != "" || len(repo.createCalls) != 0 || len(keys.hashes) != 0 {
				t.Fatal("invalid name wrote instance or issued key")
			}
		})
	}
}

func TestServiceUpdateNameValidation(t *testing.T) {
	for _, tc := range []struct {
		old, next string
		valid     bool
	}{
		{"good", "good", true},
		{"good", "stats", false}, {"good", " leading", false}, {"good", "name_", false},
	} {
		t.Run(tc.old+"/"+tc.next, func(t *testing.T) {
			inst := model.Instance{ID: uuid.New(), Name: tc.old}
			repo := newFakeRepo(inst)
			svc := NewService(repo, nil, nil, nil, nil, zerolog.Nop())
			external := "new-ref"
			got, err := svc.Update(context.Background(), inst.ID, UpdateInput{Name: &tc.next, ExternalRef: &external})
			if !tc.valid {
				if !errors.Is(err, ErrInvalidInstanceName) {
					t.Fatalf("Update error = %v, want invalid instance name", err)
				}
				if len(repo.updateCalls) != 0 {
					t.Fatal("invalid rename persisted")
				}
				return
			}
			if err != nil || got.Name != tc.next || got.ExternalRef != external {
				t.Fatalf("Update = %+v, %v", got, err)
			}
		})
	}
}

// TestServiceUpdateRevalidatesUnchangedName pins that the unchanged-name skip
// is gone: every update input is validated, even when it equals the stored
// name. Legacy rows with names outside the current grammar can no longer
// exist, so an unchanged name is always valid — but the revalidation must run
// (an invalid input is rejected instead of silently accepted).
func TestServiceUpdateRevalidatesUnchangedName(t *testing.T) {
	inst := model.Instance{ID: uuid.New(), Name: "bad name"}
	repo := newFakeRepo(inst)
	svc := NewService(repo, nil, nil, nil, nil, zerolog.Nop())
	same := inst.Name
	if _, err := svc.Update(context.Background(), inst.ID, UpdateInput{Name: &same}); !errors.Is(err, ErrInvalidInstanceName) {
		t.Fatalf("Update(unchanged invalid name) = %v, want invalid instance name", err)
	}
	if len(repo.updateCalls) != 0 {
		t.Fatal("revalidated unchanged name persisted")
	}
}

func TestValidateInstanceName(t *testing.T) {
	for _, name := range []string{"a", "Z", "1", "Loja_SP-1", "Stats", "STATS", strings.Repeat("a", 64)} {
		if err := ValidateInstanceName(name); err != nil {
			t.Errorf("Validate(%q): %v", name, err)
		}
	}
	for _, name := range []string{"", "stats", "a_", "-a", "é", "a b", strings.Repeat("a", 65), "550e8400e29b41d4a716446655440000"} {
		if !errors.Is(ValidateInstanceName(name), ErrInvalidInstanceName) {
			t.Errorf("Validate(%q) should reject", name)
		}
	}
}

func TestServiceNameErrorMapping(t *testing.T) {
	for _, tc := range []struct{ stored, domain error }{
		{storage.ErrInvalidInstanceName, ErrInvalidInstanceName},
		{storage.ErrInstanceNameTaken, ErrInstanceNameTaken},
	} {
		if !errors.Is(mapError("get", tc.stored), tc.domain) {
			t.Errorf("mapping %v", tc.stored)
		}
	}
	owner := model.User{ID: uuid.New()}
	repo := newFakeRepo()
	repo.createErr = storage.ErrInstanceNameTaken
	keys := newFakeKeyRepo()
	svc := NewService(repo, nil, nil, newFakeUserRepo(owner), keys, zerolog.Nop())
	_, key, err := svc.Create(context.Background(), CreateInput{Name: "valid", OwnerUserID: &owner.ID})
	if !errors.Is(err, ErrInstanceNameTaken) || key != "" || len(keys.hashes) != 0 {
		t.Fatalf("taken Create = %q, %v", key, err)
	}
	inst := model.Instance{ID: uuid.New(), Name: "original"}
	repo.instances[inst.ID] = inst
	repo.updateErr = storage.ErrInstanceNameTaken
	next := "claimed"
	if _, err := svc.Update(context.Background(), inst.ID, UpdateInput{Name: &next}); !errors.Is(err, ErrInstanceNameTaken) {
		t.Fatalf("taken Update: %v", err)
	}
}

func TestServiceGetByName(t *testing.T) {
	inst := model.Instance{ID: uuid.New(), Name: "Exact_Name"}
	repo := newFakeRepo(inst)
	svc := NewService(repo, nil, nil, nil, nil, zerolog.Nop())
	got, err := svc.GetByName(context.Background(), inst.Name)
	if err != nil || got.ID != inst.ID {
		t.Fatalf("GetByName = %+v, %v", got, err)
	}
	if _, err := svc.GetByName(context.Background(), "exact_name"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing name: %v", err)
	}
}
