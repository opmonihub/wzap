// Package media stores message media bytes in an object store (MinIO) with
// metadata in the database, serving the content until its TTL expires. The
// local data dir keeps a disposable cache of fetched objects for callers
// that need a filesystem path (the whatsmeow session store).
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
	"wzap/internal/storage"
)

// Errors reported by the storage and mapped to HTTP status codes by the
// handlers.
var (
	// ErrNotFound reports that the requested media does not exist or its
	// content is gone.
	ErrNotFound = errors.New("media not found")
	// ErrExpired reports that the media exists but its retention window ended.
	ErrExpired = errors.New("media expired")
	// ErrEmpty reports that the content to store was empty.
	ErrEmpty = errors.New("media is empty")
	// ErrTooLarge reports that the content exceeds the configured limit.
	ErrTooLarge = errors.New("media exceeds the size limit")
)

// Outbound media kinds, used by the upload handler and the outbound sender.
const (
	KindImage    = "image"
	KindVideo    = "video"
	KindAudio    = "audio"
	KindDocument = "document"
)

// allowedMimes are the media types accepted on upload, mirroring the formats
// supported by WhatsApp for images, video, audio and documents.
var allowedMimes = map[string]struct{}{
	"image/jpeg":                    {},
	"image/png":                     {},
	"image/webp":                    {},
	"video/mp4":                     {},
	"video/3gpp":                    {},
	"audio/aac":                     {},
	"audio/amr":                     {},
	"audio/mpeg":                    {},
	"audio/mp4":                     {},
	"audio/ogg":                     {},
	"application/pdf":               {},
	"text/plain":                    {},
	"application/msword":            {},
	"application/vnd.ms-excel":      {},
	"application/vnd.ms-powerpoint": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
}

// AllowedMime reports whether mimetype is accepted on upload. The comparison
// ignores case and surrounding spaces.
func AllowedMime(mimetype string) bool {
	_, ok := allowedMimes[strings.ToLower(strings.TrimSpace(mimetype))]
	return ok
}

// Kind returns the outbound kind of an allowed mimetype: every image goes as
// an image, every video as a video, every audio as an audio and the remaining
// allowed formats (documents) go as documents. An unaccepted mimetype has no
// kind, matching AllowedMime.
func Kind(mimetype string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(mimetype))
	if _, ok := allowedMimes[normalized]; !ok {
		return "", false
	}
	switch {
	case strings.HasPrefix(normalized, "image/"):
		return KindImage, true
	case strings.HasPrefix(normalized, "video/"):
		return KindVideo, true
	case strings.HasPrefix(normalized, "audio/"):
		return KindAudio, true
	default:
		return KindDocument, true
	}
}

// Storage stores media objects in an object store and records their metadata
// through a repository. The data dir keeps a cache of fetched objects so a
// consumer that needs a real file (the whatsmeow session store) gets one;
// the cache is disposable and never the authority.
//
// Ordering rules: the object is uploaded before its row exists and removed
// when the row cannot be created; the row's object_deleted_at is marked only
// after the remote deletion is confirmed, so metadata survives expiration.
type Storage struct {
	dir      string
	repo     storage.MediaRepository
	objects  Objects
	maxBytes int64
	ttl      time.Duration
	now      func() time.Time
}

// NewStorage returns a storage rooted at dir that accepts up to maxBytes per
// media and expires every saved media after ttl. With objects nil the
// storage serves a filesystem-only mode: objects are written under dir and
// reads come from it — used by tests and deployments still mid-migration.
func NewStorage(dir string, repo storage.MediaRepository, maxBytes int64, ttl time.Duration) *Storage {
	return &Storage{dir: dir, repo: repo, maxBytes: maxBytes, ttl: ttl, now: time.Now}
}

// SetObjects points the storage at the configured object store. The bucket
// the store reports becomes the bucket written on every new media row.
func (s *Storage) SetObjects(objects Objects) {
	s.objects = objects
}

