package httpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"wzap/internal/storage"
)

type fakeAPIKeyRepository struct {
	byHash map[string]uuid.UUID
	err    error
}

const testToken = "test-service-token"

func decodeJSON(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
}
func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, body, &payload)
	return payload.Error.Code
}
func (f *fakeAPIKeyRepository) SetHash(_ context.Context, instanceID uuid.UUID, hash string) error {
	if f.err != nil {
		return f.err
	}
	if f.byHash == nil {
		f.byHash = map[string]uuid.UUID{}
	}

	for h, id := range f.byHash {
		if id == instanceID {
			delete(f.byHash, h)
		}
	}
	f.byHash[hash] = instanceID
	return nil
}
func (f *fakeAPIKeyRepository) InstanceByHash(_ context.Context, hash string) (uuid.UUID, error) {
	if f.err != nil {
		return uuid.Nil, f.err
	}
	if id, ok := f.byHash[hash]; ok {
		return id, nil
	}
	return uuid.Nil, storage.ErrNotFound
}
func (f *fakeAPIKeyRepository) ClearHash(_ context.Context, instanceID uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	for hash, id := range f.byHash {
		if id == instanceID {
			delete(f.byHash, hash)
		}
	}
	return nil
}
func (f *fakeAPIKeyRepository) CountByOwner(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}
func (f *fakeAPIKeyRepository) CountAll(_ context.Context) (int, error) { return 0, nil }
