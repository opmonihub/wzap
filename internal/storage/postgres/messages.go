package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"wzap/internal/model"
	"wzap/internal/storage"
)

const messageColumns = `id, instance_id, message_type, recipient_jid, payload, media_id, send_status, ` +
	`COALESCE(wa_id, '') AS wa_id, COALESCE(last_error_message, '') AS last_error_message, ` +
	`COALESCE(last_error_code, '') AS last_error_code, last_error_at, ` +
	`retry_count, next_attempt_at, delivered_at, read_at, created_at, updated_at`

const listMessagesQuery = `SELECT ` + messageColumns + ` FROM message_queue ` +
	`WHERE instance_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`

const listMessagesAfterQuery = `SELECT ` + messageColumns + ` FROM message_queue ` +
	`WHERE instance_id = $1 AND (created_at, id) < (SELECT created_at, id FROM message_queue WHERE id = $2) ` +
	`ORDER BY created_at DESC, id DESC LIMIT $3`

const claimQueuedSelectQuery = `SELECT ` + messageColumns + ` FROM message_queue ` +
	`WHERE send_status = 'queued' AND (next_attempt_at IS NULL OR next_attempt_at <= now()) ` +
	`ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1`

const claimQueuedUpdateQuery = `UPDATE message_queue SET send_status = 'sending', updated_at = now() ` +
	`WHERE id = ANY($1) RETURNING ` + messageColumns

// MessageRepository is the pgx-backed storage.MessageRepository.
type MessageRepository struct {
	pool *pgxpool.Pool
}

var _ storage.MessageRepository = (*MessageRepository)(nil)

// NewMessageRepository returns a message repository backed by pool.
func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

// Create persists a new outbound message and returns it with database
// timestamps. A media reference must point at a media row of the same
// instance; a missing or cross-instance media is ErrNotFound.
func (r *MessageRepository) Create(ctx context.Context, message model.OutboundMessage) (*model.OutboundMessage, error) {
	if message.MediaID != nil {
		var exists bool
		if err := r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM media WHERE id = $1 AND instance_id = $2)`,
			*message.MediaID, message.InstanceID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("create message: check media: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("create message: %w", storage.ErrNotFound)
		}
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO message_queue (id, instance_id, message_type, recipient_jid, payload, media_id, send_status, wa_id)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE(NULLIF($7, ''), 'queued'), NULLIF($8, ''))
		RETURNING `+messageColumns,
		message.ID, message.InstanceID, message.Type, message.RecipientJID,
		message.Payload, message.MediaID, message.Status, message.WhatsAppMessageID,
	)

	created, err := scanMessage(row)
	if err != nil {
		return nil, mapMessageError("create message", err)
	}
	return created, nil
}

// Get returns the message with the given id or storage.ErrNotFound.
func (r *MessageRepository) Get(ctx context.Context, id uuid.UUID) (*model.OutboundMessage, error) {
	message, err := scanMessage(r.pool.QueryRow(ctx,
		`SELECT `+messageColumns+` FROM message_queue WHERE id = $1`, id))
	if err != nil {
		return nil, mapMessageError("get message", err)
	}
	return message, nil
}