// Objects returns the configured object store, or nil in filesystem mode.
func (s *Storage) Objects() Objects {
	return s.objects
}

// objectKey builds the stable remote key for a media id, identical to the
// relative cache path under the data dir.
func objectKey(instanceID, id uuid.UUID) string {
	return filepath.Join("media", instanceID.String(), id.String())
}

// Save uploads data under media/<instance_id>/<media_id>, records its
// metadata and returns the stored row. Empty and over-limit content is
// rejected before anything is written; when the row cannot be created the
// uploaded object is removed, so a failed save leaves nothing behind.
func (s *Storage) Save(
	ctx context.Context, instanceID uuid.UUID, direction, messageID, mimetype, filename string, data []byte,
) (*model.Media, error) {
	if len(data) == 0 {
		return nil, ErrEmpty
	}
	if int64(len(data)) > s.maxBytes {
		return nil, fmt.Errorf("%w: %d bytes, limit %d", ErrTooLarge, len(data), s.maxBytes)
	}

	id := uuid.New()
	key := objectKey(instanceID, id)
	bucket := ""

	if s.objects != nil {
		bucket = s.objects.Bucket()
		if err := s.objects.Put(ctx, bucket, key, data, mimetype); err != nil {
			return nil, fmt.Errorf("save media: %w", err)
		}
	} else {
		if err := writeMediaFile(filepath.Join(s.dir, key), data); err != nil {
			return nil, fmt.Errorf("save media: %w", err)
		}
		bucket = "local"
	}

	now := s.now()
	sum := sha256.Sum256(data)
	created, err := s.repo.Create(ctx, model.Media{
		ID:         id,
		InstanceID: instanceID,
		Direction:  direction,
		MessageID:  messageID,
		Mimetype:   mimetype,
		Filename:   sanitizeFilename(filename),
		SizeBytes:  int64(len(data)),
		Bucket:     bucket,
		ObjectKey:  key,
		SHA256:     hex.EncodeToString(sum[:]),
		CreatedAt:  now,
		ExpiresAt:  now.Add(s.ttl),
	})
	if err != nil {
		// The row is the only reference to the content: drop the orphan.
		s.discardContent(ctx, model.Media{Bucket: bucket, ObjectKey: key})
		return nil, fmt.Errorf("save media: %w", mapRepoError(err))
	}
	return created, nil
}

// Open returns the content of the media with the given id along with its
// metadata. An unknown id, an expired TTL, a confirmed-deleted object or a
// missing object report ErrNotFound/ErrExpired.
func (s *Storage) Open(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.Media, error) {
	record, err := s.fetchable(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return s.openRecord(ctx, record)
}

// openRecord streams the content of an already validated record.
func (s *Storage) openRecord(ctx context.Context, record *model.Media) (io.ReadCloser, *model.Media, error) {
	if s.objects != nil {
		body, err := s.objects.Get(ctx, record.Bucket, record.ObjectKey)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, nil, fmt.Errorf("open media %s: %w", record.ID, ErrNotFound)
			}
			return nil, nil, fmt.Errorf("open media %s: %w", record.ID, err)
		}
		return body, record, nil
	}

	path, err := s.path(record.ObjectKey)
	if err != nil {
		return nil, nil, fmt.Errorf("open media %s: %w", record.ID, err)
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("open media %s: %w", record.ID, ErrNotFound)
		}
		return nil, nil, fmt.Errorf("open media %s: %w", record.ID, err)
	}
	return file, record, nil
}

