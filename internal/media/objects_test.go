package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
)

// fakeObjects is an in-memory Objects for object-store tests. Objects are
// keyed by (bucket, objectKey) exactly as the real store uses row buckets.
type fakeObjects struct {
	mu        sync.Mutex
	bucket    string
	data      map[string][]byte
	putErr    error
	getErr    error
	deleteErr error
	existsErr error
	deleted   []string
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{bucket: "wzap-media", data: map[string][]byte{}}
}

func (f *fakeObjects) Bucket() string { return f.bucket }

// fullKey composes the internal (bucket, key) identity.
func (f *fakeObjects) fullKey(bucket, key string) string {
	return bucket + "\x00" + key
}

// seed writes bytes directly into the store, bypassing Put error injection.
func (f *fakeObjects) seed(bucket, key string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[f.fullKey(bucket, key)] = append([]byte(nil), data...)
}

// drop removes one object directly, bypassing Delete error injection.
func (f *fakeObjects) drop(bucket, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, f.fullKey(bucket, key))
}

func (f *fakeObjects) EnsureBucket(context.Context) error { return nil }

func (f *fakeObjects) Put(_ context.Context, bucket, key string, data []byte, _ string) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[f.fullKey(bucket, key)] = append([]byte(nil), data...)
	return nil
}

func (f *fakeObjects) Get(_ context.Context, bucket, key string) (io.ReadCloser, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.data[f.fullKey(bucket, key)]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeObjects) Delete(_ context.Context, bucket, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, f.fullKey(bucket, key))
	f.deleted = append(f.deleted, f.fullKey(bucket, key))
	return nil
}

func (f *fakeObjects) Exists(_ context.Context, bucket, key string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.data[f.fullKey(bucket, key)]
	return ok, nil
}

func (f *fakeObjects) has(key string) bool {
	return f.hasIn(f.bucket, key)
}

// hasIn reports whether (bucket, key) exists.
func (f *fakeObjects) hasIn(bucket, key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.data[f.fullKey(bucket, key)]
	return ok
}

// deletedFrom reports whether a delete operation recorded (bucket, key).
func (f *fakeObjects) deletedFrom(bucket, key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	want := f.fullKey(bucket, key)
	for _, got := range f.deleted {
		if got == want {
			return true
		}
	}
	return false
}

func TestStorageSaveUploadsObjectAndRow(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()

	saved, err := store.Save(ctx, instanceID, "inbound", "", "image/png", "foto.png", []byte("conteúdo"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.Bucket != "wzap-media" {
		t.Errorf("Bucket = %q, want wzap-media", saved.Bucket)
	}
	if saved.ObjectKey != filepath.Join("media", instanceID.String(), saved.ID.String()) {
		t.Errorf("ObjectKey = %q", saved.ObjectKey)
	}
	if !objects.has(saved.ObjectKey) {
		t.Error("object missing in the store")
	}
	if files := countFiles(t, dir); files != 0 {
		t.Errorf("object mode wrote %d local file(s); the data dir is cache-only", files)
	}
}

func TestStorageSaveS3PutFailureDoesNotWriteLocalFile(t *testing.T) {
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	objects.putErr = errors.New("s3 unavailable")
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)

	if _, err := store.Save(context.Background(), uuid.New(), "inbound", "", "image/png", "x.png", []byte("data")); err == nil {
		t.Fatal("Save: expected S3 put failure")
	}
	if len(repo.records) != 0 {
		t.Errorf("records = %d, want none after failed put", len(repo.records))
	}
	if count := countFiles(t, dir); count != 0 {
		t.Errorf("wrote %d local file(s), want none when object put fails", count)
	}
}

func TestStorageSaveDropsObjectWhenRowCreateFails(t *testing.T) {
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	repo.createErr = errors.New("insert failed")
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)

	if _, err := store.Save(context.Background(), uuid.New(), "inbound", "", "image/png", "x.png", []byte("data")); err == nil {
		t.Fatal("Save succeeded with a failing repository")
	}
	if len(objects.data) != 0 {
		t.Errorf("failed Save left %d orphan object(s)", len(objects.data))
	}
	if len(objects.deleted) != 1 {
		t.Errorf("expected exactly one cleanup delete, got %v", objects.deleted)
	}
}

