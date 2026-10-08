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
	"wzap/internal/storage"
)

// fakeMediaRepo is an in-memory storage.MediaRepository for storage tests.
type fakeMediaRepo struct {
	records   map[uuid.UUID]model.Media
	createErr error
	listErr   error
	deleteErr error
}

func newFakeMediaRepo() *fakeMediaRepo {
	return &fakeMediaRepo{records: map[uuid.UUID]model.Media{}}
}

func (f *fakeMediaRepo) Create(_ context.Context, record model.Media) (*model.Media, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.records[record.ID] = record
	copied := record
	return &copied, nil
}

func (f *fakeMediaRepo) Get(_ context.Context, id uuid.UUID) (*model.Media, error) {
	record, ok := f.records[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	copied := record
	return &copied, nil
}

func (f *fakeMediaRepo) ListByInstance(_ context.Context, instanceID uuid.UUID) ([]model.Media, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	records := []model.Media{}
	for _, record := range f.records {
		if record.InstanceID == instanceID {
			records = append(records, record)
		}
	}
	return records, nil
}

func (f *fakeMediaRepo) ListExpired(_ context.Context, now time.Time) ([]model.Media, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	records := []model.Media{}
	for _, record := range f.records {
		if !record.ExpiresAt.After(now) && record.ObjectDeletedAt == nil {
			records = append(records, record)
		}
	}
	return records, nil
}

func (f *fakeMediaRepo) Delete(_ context.Context, id uuid.UUID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.records[id]; !ok {
		return storage.ErrNotFound
	}
	delete(f.records, id)
	return nil
}

func (f *fakeMediaRepo) MarkObjectDeleted(_ context.Context, id uuid.UUID, at time.Time) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	record, ok := f.records[id]
	if !ok {
		return storage.ErrNotFound
	}
	record.ObjectDeletedAt = &at
	f.records[id] = record
	return nil
}

func (f *fakeMediaRepo) DeleteByInstance(_ context.Context, instanceID uuid.UUID) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	var removed int64
	for id, record := range f.records {
		if record.InstanceID == instanceID {
			delete(f.records, id)
			removed++
		}
	}
	return removed, nil
}

// seedFile writes data at dir/rel, creating the parent directories.
func seedFile(t *testing.T, dir, rel string, data []byte) {
	t.Helper()

	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create media dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}
}

// countFiles returns how many regular files live under dir.
func countFiles(t *testing.T, dir string) int {
	t.Helper()

	count := 0
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return count
}

func TestStorageSaveAndOpen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
	instanceID := uuid.New()
	messageID := uuid.NewString()
	payload := []byte("conteúdo da mídia")

	saved, err := store.Save(ctx, instanceID, "inbound", messageID, "image/jpeg", "foto.jpg", payload)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if saved.ID == uuid.Nil {
		t.Error("Save: ID is nil")
	}
	if saved.ID.Version() != 4 {
		t.Errorf("Save: ID %s is not a random UUIDv4", saved.ID)
	}
	if saved.InstanceID != instanceID {
		t.Errorf("Save: InstanceID = %s, want %s", saved.InstanceID, instanceID)
	}
	if saved.Direction != "inbound" {
		t.Errorf("Save: Direction = %q, want inbound", saved.Direction)
	}
	if saved.MessageID != messageID {
		t.Errorf("Save: MessageID = %q, want %q", saved.MessageID, messageID)
	}
	if saved.Mimetype != "image/jpeg" {
		t.Errorf("Save: Mimetype = %q, want image/jpeg", saved.Mimetype)
	}
	if saved.Filename != "foto.jpg" {
		t.Errorf("Save: Filename = %q, want foto.jpg", saved.Filename)
	}
	if saved.SizeBytes != int64(len(payload)) {
		t.Errorf("Save: SizeBytes = %d, want %d", saved.SizeBytes, len(payload))
	}
	if saved.Bucket != "local" {
		t.Errorf("Save: Bucket = %q, want local for disk-only storage", saved.Bucket)
	}
	wantHash := sha256.Sum256(payload)
	if saved.SHA256 != hex.EncodeToString(wantHash[:]) {
		t.Errorf("Save: SHA256 = %q, want %q", saved.SHA256, hex.EncodeToString(wantHash[:]))
	}
	if saved.CreatedAt.IsZero() {
		t.Error("Save: CreatedAt is zero")
	}
	if want := saved.CreatedAt.Add(time.Hour); !saved.ExpiresAt.Equal(want) {
		t.Errorf("Save: ExpiresAt = %v, want %v", saved.ExpiresAt, want)
	}

	wantPath := filepath.Join("media", instanceID.String(), saved.ID.String())
	if saved.ObjectKey != wantPath {
		t.Errorf("Save: StoragePath = %q, want %q", saved.ObjectKey, wantPath)
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, wantPath))
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !bytes.Equal(onDisk, payload) {
		t.Errorf("stored bytes = %q, want %q", onDisk, payload)
	}

	body, opened, err := store.Open(ctx, saved.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = body.Close() }()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read opened media: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("Open bytes = %q, want %q", got, payload)
	}
	if opened.ID != saved.ID || opened.Mimetype != saved.Mimetype || opened.SizeBytes != saved.SizeBytes {
		t.Errorf("Open metadata = %+v, want id %s, mimetype %q, size %d",
			opened, saved.ID, saved.Mimetype, saved.SizeBytes)
	}
}