// Path returns the absolute filesystem path of the media with the given id
// along with its metadata, materializing the object into the data-dir cache
// when the storage is object-backed. It lets a consumer hand the file to
// something that reads it directly (the whatsmeow session store) instead of
// streaming it through this storage.
//
// The cached file is disposable: it can be recreated from the object at any
// time while the row is alive and unexpired. The expiry and deleted checks
// match Open.
//
// TOCTOU note: the object can still vanish between this call and the
// consumer's read; the media sender surfaces a session read failure that the
// outbox retries.
func (s *Storage) Path(ctx context.Context, id uuid.UUID) (string, *model.Media, error) {
	record, err := s.fetchable(ctx, id)
	if err != nil {
		return "", nil, err
	}

	cachePath, err := s.path(record.ObjectKey)
	if err != nil {
		return "", nil, fmt.Errorf("open media %s: %w", id, err)
	}

	if s.objects == nil {
		// Filesystem mode: the stored file is already the path.
		if _, err := os.Stat(cachePath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return "", nil, fmt.Errorf("open media %s: %w", id, ErrNotFound)
			}
			return "", nil, fmt.Errorf("open media %s: %w", id, err)
		}
		return cachePath, record, nil
	}

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, record, nil
	}

	body, rec, err := s.openRecord(ctx, record)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(body)
	if err != nil {
		return "", nil, fmt.Errorf("open media %s: %w", id, err)
	}
	if err := writeMediaFile(cachePath, data); err != nil {
		return "", nil, fmt.Errorf("cache media %s: %w", id, err)
	}
	return cachePath, rec, nil
}

// fetchable returns the record when it exists, is unexpired and its object
// is not confirmed deleted.
func (s *Storage) fetchable(ctx context.Context, id uuid.UUID) (*model.Media, error) {
	record, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("open media %s: %w", id, mapRepoError(err))
	}
	if record.ObjectDeletedAt != nil {
		return nil, fmt.Errorf("open media %s: %w", id, ErrNotFound)
	}
	if !s.now().Before(record.ExpiresAt) {
		return nil, fmt.Errorf("open media %s: %w", id, ErrExpired)
	}
	return record, nil
}

// DeleteByInstance removes every media of an instance: remote objects first,
// then the rows, then the cache directory. When an object cannot be removed
// its row is kept, so a retry can finish the job instead of leaking content.
func (s *Storage) DeleteByInstance(ctx context.Context, instanceID uuid.UUID) error {
	records, err := s.repo.ListByInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("delete instance media %s: list: %w", instanceID, err)
	}

	var failures []error
	for _, record := range records {
		if err := s.removeContent(ctx, record); err != nil {
			failures = append(failures, err)
		}
	}
	if err := errors.Join(failures...); err != nil {
		return fmt.Errorf("delete instance media %s: remove objects: %w", instanceID, err)
	}

	if _, err := s.repo.DeleteByInstance(ctx, instanceID); err != nil {
		return fmt.Errorf("delete instance media %s: delete rows: %w", instanceID, err)
	}

	// Drop the instance cache directory, including empty parents and temp
	// files left by a crash.
	if err := os.RemoveAll(filepath.Join(s.dir, "media", instanceID.String())); err != nil {
		return fmt.Errorf("delete instance media %s: remove directory: %w", instanceID, err)
	}
	return nil
}

// DeleteExpired deletes the remote objects of expired media and marks each
// row's object_deleted_at after the removal is confirmed, preserving the
// metadata row. An already absent object counts as confirmed (remote delete
// is idempotent); a failed deletion keeps the row unmarked so a later pass
// retries. Returns how many rows were marked.
func (s *Storage) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	records, err := s.repo.ListExpired(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired media: list: %w", err)
	}

	removed := 0
	var failures []error
	for _, record := range records {
		if err := s.removeContent(ctx, record); err != nil {
			failures = append(failures, err)
			continue
		}
		markedAt := s.now()
		if err := s.repo.MarkObjectDeleted(ctx, record.ID, markedAt); err != nil && !errors.Is(err, storage.ErrNotFound) {
			failures = append(failures, fmt.Errorf("mark media %s deleted: %w", record.ID, err))
			continue
		}
		_ = s.removeCacheFile(record.ObjectKey)
		removed++
	}
	if err := errors.Join(failures...); err != nil {
		return removed, fmt.Errorf("delete expired media: %w", err)
	}
	return removed, nil
}