func TestStorageOpenStreamsObject(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()

	saved, err := store.Save(ctx, instanceID, "inbound", "", "image/jpeg", "foto.jpg", []byte("conteúdo"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	body, record, err := store.Open(ctx, saved.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = body.Close() }()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(got) != "conteúdo" {
		t.Errorf("body = %q", got)
	}
	if record.ID != saved.ID {
		t.Errorf("record id = %s, want %s", record.ID, saved.ID)
	}
}

func TestStorageOpenConfirmedDeletedReportsNotFound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()

	saved, err := store.Save(ctx, instanceID, "inbound", "", "image/png", "f.png", []byte("x"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	marked := time.Now()
	if err := repo.MarkObjectDeleted(ctx, saved.ID, marked); err != nil {
		t.Fatalf("MarkObjectDeleted: %v", err)
	}
	// The object may still be retrievable; the confirmed-deleted flag wins.
	if _, _, err := store.Open(ctx, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open error = %v, want ErrNotFound", err)
	}
	if _, _, err := store.Path(ctx, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Path error = %v, want ErrNotFound", err)
	}
}

func TestStoragePathMaterializesCacheFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()

	saved, err := store.Save(ctx, instanceID, "outbound", "", "image/png", "s.png", []byte("send me"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, record, err := store.Path(ctx, saved.ID)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if record.ID != saved.ID {
		t.Errorf("record id = %s, want %s", record.ID, saved.ID)
	}
	want := filepath.Join(dir, saved.ObjectKey)
	if path != want {
		t.Errorf("Path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized file: %v", err)
	}
	if string(got) != "send me" {
		t.Errorf("materialized content = %q", got)
	}
	// A second call serves the cache without a fresh object fetch: drop the
	// object and confirm Path still resolves from the cache.
	objects.drop(saved.Bucket, saved.ObjectKey)
	if _, _, err := store.Path(ctx, saved.ID); err != nil {
		t.Errorf("Path with missing object but warm cache: %v", err)
	}
	// A cold cache with a missing object reports ErrNotFound.
	if err := os.Remove(path); err != nil {
		t.Fatalf("drop cache: %v", err)
	}
	if _, _, err := store.Path(ctx, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Path error = %v, want ErrNotFound", err)
	}
}

func TestStorageDeleteExpiredMarksRowAfterConfirmedRemoval(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()
	now := time.Now()

	saved, err := store.Save(ctx, instanceID, "inbound", "", "image/png", "e.png", []byte("x"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Materialize the cache file before the row ages out.
	if _, _, err := store.Path(ctx, saved.ID); err != nil {
		t.Fatalf("warm the cache: %v", err)
	}
	// Age the row past its TTL.
	record := repo.records[saved.ID]
	record.ExpiresAt = now.Add(-time.Minute)
	repo.records[saved.ID] = record

	removed, err := store.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if objects.has(saved.ObjectKey) {
		t.Error("expired object still in the store")
	}
	row := repo.records[saved.ID]
	if row.ObjectDeletedAt == nil {
		t.Error("row missing object_deleted_at after confirmed removal")
	}
	if _, err := os.Stat(filepath.Join(dir, saved.ObjectKey)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("cache file still present: %v", err)
	}
	// A second pass is a no-op: the row no longer shows up as expired pending.
	again, err := store.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatalf("second DeleteExpired: %v", err)
	}
	if again != 0 {
		t.Errorf("second pass removed = %d, want 0 (idempotent)", again)
	}
}

func TestStorageDeleteExpiredKeepsRowUnmarkedWhenObjectDeleteFails(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	objects.deleteErr = errors.New("minio unreachable")
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()
	now := time.Now()

	saved, err := store.Save(ctx, instanceID, "inbound", "", "image/png", "e.png", []byte("x"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	record := repo.records[saved.ID]
	record.ExpiresAt = now.Add(-time.Minute)
	repo.records[saved.ID] = record

	removed, err := store.DeleteExpired(ctx, now)
	if err == nil {
		t.Error("DeleteExpired succeeded while the object store failed")
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if repo.records[saved.ID].ObjectDeletedAt != nil {
		t.Error("row was marked deleted although the object removal failed")
	}
	if !objects.has(saved.ObjectKey) {
		t.Error("object vanished although the delete failed")
	}
}

func TestStorageDeleteExpiredIdempotentWhenObjectAlreadyGone(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()
	now := time.Now()

	saved, err := store.Save(ctx, instanceID, "inbound", "", "image/png", "e.png", []byte("x"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	record := repo.records[saved.ID]
	record.ExpiresAt = now.Add(-time.Minute)
	repo.records[saved.ID] = record
	// The object vanished out of band: delete still counts as confirmed.
	objects.drop(saved.Bucket, saved.ObjectKey)

	removed, err := store.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if repo.records[saved.ID].ObjectDeletedAt == nil {
		t.Error("row missing object_deleted_at for an already absent object")
	}
}

// TestStorageDeleteExpiredUsesRowBucket pins the row-truth rule: the delete
// targets the bucket recorded on the media row, never the currently
// configured one. An upgrade running with a different WZAP_S3_BUCKET must
// not delete objects out of the bucket they were written to — deleting from
// the wrong bucket orphans real objects while stamping the rows as gone.
func TestStorageDeleteExpiredUsesRowBucket(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	now := time.Now()

	// Row written by a deployment with a different bucket name.
	legacyKey := filepath.Join("media", uuid.NewString(), "legacy")
	legacy := model.Media{
		ID:         uuid.New(),
		InstanceID: uuid.New(),
		Direction:  "inbound",
		Mimetype:   "image/png",
		SizeBytes:  1,
		Bucket:     "wzap-media-legacy",
		ObjectKey:  legacyKey,
		ExpiresAt:  now.Add(-time.Minute),
	}
	if _, err := repo.Create(ctx, legacy); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	objects.seed("wzap-media-legacy", legacyKey, []byte("real object"))
	// Same key in the currently configured bucket: must survive untouched.
	objects.seed(objects.bucket, legacyKey, []byte("innocent neighbor"))

	// Current row saved through the storage itself.
	instanceID := uuid.New()
	fresh, err := store.Save(ctx, instanceID, "inbound", "", "image/png", "f.png", []byte("z"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	freshRow := repo.records[fresh.ID]
	freshRow.ExpiresAt = now.Add(-time.Minute)
	repo.records[fresh.ID] = freshRow

	removed, err := store.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	// The legacy object was deleted from its own row bucket.
	if objects.hasIn("wzap-media-legacy", legacyKey) {
		t.Error("legacy object still present in its row bucket")
	}
	if !objects.deletedFrom("wzap-media-legacy", legacyKey) {
		t.Error("delete did not target the row bucket wzap-media-legacy")
	}
	// The configured bucket never saw a delete for that key.
	if !objects.hasIn(objects.bucket, legacyKey) {
		t.Error("object in the configured bucket was deleted by the legacy row")
	}
	if objects.deletedFrom(objects.bucket, legacyKey) {
		t.Error("a delete was issued against the configured bucket for the legacy row key")
	}

	// Every processed row is marked deleted exactly against its real object.
	for _, row := range []model.Media{legacy, repo.records[fresh.ID]} {
		if repo.records[row.ID].ObjectDeletedAt == nil {
			t.Errorf("row %s missing object_deleted_at", row.ID)
		}
	}
}

func TestStorageDeleteByInstanceRemovesObjectsRowsAndCache(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceA, instanceB := uuid.New(), uuid.New()

	first, err := store.Save(ctx, instanceA, "inbound", "", "image/jpeg", "a.jpg", []byte("a"))
	if err != nil {
		t.Fatalf("first Save: %v", err)
	}
	second, err := store.Save(ctx, instanceA, "outbound", "", "image/jpeg", "b.jpg", []byte("b"))
	if err != nil {
		t.Fatalf("second Save: %v", err)
	}
	kept, err := store.Save(ctx, instanceB, "inbound", "", "image/jpeg", "c.jpg", []byte("c"))
	if err != nil {
		t.Fatalf("third Save: %v", err)
	}
	// Materialize a cache file for one of the removed records.
	if _, _, err := store.Path(ctx, first.ID); err != nil {
		t.Fatalf("warm cache: %v", err)
	}

	if err := store.DeleteByInstance(ctx, instanceA); err != nil {
		t.Fatalf("DeleteByInstance: %v", err)
	}

	for _, record := range []*model.Media{first, second} {
		if objects.has(record.ObjectKey) {
			t.Errorf("object %s still in the store", record.ObjectKey)
		}
		if _, ok := repo.records[record.ID]; ok {
			t.Errorf("record %s still stored", record.ID)
		}
	}
	if !objects.has(kept.ObjectKey) {
		t.Error("other instance object was removed")
	}
	if _, ok := repo.records[kept.ID]; !ok {
		t.Error("other instance record was removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "media", instanceA.String())); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("instance cache dir still present: %v", err)
	}
}

// model.Media carries the object-store identity for every read and delete.
func TestStorageRecordCarriesBucketAndKey(t *testing.T) {
	record := model.Media{Bucket: "wzap-media", ObjectKey: "media/i/m"}
	if record.Bucket != "wzap-media" || record.ObjectKey == "" {
		t.Error("media record must expose bucket and object key")
	}
}
