package instance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"wzap/internal/media"
	"wzap/internal/model"
	"wzap/internal/session/sessiontest"
	"wzap/internal/storage"
)

// cascadingMediaRepo models the media FK: instance deletion cascades existing
// rows and rejects later inserts, just as Postgres does.
type cascadingMediaRepo struct {
	storage.MediaRepository
	mu      sync.Mutex
	rows    map[uuid.UUID]model.Media
	deleted bool
}

func (r *cascadingMediaRepo) Create(_ context.Context, row model.Media) (*model.Media, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted {
		return nil, storage.ErrNotFound
	}
	r.rows[row.ID] = row
	return &row, nil
}
func (r *cascadingMediaRepo) ListByInstance(_ context.Context, id uuid.UUID) ([]model.Media, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := []model.Media{}
	for _, row := range r.rows {
		if row.InstanceID == id {
			rows = append(rows, row)
		}
	}
	return rows, nil
}
func (r *cascadingMediaRepo) DeleteByInstance(_ context.Context, id uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for key, row := range r.rows {
		if row.InstanceID == id {
			delete(r.rows, key)
			n++
		}
	}
	return n, nil
}

type pausedInstanceDeleteRepo struct {
	*fakeRepo
	media   *cascadingMediaRepo
	entered chan struct{}
	release chan struct{}
}

func (r *pausedInstanceDeleteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	close(r.entered)
	<-r.release
	r.media.mu.Lock()
	r.media.deleted = true
	clear(r.media.rows)
	r.media.mu.Unlock()
	return r.fakeRepo.Delete(ctx, id)
}
func TestServiceDeleteConcurrentMediaSaveDoesNotOrphanFile(t *testing.T) {
	id := uuid.New()
	metadata := &cascadingMediaRepo{rows: map[uuid.UUID]model.Media{}}
	repo := &pausedInstanceDeleteRepo{fakeRepo: newFakeRepo(model.Instance{ID: id, Name: "deleting"}), media: metadata, entered: make(chan struct{}), release: make(chan struct{})}
	dir := t.TempDir()
	mediaStorage := media.NewStorage(dir, metadata, 1024, time.Hour)
	svc := NewService(repo, sessiontest.New(nil), mediaStorage, nil, nil, zerolog.Nop())
	deletion := make(chan error, 1)
	go func() { deletion <- svc.Delete(context.Background(), id) }()
	<-repo.entered // media cleanup is finished, but instance cascade has not run.
	saved := make(chan error, 1)
	go func() {
		_, err := mediaStorage.Save(context.Background(), id, "inbound", "", "image/png", "x.png", []byte("data"))
		saved <- err
	}()
	select {
	case err := <-saved:
		saved <- err
	case <-time.After(100 * time.Millisecond):
	}
	close(repo.release)
	if err := <-deletion; err != nil {
		t.Fatal(err)
	}
	if err := <-saved; !errors.Is(err, media.ErrNotFound) {
		t.Errorf("concurrent Save error=%v; want deleted instance rejection", err)
	}
	files := 0
	if err := filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if files != 0 {
		t.Fatalf("instance deletion left %d orphan file(s)", files)
	}
}