// discardContent removes an orphan object/file after a failed row create;
// removal failures are best-effort here because nothing references it yet.
func (s *Storage) discardContent(ctx context.Context, record model.Media) {
	if err := s.removeContent(ctx, record); err != nil {
		// Orphan object left behind; the object store's own lifecycle is the
		// backstop and the caller already got the create error.
		_ = err
	}
}

// removeContent deletes the remote object (or, in filesystem mode, the
// stored file). The object lives in the row's bucket: the row is the
// metadata authority, so a deployment whose WZAP_S3_BUCKET changed never
// deletes from (and corrupts) the wrong bucket. An already absent object is
// a confirmed removal.
func (s *Storage) removeContent(ctx context.Context, record model.Media) error {
	if s.objects != nil {
		return s.objects.Delete(ctx, record.Bucket, record.ObjectKey)
	}
	return s.removeCacheFile(record.ObjectKey)
}

// removeCacheFile drops one cached file under the data dir; a missing file
// is not an error.
func (s *Storage) removeCacheFile(rel string) error {
	path, err := s.path(rel)
	if err != nil {
		return fmt.Errorf("remove media file %q: %w", rel, err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove media file %q: %w", rel, err)
	}
	return nil
}

// MigrateLocalFiles uploads every media whose content still lives only as a
// local file under the data dir — the object is absent from the bucket
// recorded on the row — verifying the SHA-256 of the local content before
// accepting it. The local file stays in place: it is the recovery path until
// the cutover is rehearsed and the cleanup step of the deployment removes
// it. Returns the count of uploaded objects. It is a no-op in filesystem
// mode (nothing to migrate).
//
// Buckets: the existence probe runs on the bucket recorded on the row (the
// metadata authority), so a row whose object already lives in its own bucket
// is skipped untouched. When the object is absent, the upload goes into the
// configured bucket — the one the deployment provisions (EnsureBucket) —
// and the row is rewritten to record where the object landed, exactly like
// Save. A filesystem-era row (bucket "local") or a row backfilled with
// another deployment's bucket has no object yet; uploading into its stale
// bucket would strand media behind a bucket nobody provisioned.
//
// Error behavior, per row: any error — an absent or unreachable bucket,
// access denial, a failed upload, a failed row rewrite, a local file that
// exists but cannot be read (permission denied, I/O error, wrong file
// type), an object key that escapes the data dir, or a row with no copy at
// all (no local file and no object in the recorded bucket) — is aggregated
// as a per-row failure, returned joined with the partial count, while the
// loop always continues to the remaining rows. One stuck row can therefore
// neither abort the batch nor trap the migration in a re-upload loop, and
// the command's exit status cannot turn green while media it was
// responsible for stays unreachable in the object store. A missing local
// file is NOT an error when the row's bucket holds the object (already
// migrated; the cleanup removed the file) — the probe decides, so a row is
// only skipped with a confirmed copy. Reruns are idempotent and converge: a
// rewritten row is recognized by the probe and skipped, a row whose rewrite
// failed is re-uploaded (same key, same bytes) and rewritten again, and the
// local file is kept as the recovery path throughout.
func (s *Storage) MigrateLocalFiles(ctx context.Context, onProgress func(done, total int)) (int, error) {
	if s.objects == nil {
		return 0, nil
	}
	instances, err := s.repo.ListInstancesWithMedia(ctx)
	if err != nil {
		return 0, fmt.Errorf("media migration: list instances: %w", err)
	}

	var pending []model.Media
	for _, instanceID := range instances {
		records, err := s.repo.ListByInstance(ctx, instanceID)
		if err != nil {
			return 0, fmt.Errorf("media migration: list instance %s: %w", instanceID, err)
		}
		pending = append(pending, records...)
	}

	migrated := 0
	var failures []error
	for i, record := range pending {
		if onProgress != nil {
			onProgress(i, len(pending))
		}
		if record.ObjectDeletedAt != nil {
			continue
		}
		localPath, err := s.path(record.ObjectKey)
		if err != nil {
			// An escaping key is row corruption, not a missing file: the
			// migration reports the row instead of silently dropping it.
			failures = append(failures, fmt.Errorf("media %s: resolve local path: %w", record.ID, err))
			continue
		}
		data, readErr := os.ReadFile(localPath)
		if readErr != nil {
			if !errors.Is(readErr, fs.ErrNotExist) {
				// The file is there but cannot be read (permission denied,
				// I/O error, wrong file type): its bytes cannot be verified
				// or uploaded, so the row fails — skipping it would release
				// a cutover that leaves the media unreachable.
				failures = append(failures, fmt.Errorf("media %s: read local file: %w", record.ID, readErr))
				continue
			}
			// No local file: either already migrated (the cleanup removed
			// it) or never transferred. The row bucket — the metadata
			// authority — decides: object present means migrated and the
			// row is skipped; object absent means the row has no copy
			// anywhere and the migration fails for it instead of letting
			// the media vanish from the result.
			exists, probeErr := s.objects.Exists(ctx, record.Bucket, record.ObjectKey)
			if probeErr != nil {
				failures = append(failures, fmt.Errorf("media %s: %w", record.ID, probeErr))
				continue
			}
			if !exists {
				failures = append(failures, fmt.Errorf("media %s: no local file and no object in bucket %q", record.ID, record.Bucket))
			}
			continue
		}
		if record.SHA256 != "" {
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != record.SHA256 {
				failures = append(failures, fmt.Errorf("media %s: local checksum mismatch, skipping", record.ID))
				continue
			}
		}
		// The probe stays on the row bucket: a row whose object already
		// lives in its own bucket is left exactly as it is. A missing object
		// (404/NotFound) is uploaded into the configured bucket — the one
		// the deployment provisions — and the row is rewritten to record
		// where the object landed, so the next run's probe (now on the
		// upload bucket) finds it and the migration converges instead of
		// re-uploading. Any other error defers the row to a rerun without
		// aborting the batch (see the doc comment above).
		exists, err := s.objects.Exists(ctx, record.Bucket, record.ObjectKey)
		if err != nil {
			failures = append(failures, fmt.Errorf("media %s: %w", record.ID, err))
			continue
		}
		if exists {
			continue
		}
		uploadBucket := s.objects.Bucket()
		if err := s.objects.Put(ctx, uploadBucket, record.ObjectKey, data, record.Mimetype); err != nil {
			failures = append(failures, fmt.Errorf("media %s: %w", record.ID, err))
			continue
		}
		if err := s.repo.SetBucket(ctx, record.ID, uploadBucket); err != nil {
			failures = append(failures, fmt.Errorf("media %s: %w", record.ID, err))
			continue
		}
		migrated++
	}
	if err := errors.Join(failures...); err != nil {
		return migrated, fmt.Errorf("media migration: %w", err)
	}
	return migrated, nil
}

// path resolves an object key against the data dir, refusing one that
// escapes it. Keys come from our own rows, so an escaping value means
// corruption and is reported as not found instead of touching the disk.
func (s *Storage) path(key string) (string, error) {
	clean := filepath.Clean(key)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, clean), nil
}

// writeMediaFile writes data at path atomically, so a crash never leaves a
// partial file under the final name.
func writeMediaFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".media-*")
	if err != nil {
		return fmt.Errorf("create temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename into %s: %w", path, err)
	}
	return nil
}

// sanitizeFilename keeps only the base name so a hostile name cannot smuggle
// a path into the metadata; the object key never uses it.
func sanitizeFilename(filename string) string {
	name := strings.TrimSpace(filename)
	if name == "" {
		return ""
	}
	return filepath.Base(name)
}

// mapRepoError translates the storage sentinel into the media one, so callers
// do not depend on the repository package.
func mapRepoError(err error) error {
	if errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
