package httpapi_test

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/storage"
)

// fakeIdempotency is an in-memory IdempotencyRepository for the middleware
// tests. It records the calls and can force acquisition failures.
type fakeIdempotency struct {
	records    map[string]*model.IdempotencyRecord
	acquireErr error

	acquires  []acquireCall
	completes []completeCall
	releases  []releaseCall
}

// acquireCall records one Acquire invocation.
type acquireCall struct {
	instanceID  uuid.UUID
	key         string
	fingerprint string
	expiresAt   time.Time
}

// completeCall records one Complete invocation.
type completeCall struct {
	instanceID uuid.UUID
	key        string
	status     int
	body       string
}

// releaseCall records one Release invocation.
type releaseCall struct {
	instanceID uuid.UUID
	key        string
}

func newFakeIdempotency() *fakeIdempotency {
	return &fakeIdempotency{records: make(map[string]*model.IdempotencyRecord)}
}
func fakeRecordKey(instanceID uuid.UUID, key string) string {
	return instanceID.String() + "\x00" + key
}

// Acquire owns a key that does not exist yet, replays a completed one and
// reports in-progress and mismatched keys.
func (f *fakeIdempotency) Acquire(
	_ context.Context, instanceID uuid.UUID, key, fingerprint string, expiresAt time.Time,
) (*model.IdempotencyRecord, bool, error) {
	f.acquires = append(f.acquires, acquireCall{
		instanceID: instanceID, key: key, fingerprint: fingerprint, expiresAt: expiresAt,
	})
	if f.acquireErr != nil {
		return nil, false, f.acquireErr
	}

	record, ok := f.records[fakeRecordKey(instanceID, key)]
	if !ok {
		record = &model.IdempotencyRecord{
			InstanceID:  instanceID,
			Key:         key,
			Fingerprint: fingerprint,
			Status:      "in_progress",
			ExpiresAt:   expiresAt,
		}
		f.records[fakeRecordKey(instanceID, key)] = record
		return record, true, nil
	}
	if record.Fingerprint != fingerprint {
		return nil, false, fmt.Errorf("acquire idempotency key: %w", storage.ErrFingerprintMismatch)
	}
	if record.Status != "completed" {
		return nil, false, fmt.Errorf("acquire idempotency key: %w", storage.ErrInProgress)
	}
	return record, false, nil
}

// Complete stores the response under key.
func (f *fakeIdempotency) Complete(_ context.Context, instanceID uuid.UUID, key string, status int, body []byte) error {
	f.completes = append(f.completes, completeCall{instanceID: instanceID, key: key, status: status, body: string(body)})
	record, ok := f.records[fakeRecordKey(instanceID, key)]
	if !ok {
		return fmt.Errorf("complete idempotency key: %w", storage.ErrNotFound)
	}
	record.Status = "completed"
	record.ResponseStatus = status
	record.ResponseBody = append([]byte(nil), body...)
	return nil
}

// Release drops key.
func (f *fakeIdempotency) Release(_ context.Context, instanceID uuid.UUID, key string) error {
	f.releases = append(f.releases, releaseCall{instanceID: instanceID, key: key})
	delete(f.records, fakeRecordKey(instanceID, key))
	return nil
}

// DeleteExpired is unused by the middleware.
func (f *fakeIdempotency) DeleteExpired(context.Context) (int64, error) { return 0, nil }
