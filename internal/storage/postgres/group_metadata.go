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

const groupMetadataColumns = `instance_id, group_jid, name, description, participant_count, updated_at`

// GroupMetadataRepository is the pgx-backed storage.GroupMetadataRepository.
type GroupMetadataRepository struct {
	pool *pgxpool.Pool
}

var _ storage.GroupMetadataRepository = (*GroupMetadataRepository)(nil)

// NewGroupMetadataRepository returns a group metadata repository backed by
// pool.
func NewGroupMetadataRepository(pool *pgxpool.Pool) *GroupMetadataRepository {
	return &GroupMetadataRepository{pool: pool}
}

// Upsert stores the refreshed metadata of a group, refreshing updated_at, and
// returns the stored row.
func (r *GroupMetadataRepository) Upsert(ctx context.Context, meta model.GroupMetadata) (model.GroupMetadata, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO group_metadata (instance_id, group_jid, name, description, participant_count, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (instance_id, group_jid)
		DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description,
			participant_count = EXCLUDED.participant_count, updated_at = now()
		RETURNING `+groupMetadataColumns,
		meta.InstanceID, meta.GroupJID, meta.Name, meta.Description, meta.ParticipantCount,
	)

	stored, err := scanGroupMetadata(row)
	if err != nil {
		return model.GroupMetadata{}, fmt.Errorf("upsert group metadata: %w", err)
	}
	return stored, nil
}

// Get returns the cached metadata of a group or storage.ErrNotFound.
func (r *GroupMetadataRepository) Get(ctx context.Context, instanceID uuid.UUID, groupJID string) (model.GroupMetadata, error) {
	stored, err := scanGroupMetadata(r.pool.QueryRow(ctx,
		`SELECT `+groupMetadataColumns+` FROM group_metadata WHERE instance_id = $1 AND group_jid = $2`,
		instanceID, groupJID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.GroupMetadata{}, fmt.Errorf("get group metadata: %w", storage.ErrNotFound)
		}
		return model.GroupMetadata{}, fmt.Errorf("get group metadata: %w", err)
	}
	return stored, nil
}

// DeleteByInstance removes every cached row of an instance.
func (r *GroupMetadataRepository) DeleteByInstance(ctx context.Context, instanceID uuid.UUID) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM group_metadata WHERE instance_id = $1`, instanceID)
	if err != nil {
		return 0, fmt.Errorf("delete group metadata: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanGroupMetadata(scanner rowScanner) (model.GroupMetadata, error) {
	var meta model.GroupMetadata
	if err := scanner.Scan(
		&meta.InstanceID, &meta.GroupJID, &meta.Name, &meta.Description,
		&meta.ParticipantCount, &meta.UpdatedAt,
	); err != nil {
		return model.GroupMetadata{}, err
	}
	return meta, nil
}