// ListByInstance returns up to limit messages of instanceID ordered by
// created_at descending and the cursor to fetch the next page.
func (r *MessageRepository) ListByInstance(
	ctx context.Context, instanceID uuid.UUID, limit int, cursor string,
) ([]model.OutboundMessage, string, error) {
	if limit <= 0 {
		return []model.OutboundMessage{}, "", nil
	}

	query := listMessagesQuery
	args := []any{instanceID, limit + 1}
	if cursor != "" {
		cursorID, err := uuid.Parse(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("list messages: %w", storage.ErrInvalidCursor)
		}
		query = listMessagesAfterQuery
		args = []any{instanceID, cursorID, limit + 1}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	messages := []model.OutboundMessage{}
	for rows.Next() {
		var message model.OutboundMessage
		if err := scanMessageRow(rows, &message); err != nil {
			return nil, "", fmt.Errorf("list messages: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("list messages: %w", err)
	}

	nextCursor := ""
	if len(messages) > limit {
		messages = messages[:limit]
		nextCursor = messages[limit-1].ID.String()
	}
	return messages, nextCursor, nil
}

// ClaimQueued atomically moves up to limit due queued messages to sending,
// oldest first, skipping rows locked by other transactions.
func (r *MessageRepository) ClaimQueued(ctx context.Context, limit int) ([]model.OutboundMessage, error) {
	if limit <= 0 {
		return []model.OutboundMessage{}, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("claim queued messages: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, claimQueuedSelectQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("claim queued messages: select: %w", err)
	}

	ids := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var message model.OutboundMessage
		if err := scanMessageRow(rows, &message); err != nil {
			rows.Close()
			return nil, fmt.Errorf("claim queued messages: %w", err)
		}
		ids = append(ids, message.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim queued messages: %w", err)
	}

	if len(ids) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("claim queued messages: commit: %w", err)
		}
		return []model.OutboundMessage{}, nil
	}

	updated, err := tx.Query(ctx, claimQueuedUpdateQuery, ids)
	if err != nil {
		return nil, fmt.Errorf("claim queued messages: update: %w", err)
	}

	claimedByID := make(map[uuid.UUID]model.OutboundMessage, len(ids))
	for updated.Next() {
		message, err := scanMessage(updated)
		if err != nil {
			updated.Close()
			return nil, fmt.Errorf("claim queued messages: %w", err)
		}
		claimedByID[message.ID] = *message
	}
	updated.Close()
	if err := updated.Err(); err != nil {
		return nil, fmt.Errorf("claim queued messages: %w", err)
	}

	claimed := make([]model.OutboundMessage, 0, len(ids))
	for _, id := range ids {
		message, ok := claimedByID[id]
		if !ok {
			return nil, fmt.Errorf("claim queued messages: message %s missing from update", id)
		}
		claimed = append(claimed, message)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("claim queued messages: commit: %w", err)
	}
	return claimed, nil
}

// MarkSent confirms the WhatsApp identifier and its status event together.
func (r *MessageRepository) MarkSent(ctx context.Context, id uuid.UUID, whatsAppMessageID string, event model.OutboxEvent) (bool, error) {
	return r.markTerminal(ctx, id, "sent", whatsAppMessageID, "", event)
}

// MarkFailed confirms the definitive failure and its status event together.
func (r *MessageRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, event model.OutboxEvent) (bool, error) {
	return r.markTerminal(ctx, id, "failed", "", errMsg, event)
}

func (r *MessageRepository) markTerminal(ctx context.Context, id uuid.UUID, status, whatsAppMessageID, errMsg string, event model.OutboxEvent) (bool, error) {
	if event.ID == uuid.Nil || event.Subject == "" || len(event.Envelope) == 0 {
		return false, fmt.Errorf("mark message %s: status event is required", status)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("mark message %s: begin: %w", status, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var storedStatus, storedWhatsAppID, storedError string
	if err := tx.QueryRow(ctx, `SELECT send_status, COALESCE(wa_id, ''), COALESCE(last_error_message, '')
		FROM message_queue WHERE id = $1 FOR UPDATE`, id).Scan(&storedStatus, &storedWhatsAppID, &storedError); err != nil {
		return false, mapMessageError("mark message "+status, err)
	}
	if storedStatus == "sent" || storedStatus == "failed" {
		if storedStatus == status && ((status == "sent" && storedWhatsAppID == whatsAppMessageID) || (status == "failed" && storedError == errMsg)) {
			// The relay can already have deleted the pending event. A
			// matching terminal outcome still confirms a retry after an
			// uncertain commit without inserting or publishing it again.
			return false, nil
		}
		return false, fmt.Errorf("mark message %s: %w", status, storage.ErrMessageOutcomeConflict)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE message_queue
		SET send_status = $2,
		    wa_id = CASE WHEN $2 = 'sent' THEN NULLIF($3, '') ELSE wa_id END,
		    last_error_code = CASE WHEN NULLIF($4, '') IS NULL THEN NULL ELSE 'send_failed' END,
		    last_error_message = NULLIF($4, ''),
		    last_error_at = CASE WHEN NULLIF($4, '') IS NULL THEN NULL ELSE now() END,
		    updated_at = now()
		WHERE id = $1`, id, status, whatsAppMessageID, errMsg); err != nil {
		return false, fmt.Errorf("mark message %s: update: %w", status, err)
	}
	if err := enqueueEvent(ctx, tx, event.ID, event.Subject, event.Envelope); err != nil {
		return false, fmt.Errorf("mark message %s: %w", status, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("mark message %s: commit: %w", status, err)
	}
	return true, nil
}

// MarkRetrying moves the message back to queued for a later attempt.
func (r *MessageRepository) MarkRetrying(ctx context.Context, id uuid.UUID, errMsg string, nextAttemptAt time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE message_queue
		SET send_status = 'queued', retry_count = retry_count + 1,
		    last_error_code = CASE WHEN NULLIF($2, '') IS NULL THEN NULL ELSE 'send_retry' END,
		    last_error_message = NULLIF($2, ''),
		    last_error_at = CASE WHEN NULLIF($2, '') IS NULL THEN NULL ELSE now() END,
		    next_attempt_at = $3, updated_at = now()
		WHERE id = $1`, id, errMsg, nextAttemptAt)
	if err != nil {
		return fmt.Errorf("mark message retrying: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark message retrying: %w", storage.ErrNotFound)
	}
	return nil
}

// UpdateReceipt records a delivery or read milestone by instance and WhatsApp message id.
func (r *MessageRepository) UpdateReceipt(
	ctx context.Context, instanceID uuid.UUID, whatsAppMessageID, status string, at time.Time,
) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE message_queue
		SET delivered_at = COALESCE(delivered_at, $4),
		    read_at = CASE WHEN $3 IN ('read', 'played') THEN COALESCE(read_at, $4) ELSE read_at END,
		    updated_at = now()
		WHERE instance_id = $1 AND wa_id = $2 AND $3 IN ('delivered', 'read', 'played')`,
		instanceID, whatsAppMessageID, status, at)
	if err != nil {
		return false, fmt.Errorf("update message receipt: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RequeueStuck moves sending messages last updated before olderThan back to
// queued, excluding active claims, and returns how many were recovered.
func (r *MessageRepository) RequeueStuck(ctx context.Context, olderThan time.Time, activeIDs []uuid.UUID) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE message_queue
		SET send_status = 'queued', updated_at = now()
		WHERE send_status = 'sending' AND updated_at < $1
		    AND NOT (id = ANY(COALESCE($2::uuid[], '{}'::uuid[])))`, olderThan, activeIDs)
	if err != nil {
		return 0, fmt.Errorf("requeue stuck messages: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanMessage(scanner rowScanner) (*model.OutboundMessage, error) {
	var message model.OutboundMessage
	if err := scanMessageRow(scanner, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

func scanMessageRow(scanner rowScanner, message *model.OutboundMessage) error {
	return scanner.Scan(
		&message.ID, &message.InstanceID, &message.Type, &message.RecipientJID,
		&message.Payload, &message.MediaID, &message.Status, &message.WhatsAppMessageID,
		&message.LastError, &message.LastErrorCode, &message.LastErrorAt,
		&message.Attempts, &message.NextAttemptAt, &message.DeliveredAt, &message.ReadAt,
		&message.CreatedAt, &message.UpdatedAt,
	)
}

func mapMessageError(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, storage.ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return fmt.Errorf("%s: %w", op, storage.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", op, err)
}