func TestStorageSaveOpaqueIDs(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewStorage(dir, newFakeMediaRepo(), 1<<20, time.Hour)
	instanceID := uuid.New()

	first, err := store.Save(ctx, instanceID, "outbound", "", "image/png", "a.png", []byte("a"))
	if err != nil {
		t.Fatalf("first Save: %v", err)
	}
	second, err := store.Save(ctx, instanceID, "outbound", "", "image/png", "b.png", []byte("b"))
	if err != nil {
		t.Fatalf("second Save: %v", err)
	}

	if first.ID == second.ID {
		t.Error("two saved media share the same ID")
	}
	if first.ID.Version() != 4 || second.ID.Version() != 4 {
		t.Errorf("IDs %s/%s are not random UUIDv4", first.ID, second.ID)
	}
}

func TestStorageSaveRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1024, time.Hour)

	for name, data := range map[string][]byte{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			_, err := store.Save(context.Background(), uuid.New(), "outbound", "", "image/png", "x.png", data)
			if !errors.Is(err, ErrEmpty) {
				t.Fatalf("Save error = %v, want ErrEmpty", err)
			}
		})
	}

	if files := countFiles(t, dir); files != 0 {
		t.Errorf("rejected Save left %d file(s) behind", files)
	}
	if len(repo.records) != 0 {
		t.Errorf("rejected Save left %d record(s) behind", len(repo.records))
	}
}

func TestStorageSaveEnforcesMaxBytes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewStorage(dir, newFakeMediaRepo(), 8, time.Hour)

	if _, err := store.Save(ctx, uuid.New(), "outbound", "", "image/png", "big.png",
		bytes.Repeat([]byte("a"), 9)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over-limit Save error = %v, want ErrTooLarge", err)
	}
	if _, err := store.Save(ctx, uuid.New(), "outbound", "", "image/png", "exact.png",
		bytes.Repeat([]byte("a"), 8)); err != nil {
		t.Fatalf("Save at the limit: %v", err)
	}
}

func TestStorageSaveRemovesFileWhenRepositoryFails(t *testing.T) {
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	repo.createErr = errors.New("insert failed")
	store := NewStorage(dir, repo, 1024, time.Hour)

	if _, err := store.Save(context.Background(), uuid.New(), "inbound", "", "image/png", "x.png", []byte("data")); err == nil {
		t.Fatal("Save succeeded with a failing repository")
	}
	if files := countFiles(t, dir); files != 0 {
		t.Errorf("failed Save left %d file(s) behind", files)
	}
}

