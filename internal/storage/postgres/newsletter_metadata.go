package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"wzap/internal/model"
	"wzap/internal/storage"
)

const newsletterMetadataColumns = `instance_id, channel_jid, title, description, follower_count, updated_at`

// NewsletterMetadataRepository is the pgx-backed
// storage.NewsletterMetadataRepository.
type NewsletterMetadataRepository struct {
	pool *pgxpool.Pool
}

var _ storage.NewsletterMetadataRepository = (*NewsletterMetadataRepository)(nil)

// NewNewsletterMetadataRepository returns a newsletter metadata repository
// backed by pool.
func NewNewsletterMetadataRepository(pool *pgxpool.Pool) *NewsletterMetadataRepository {
	return &NewsletterMetadataRepository{pool: pool}
}

// Upsert stores the refreshed metadata of a channel, refreshing updated_at,
// and returns the stored row.
func (r *NewsletterMetadataRepository) Upsert(ctx context.Context, meta model.NewsletterMetadata) (model.NewsletterMetadata, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO channel_metadata (instance_id, channel_jid, title, description, follower_count, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (instance_id, channel_jid)
		DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description,
			follower_count = EXCLUDED.follower_count, updated_at = now()
		RETURNING `+newsletterMetadataColumns,
		meta.InstanceID, meta.ChannelJID, meta.Title, meta.Description, meta.FollowerCount,
	)

	stored, err := scanNewsletterMetadata(row)
	if err != nil {
		return model.NewsletterMetadata{}, fmt.Errorf("upsert newsletter metadata: %w", err)
	}
	return stored, nil
}

// Get returns the cached metadata of a channel or storage.ErrNotFound.
func (r *NewsletterMetadataRepository) Get(ctx context.Context, instanceID uuid.UUID, channelJID string) (model.NewsletterMetadata, error) {
	stored, err := scanNewsletterMetadata(r.pool.QueryRow(ctx,
		`SELECT `+newsletterMetadataColumns+` FROM channel_metadata WHERE instance_id = $1 AND channel_jid = $2`,
		instanceID, channelJID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.NewsletterMetadata{}, fmt.Errorf("get newsletter metadata: %w", storage.ErrNotFound)
		}
		return model.NewsletterMetadata{}, fmt.Errorf("get newsletter metadata: %w", err)
	}
	return stored, nil
}

// ListByInstance returns every cached row of an instance ordered by channel
// JID.
func (r *NewsletterMetadataRepository) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]model.NewsletterMetadata, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+newsletterMetadataColumns+` FROM channel_metadata WHERE instance_id = $1 ORDER BY channel_jid ASC`,
		instanceID)
	if err != nil {
		return nil, fmt.Errorf("list newsletter metadata: %w", err)
	}
	defer rows.Close()

	metas := []model.NewsletterMetadata{}
	for rows.Next() {
		meta, err := scanNewsletterMetadata(rows)
		if err != nil {
			return nil, fmt.Errorf("list newsletter metadata: %w", err)
		}
		metas = append(metas, meta)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list newsletter metadata: %w", err)
	}
	return metas, nil
}

// DeleteByInstance removes every cached row of an instance.
func (r *NewsletterMetadataRepository) DeleteByInstance(ctx context.Context, instanceID uuid.UUID) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM channel_metadata WHERE instance_id = $1`, instanceID)
	if err != nil {
		return 0, fmt.Errorf("delete newsletter metadata: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanNewsletterMetadata(scanner rowScanner) (model.NewsletterMetadata, error) {
	var meta model.NewsletterMetadata
	if err := scanner.Scan(
		&meta.InstanceID, &meta.ChannelJID, &meta.Title, &meta.Description,
		&meta.FollowerCount, &meta.UpdatedAt,
	); err != nil {
		return model.NewsletterMetadata{}, err
	}
	return meta, nil
}
