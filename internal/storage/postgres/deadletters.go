package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"wzap/internal/storage"
)

// deadLetterRetentionPerInstance bounds the dead-letter tail kept per
// instance: every record trims older rows beyond the newest N, so a failing
// endpoint cannot grow the table without bound.
const deadLetterRetentionPerInstance = 500

// DeadLetterRepository is the pgx-backed storage.DeadLetterRepository.
type DeadLetterRepository struct {
	pool *pgxpool.Pool
}

var _ storage.DeadLetterRepository = (*DeadLetterRepository)(nil)

// NewDeadLetterRepository returns a dead-letter repository backed by pool.
func NewDeadLetterRepository(pool *pgxpool.Pool) *DeadLetterRepository {
	return &DeadLetterRepository{pool: pool}
}

// RecordDeadLetter stores one exhausted webhook delivery. A repeated event_id
// (replayed event re-recorded) is a silent no-op. The per-instance tail is
// trimmed past the retention bound in the same transaction.
func (r *DeadLetterRepository) RecordDeadLetter(ctx context.Context, instanceID, eventID uuid.UUID, eventType string, payload []byte, attempts int, lastError string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("record webhook dead letter: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO webhook_dead_letters (instance_id, event_id, event_type, payload, attempts, last_error)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
		ON CONFLICT (event_id) DO NOTHING`,
		instanceID, eventID, eventType, string(payload), attempts, lastError,
	); err != nil {
		return fmt.Errorf("record webhook dead letter: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM webhook_dead_letters
		WHERE instance_id = $1 AND id NOT IN (
			SELECT id FROM webhook_dead_letters
			WHERE instance_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		)`, instanceID, deadLetterRetentionPerInstance,
	); err != nil {
		return fmt.Errorf("trim webhook dead letters: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("record webhook dead letter: %w", err)
	}
	return nil
}