func TestStorageSaveSanitizesFilename(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewStorage(dir, newFakeMediaRepo(), 1024, time.Hour)
	instanceID := uuid.New()

	saved, err := store.Save(ctx, instanceID, "outbound", "", "application/pdf", "../../etc/passwd", []byte("data"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.Filename != "passwd" {
		t.Errorf("Filename = %q, want the base name passwd", saved.Filename)
	}

	unnamed, err := store.Save(ctx, instanceID, "outbound", "", "application/pdf", "  ", []byte("data"))
	if err != nil {
		t.Fatalf("Save without filename: %v", err)
	}
	if unnamed.Filename != "" {
		t.Errorf("Filename = %q, want empty", unnamed.Filename)
	}
}

func TestStorageOpenErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1024, time.Hour)
	instanceID := uuid.New()

	saved, err := store.Save(ctx, instanceID, "inbound", "", "image/jpeg", "foto.jpg", []byte("conteúdo"))
	if err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	t.Run("unknown id", func(t *testing.T) {
		if _, _, err := store.Open(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open error = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		expired := model.Media{
			ID:         uuid.New(),
			InstanceID: instanceID,
			ExpiresAt:  time.Now().Add(-time.Minute),
			ObjectKey:  filepath.Join("media", instanceID.String(), "expired"),
		}
		seedFile(t, dir, expired.ObjectKey, []byte("old"))
		if _, err := repo.Create(ctx, expired); err != nil {
			t.Fatalf("seed expired record: %v", err)
		}
		if _, _, err := store.Open(ctx, expired.ID); !errors.Is(err, ErrExpired) {
			t.Errorf("Open error = %v, want ErrExpired", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if err := os.Remove(filepath.Join(dir, saved.ObjectKey)); err != nil {
			t.Fatalf("remove stored file: %v", err)
		}
		if _, _, err := store.Open(ctx, saved.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open error = %v, want ErrNotFound", err)
		}
	})

	t.Run("path escapes the data dir", func(t *testing.T) {
		escape := model.Media{
			ID:         uuid.New(),
			InstanceID: instanceID,
			ExpiresAt:  time.Now().Add(time.Hour),
			ObjectKey:  filepath.Join("..", "etc", "passwd"),
		}
		if _, err := repo.Create(ctx, escape); err != nil {
			t.Fatalf("seed escaping record: %v", err)
		}
		if _, _, err := store.Open(ctx, escape.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open error = %v, want ErrNotFound", err)
		}
	})
}

func TestStorageDeleteExpiredRemovesFileAndRow(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1024, time.Hour)
	instanceID := uuid.New()
	now := time.Now()

	expired := model.Media{
		ID:         uuid.New(),
		InstanceID: instanceID,
		Direction:  "inbound",
		Mimetype:   "image/png",
		SizeBytes:  3,
		ObjectKey:  filepath.Join("media", instanceID.String(), "expired"),
		ExpiresAt:  now.Add(-time.Minute),
	}
	fresh := model.Media{
		ID:         uuid.New(),
		InstanceID: instanceID,
		Direction:  "inbound",
		Mimetype:   "image/png",
		SizeBytes:  3,
		ObjectKey:  filepath.Join("media", instanceID.String(), "fresh"),
		ExpiresAt:  now.Add(time.Hour),
	}
	seedFile(t, dir, expired.ObjectKey, []byte("old"))
	seedFile(t, dir, fresh.ObjectKey, []byte("new"))
	for _, record := range []model.Media{expired, fresh} {
		if _, err := repo.Create(ctx, record); err != nil {
			t.Fatalf("seed record: %v", err)
		}
	}

	removed, err := store.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("DeleteExpired removed = %d, want 1", removed)
	}

	if _, err := os.Stat(filepath.Join(dir, expired.ObjectKey)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expired file still present: %v", err)
	}
	record, ok := repo.records[expired.ID]
	if !ok {
		t.Error("expired metadata row was deleted instead of preserved")
	} else if record.ObjectDeletedAt == nil {
		t.Error("expired record missing object_deleted_at after confirmed removal")
	}
	if _, err := os.Stat(filepath.Join(dir, fresh.ObjectKey)); err != nil {
		t.Errorf("fresh file missing: %v", err)
	}
	if _, ok := repo.records[fresh.ID]; !ok {
		t.Error("fresh record was removed")
	}
}

func TestStorageDeleteExpiredKeepsRowWhenFileRemovalFails(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1024, time.Hour)
	now := time.Now()

	escaping := model.Media{
		ID:         uuid.New(),
		InstanceID: uuid.New(),
		ObjectKey:  filepath.Join("..", "outside"),
		ExpiresAt:  now.Add(-time.Minute),
	}
	if _, err := repo.Create(ctx, escaping); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	removed, err := store.DeleteExpired(ctx, now)
	if err == nil {
		t.Error("DeleteExpired succeeded with an unremovable path")
	}
	if removed != 0 {
		t.Errorf("DeleteExpired removed = %d, want 0", removed)
	}
	if _, ok := repo.records[escaping.ID]; !ok {
		t.Error("row was deleted even though its file could not be removed")
	}
}

