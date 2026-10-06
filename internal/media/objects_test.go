package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// fakeObjects is an in-memory Objects for object-store tests.
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

func (f *fakeObjects) EnsureBucket(context.Context) error { return nil }

func (f *fakeObjects) Put(_ context.Context, key string, data []byte, _ string) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = append([]byte(nil), data...)
	return nil
}

func (f *fakeObjects) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeObjects) Delete(_ context.Context, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *fakeObjects) Exists(_ context.Context, key string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.data[key]
	return ok, nil
}

func (f *fakeObjects) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.data[key]
	return ok
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
	defer body.Close()
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
	delete(objects.data, saved.ObjectKey)
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
	delete(objects.data, saved.ObjectKey)

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

func TestMigrateLocalFilesUploadsMissingObjectsWithChecksum(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()

	// Legacy row: file on disk, object absent.
	sum := sha256.Sum256([]byte("legacy"))
	legacy := model.Media{
		ID:         uuid.New(),
		InstanceID: instanceID,
		Direction:  "inbound",
		Mimetype:   "image/png",
		SizeBytes:  6,
		Bucket:     "wzap-media",
		ObjectKey:  filepath.Join("media", instanceID.String(), "legacy"),
		SHA256:     hex.EncodeToString(sum[:]),
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	if _, err := repo.Create(ctx, legacy); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	seedFile(t, dir, legacy.ObjectKey, []byte("legacy"))

	// Already-migrated row: object present, local file gone.
	migrated := model.Media{
		ID:         uuid.New(),
		InstanceID: instanceID,
		Direction:  "inbound",
		Mimetype:   "image/png",
		SizeBytes:  1,
		Bucket:     "wzap-media",
		ObjectKey:  filepath.Join("media", instanceID.String(), "migrated"),
		SHA256:     "anything",
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	if _, err := repo.Create(ctx, migrated); err != nil {
		t.Fatalf("seed migrated: %v", err)
	}
	objects.data[migrated.ObjectKey] = []byte("x")

	done, err := store.MigrateLocalFiles(ctx, nil)
	if err != nil {
		t.Fatalf("MigrateLocalFiles: %v", err)
	}
	if done != 1 {
		t.Errorf("migrated = %d, want 1", done)
	}
	if !objects.has(legacy.ObjectKey) {
		t.Error("legacy object was not uploaded")
	}
	got, _ := objects.Get(ctx, legacy.ObjectKey)
	body, _ := io.ReadAll(got)
	if string(body) != "legacy" {
		t.Errorf("uploaded content = %q", body)
	}
	// The local file is the recovery path and survives the migration.
	if _, err := os.Stat(filepath.Join(dir, legacy.ObjectKey)); err != nil {
		t.Errorf("local file removed by migration: %v", err)
	}
}

func TestMigrateLocalFilesSkipsChecksumMismatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	store.SetObjects(objects)
	instanceID := uuid.New()

	bad := model.Media{
		ID:         uuid.New(),
		InstanceID: instanceID,
		Direction:  "inbound",
		Mimetype:   "image/png",
		SizeBytes:  3,
		Bucket:     "wzap-media",
		ObjectKey:  filepath.Join("media", instanceID.String(), "bad"),
		SHA256:     "not-the-real-sum",
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	if _, err := repo.Create(ctx, bad); err != nil {
		t.Fatalf("seed bad: %v", err)
	}
	seedFile(t, dir, bad.ObjectKey, []byte("bad"))

	done, err := store.MigrateLocalFiles(ctx, nil)
	if err == nil {
		t.Error("MigrateLocalFiles succeeded with a checksum mismatch")
	}
	if done != 0 {
		t.Errorf("migrated = %d, want 0", done)
	}
	if objects.has(bad.ObjectKey) {
		t.Error("corrupt file was uploaded")
	}
}

func TestMigrateLocalFilesNoopWithoutObjects(t *testing.T) {
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	done, err := store.MigrateLocalFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("MigrateLocalFiles: %v", err)
	}
	if done != 0 {
		t.Errorf("migrated = %d, want 0", done)
	}
}

// model.Media carries the object-store identity; a zero Bucket only appears
// on pre-remodel rows that still point at a legacy local path.
func TestStorageRecordCarriesBucketAndKey(t *testing.T) {
	record := model.Media{Bucket: "wzap-media", ObjectKey: "media/i/m"}
	if record.Bucket != "wzap-media" || record.ObjectKey == "" {
		t.Error("media record must expose bucket and object key")
	}
}