func TestStorageDeleteByInstanceRemovesFilesAndRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1<<20, time.Hour)
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

	if err := store.DeleteByInstance(ctx, instanceA); err != nil {
		t.Fatalf("DeleteByInstance: %v", err)
	}

	for _, record := range []*model.Media{first, second} {
		if _, err := os.Stat(filepath.Join(dir, record.ObjectKey)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("file %s still present: %v", record.ObjectKey, err)
		}
		if _, ok := repo.records[record.ID]; ok {
			t.Errorf("record %s still stored", record.ID)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, kept.ObjectKey)); err != nil {
		t.Errorf("other instance file missing: %v", err)
	}
	if _, ok := repo.records[kept.ID]; !ok {
		t.Error("other instance record was removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "media", instanceA.String())); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("instance media dir still present: %v", err)
	}
}

func TestStoragePathReturnsStoredFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewStorage(dir, newFakeMediaRepo(), 1<<20, time.Hour)
	instanceID := uuid.New()
	payload := []byte("conteúdo da mídia")

	saved, err := store.Save(ctx, instanceID, "outbound", "", "image/jpeg", "foto.jpg", payload)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, record, err := store.Path(ctx, saved.ID)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join(dir, saved.ObjectKey); path != want {
		t.Errorf("Path = %q, want %q", path, want)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Path = %q, want an absolute path", path)
	}
	if record.ID != saved.ID || record.Mimetype != saved.Mimetype || record.SizeBytes != saved.SizeBytes {
		t.Errorf("Path metadata = %+v, want the saved record %+v", record, saved)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read path: %v", err)
	}
	if !bytes.Equal(onDisk, payload) {
		t.Errorf("file bytes = %q, want %q", onDisk, payload)
	}
}

func TestStoragePathErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo := newFakeMediaRepo()
	store := NewStorage(dir, repo, 1024, time.Hour)
	instanceID := uuid.New()

	saved, err := store.Save(ctx, instanceID, "outbound", "", "image/jpeg", "foto.jpg", []byte("conteúdo"))
	if err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	t.Run("unknown id", func(t *testing.T) {
		if _, _, err := store.Path(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Errorf("Path error = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		expired := model.Media{
			ID:         uuid.New(),
			InstanceID: instanceID,
			ExpiresAt:  time.Now().Add(-time.Minute),
			ObjectKey:  filepath.Join("media", instanceID.String(), "expired"),
		}
		seedFile(t, dir, expired.ObjectKey, []byte("old"))
		if _, err := repo.Create(ctx, expired); err != nil {
			t.Fatalf("seed expired record: %v", err)
		}
		if _, _, err := store.Path(ctx, expired.ID); !errors.Is(err, ErrExpired) {
			t.Errorf("Path error = %v, want ErrExpired", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if err := os.Remove(filepath.Join(dir, saved.ObjectKey)); err != nil {
			t.Fatalf("remove stored file: %v", err)
		}
		if _, _, err := store.Path(ctx, saved.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("Path error = %v, want ErrNotFound", err)
		}
	})
}

func TestKind(t *testing.T) {
	tests := []struct {
		mimetype string
		want     string
		ok       bool
	}{
		{mimetype: "image/jpeg", want: KindImage, ok: true},
		{mimetype: "IMAGE/PNG", want: KindImage, ok: true},
		{mimetype: "video/mp4", want: KindVideo, ok: true},
		{mimetype: "video/3gpp", want: KindVideo, ok: true},
		{mimetype: "audio/ogg", want: KindAudio, ok: true},
		{mimetype: "audio/mpeg", want: KindAudio, ok: true},
		{mimetype: "application/pdf", want: KindDocument, ok: true},
		{mimetype: "text/plain", want: KindDocument, ok: true},
		{mimetype: "application/octet-stream", ok: false},
		{mimetype: "image/svg+xml", ok: false},
		{mimetype: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.mimetype, func(t *testing.T) {
			got, ok := Kind(tt.mimetype)
			if ok != tt.ok || got != tt.want {
				t.Errorf("Kind(%q) = (%q, %v), want (%q, %v)", tt.mimetype, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestAllowedMime(t *testing.T) {
	allowed := []string{"image/jpeg", "image/png", "image/webp", "video/mp4", "audio/ogg", "application/pdf", " IMAGE/JPEG "}
	for _, mimetype := range allowed {
		if !AllowedMime(mimetype) {
			t.Errorf("AllowedMime(%q) = false, want true", mimetype)
		}
	}

	rejected := []string{"", "text/html", "application/x-executable", "image/svg+xml", "application/octet-stream"}
	for _, mimetype := range rejected {
		if AllowedMime(mimetype) {
			t.Errorf("AllowedMime(%q) = true, want false", mimetype)
		}
	}
}

// cancelAfterPutObjects models an upload completing as its caller disconnects.
// Unlike the generic object fake, deletion honors cancellation and deadlines.
type cancelAfterPutObjects struct {
	*fakeObjects
	cancel         context.CancelFunc
	cleanupBounded bool
}

func (f *cancelAfterPutObjects) Put(ctx context.Context, bucket, key string, data []byte, mime string) error {
	if err := f.fakeObjects.Put(ctx, bucket, key, data, mime); err != nil {
		return err
	}
	f.cancel()
	return nil
}
func (f *cancelAfterPutObjects) Delete(ctx context.Context, bucket, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	f.cleanupBounded = ok && time.Until(deadline) > 0 && time.Until(deadline) <= time.Minute
	return f.fakeObjects.Delete(ctx, bucket, key)
}
func TestStorageSaveCanceledAfterUploadRemovesOrphan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := newFakeMediaRepo()
	repo.createErr = context.Canceled
	objects := &cancelAfterPutObjects{fakeObjects: newFakeObjects(), cancel: cancel}
	store := NewStorage(t.TempDir(), repo, 1024, time.Hour)
	store.SetObjects(objects)
	saved, err := store.Save(ctx, uuid.New(), "inbound", "", "image/png", "x.png", []byte("data"))
	if saved != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Save = %v, %v; want original cancellation error", saved, err)
	}
	if len(objects.data) != 0 {
		t.Fatalf("failed Save left %d remote object(s) behind", len(objects.data))
	}
	if !objects.cleanupBounded {
		t.Fatal("orphan cleanup has no bounded deadline")
	}
}

// snapshotMediaRepo pauses a deletion after its list snapshot, but preserves
// thread-safe rows and the instance foreign-key behavior of the real DB.
type snapshotMediaRepo struct {
	*fakeMediaRepo
	mu       sync.Mutex
	snapshot chan struct{}
	resume   chan struct{}
	deleted  bool
}

func (r *snapshotMediaRepo) Create(ctx context.Context, record model.Media) (*model.Media, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted {
		return nil, storage.ErrNotFound
	}
	return r.fakeMediaRepo.Create(ctx, record)
}
func (r *snapshotMediaRepo) ListByInstance(ctx context.Context, id uuid.UUID) ([]model.Media, error) {
	r.mu.Lock()
	rows, err := r.fakeMediaRepo.ListByInstance(ctx, id)
	r.mu.Unlock()
	close(r.snapshot)
	<-r.resume
	return rows, err
}
func (r *snapshotMediaRepo) DeleteByInstance(ctx context.Context, id uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeMediaRepo.DeleteByInstance(ctx, id)
}
func TestStorageDeleteByInstanceConcurrentSaveRetainsReference(t *testing.T) {
	repo := &snapshotMediaRepo{fakeMediaRepo: newFakeMediaRepo(), snapshot: make(chan struct{}), resume: make(chan struct{})}
	objects := newFakeObjects()
	store := NewStorage(t.TempDir(), repo, 1024, time.Hour)
	store.SetObjects(objects)
	id := uuid.New()
	deletion := make(chan error, 1)
	go func() { deletion <- store.DeleteByInstance(context.Background(), id) }()
	<-repo.snapshot
	save := make(chan error, 1)
	go func() {
		_, err := store.Save(context.Background(), id, "inbound", "", "image/png", "x.png", []byte("data"))
		save <- err
	}()
	// Before the fix Save finishes inside the snapshot-to-delete gap. With the
	// guard it waits; allow deletion to finish without a two-party barrier.
	select {
	case err := <-save:
		if err != nil {
			t.Fatal(err)
		}
		save <- err
	case <-time.After(100 * time.Millisecond):
	}
	close(repo.resume)
	if err := <-deletion; err != nil {
		t.Fatal(err)
	}
	if err := <-save; err != nil {
		t.Fatal(err)
	}
	repo.mu.Lock()
	rows := len(repo.records)
	repo.mu.Unlock()
	objects.mu.Lock()
	stored := len(objects.data)
	objects.mu.Unlock()
	if stored != rows {
		t.Fatalf("remote objects=%d metadata rows=%d; concurrent Save lost its reference", stored, rows)
	}
}

func TestStorageDeleteWithInstanceFailureAllowsRetry(t *testing.T) {
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(t.TempDir(), repo, 1024, time.Hour)
	store.SetObjects(objects)
	id := uuid.New()
	want := errors.New("instance delete failed")
	if err := store.DeleteWithInstance(context.Background(), id, func() error { return want }); !errors.Is(err, want) {
		t.Fatalf("deletion error=%v; want callback failure", err)
	}
	if _, err := store.Save(context.Background(), id, "inbound", "", "image/png", "x.png", []byte("data")); err != nil {
		t.Fatalf("Save after failed deletion: %v", err)
	}
	if err := store.DeleteWithInstance(context.Background(), id, func() error { return nil }); err != nil {
		t.Fatalf("retry deletion: %v", err)
	}
	if len(repo.records) != 0 || len(objects.data) != 0 {
		t.Fatalf("retry left %d metadata rows and %d remote objects", len(repo.records), len(objects.data))
	}
}

func TestStorageDeleteWithInstanceWaitingSaveHonorsCancellation(t *testing.T) {
	store := NewStorage(t.TempDir(), newFakeMediaRepo(), 1024, time.Hour)
	id := uuid.New()
	entered, release := make(chan struct{}), make(chan struct{})
	deleted := make(chan error, 1)
	go func() {
		deleted <- store.DeleteWithInstance(context.Background(), id, func() error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	saved := make(chan error, 1)
	go func() {
		_, err := store.Save(ctx, id, "inbound", "", "image/png", "x.png", []byte("data"))
		saved <- err
	}()
	cancel()
	select {
	case err := <-saved:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("waiting Save error=%v; want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Error("canceled Save waited for instance deletion")
	}
	close(release)
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
}

func TestStorageDeleteWithInstanceDoesNotBlockOtherInstances(t *testing.T) {
	store := NewStorage(t.TempDir(), newFakeMediaRepo(), 1024, time.Hour)
	entered, release := make(chan struct{}), make(chan struct{})
	deleted := make(chan error, 1)
	go func() {
		deleted <- store.DeleteWithInstance(context.Background(), uuid.New(), func() error { close(entered); <-release; return nil })
	}()
	<-entered
	saved := make(chan error, 1)
	go func() {
		_, err := store.Save(context.Background(), uuid.New(), "inbound", "", "image/png", "x.png", []byte("data"))
		saved <- err
	}()
	select {
	case err := <-saved:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("deletion blocked upload for a different instance")
	}
	close(release)
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
}

func TestStorageDeleteWithInstanceKeepsInstanceOnMediaFailure(t *testing.T) {
	repo := newFakeMediaRepo()
	objects := newFakeObjects()
	store := NewStorage(t.TempDir(), repo, 1024, time.Hour)
	store.SetObjects(objects)
	id := uuid.New()
	if _, err := store.Save(context.Background(), id, "inbound", "", "image/png", "x.png", []byte("data")); err != nil {
		t.Fatal(err)
	}
	objects.deleteErr = errors.New("object backend down")
	parentDeleted := false
	removeParent := func() error { parentDeleted = true; return nil }
	if err := store.DeleteWithInstance(context.Background(), id, removeParent); !errors.Is(err, objects.deleteErr) {
		t.Fatalf("deletion error=%v; want object backend failure", err)
	}
	if parentDeleted || len(repo.records) != 1 || len(objects.data) != 1 {
		t.Fatal("media failure removed parent or lost retryable content")
	}
	objects.deleteErr = nil
	if err := store.DeleteWithInstance(context.Background(), id, removeParent); err != nil {
		t.Fatal(err)
	}
	if !parentDeleted || len(repo.records) != 0 || len(objects.data) != 0 {
		t.Fatal("retry did not remove content before parent")
	}
}

// cacheRaceRepo preserves thread-safe metadata while exposing an optional pause
// after the initial lookup has captured a row but before Path receives it.
type cacheRaceRepo struct {
	*fakeMediaRepo
	mu     sync.Mutex
	lookup chan struct{}
	resume chan struct{}
	once   sync.Once
}

func (r *cacheRaceRepo) Get(ctx context.Context, id uuid.UUID) (*model.Media, error) {
	r.mu.Lock()
	row, err := r.fakeMediaRepo.Get(ctx, id)
	r.mu.Unlock()
	if r.lookup != nil {
		r.once.Do(func() { close(r.lookup); <-r.resume })
	}
	return row, err
}
func (r *cacheRaceRepo) ListByInstance(ctx context.Context, id uuid.UUID) ([]model.Media, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeMediaRepo.ListByInstance(ctx, id)
}
func (r *cacheRaceRepo) ListExpired(ctx context.Context, now time.Time) ([]model.Media, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeMediaRepo.ListExpired(ctx, now)
}
func (r *cacheRaceRepo) DeleteByInstance(ctx context.Context, id uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeMediaRepo.DeleteByInstance(ctx, id)
}
func (r *cacheRaceRepo) MarkObjectDeleted(ctx context.Context, id uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeMediaRepo.MarkObjectDeleted(ctx, id, now)
}

type pausedCacheObjects struct {
	*fakeObjects
	read   chan struct{}
	resume chan struct{}
}

func (o *pausedCacheObjects) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	body, err := o.fakeObjects.Get(ctx, bucket, key)
	close(o.read)
	<-o.resume
	return body, err
}
func TestStoragePathDoesNotRecreateDeletedCache(t *testing.T) {
	for _, cleanup := range []string{"instance", "expiry"} {
		for _, pause := range []string{"object read", "initial metadata lookup"} {
			t.Run(cleanup+"/"+pause, func(t *testing.T) {
				dir := t.TempDir()
				repo := &cacheRaceRepo{fakeMediaRepo: newFakeMediaRepo()}
				objects := newFakeObjects()
				store := NewStorage(dir, repo, 1024, time.Hour)
				store.SetObjects(objects)
				current := time.Now()
				store.now = func() time.Time { return current }
				instanceID := uuid.New()
				row, err := store.Save(context.Background(), instanceID, "inbound", "", "image/png", "x.png", []byte("data"))
				if err != nil {
					t.Fatal(err)
				}
				reached, resume := make(chan struct{}), make(chan struct{})
				if pause == "object read" {
					store.SetObjects(&pausedCacheObjects{fakeObjects: objects, read: reached, resume: resume})
				} else {
					repo.lookup, repo.resume = reached, resume
				}
				materialized := make(chan error, 1)
				go func() { _, _, err := store.Path(context.Background(), row.ID); materialized <- err }()
				<-reached
				deleted := make(chan error, 1)
				go func() {
					if cleanup == "instance" {
						deleted <- store.DeleteWithInstance(context.Background(), instanceID, func() error { return nil })
					} else {
						_, err := store.DeleteExpired(context.Background(), current.Add(2*time.Hour))
						deleted <- err
					}
				}()
				// The object-read case holds the guard in the fix; the initial metadata
				// lookup does not, so cleanup finishes and Path must revalidate afterward.
				select {
				case err := <-deleted:
					deleted <- err
				case <-time.After(100 * time.Millisecond):
				}
				close(resume)
				pathErr := <-materialized
				if pause == "initial metadata lookup" && !errors.Is(pathErr, ErrNotFound) {
					t.Errorf("Path error=%v; want deleted metadata rejection", pathErr)
				}
				if err := <-deleted; err != nil {
					t.Fatal(err)
				}
				if files := countFiles(t, dir); files != 0 {
					t.Fatalf("Path recreated %d cache file(s) after %s cleanup", files, cleanup)
				}
			})
		}
	}
}
