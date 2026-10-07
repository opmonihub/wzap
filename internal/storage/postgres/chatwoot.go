package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	chatwootcfg "wzap/internal/chatwoot/config"
	"wzap/internal/model"
	"wzap/internal/storage"
)

const chatwootConfigColumns = `instance_id, is_enabled, url, account_id, token, ` +
	`inbox_name, is_sign_enabled, sign_delimiter, is_reopen_enabled, is_pending_enabled, ` +
	`is_merge_enabled, is_import_contacts, is_import_messages, import_days, is_auto_create, ` +
	`organization, logo, ignored_jids, created_at, updated_at`

const chatwootMessageColumns = `id, instance_id, message_id, wa_key, cw_id, ` +
	`conversation_id, inbox_id, chat_jid, is_read, created_at, updated_at`

// ChatwootConfigRepository is the pgx-backed storage.ChatwootConfigRepository.
// The token column holds only authenticated AES-256-GCM ciphertext
// (enc:v1:): Put seals plaintext before writing and Get opens after
// reading, so the in-memory contract stays plaintext while the database
// never holds the credential in cleartext. A nil key stores no tokens —
// writing a non-empty one is refused.
type ChatwootConfigRepository struct {
	pool     *pgxpool.Pool
	tokenKey []byte
}

// ChatwootMessageRepository is the pgx-backed storage.ChatwootMessageRepository.
type ChatwootMessageRepository struct {
	pool *pgxpool.Pool
}

var _ storage.ChatwootConfigRepository = (*ChatwootConfigRepository)(nil)
var _ storage.ChatwootMessageRepository = (*ChatwootMessageRepository)(nil)

// NewChatwootConfigRepository returns a Chatwoot config repository backed by
// pool. tokenKey (32 bytes) seals tokens at rest (see SealToken); with a nil
// key only tokenless (disabled) configs can be written.
func NewChatwootConfigRepository(pool *pgxpool.Pool, tokenKey []byte) *ChatwootConfigRepository {
	return &ChatwootConfigRepository{pool: pool, tokenKey: tokenKey}
}

// NewChatwootMessageRepository returns a Chatwoot message repository backed by pool.
func NewChatwootMessageRepository(pool *pgxpool.Pool) *ChatwootMessageRepository {
	return &ChatwootMessageRepository{pool: pool}
}

// NewChatwootRepositories returns the Chatwoot config and message repositories
// backed by the same pool. tokenKey seals config tokens at rest; with a nil
// key only tokenless (disabled) configs can be written.
func NewChatwootRepositories(pool *pgxpool.Pool, tokenKey []byte) (*ChatwootConfigRepository, *ChatwootMessageRepository) {
	return NewChatwootConfigRepository(pool, tokenKey), NewChatwootMessageRepository(pool)
}

// Get returns the Chatwoot config of an instance or storage.ErrNotFound when
// the instance was never configured. Sealed tokens are opened with the
// repository key, so callers always see plaintext.
func (r *ChatwootConfigRepository) Get(ctx context.Context, instanceID uuid.UUID) (*model.ChatwootConfig, error) {
	cfg, err := scanChatwootConfig(r.pool.QueryRow(ctx,
		`SELECT `+chatwootConfigColumns+` FROM chatwoot_configs WHERE instance_id = $1`, instanceID))
	if err != nil {
		return nil, mapChatwootError("get chatwoot config", err)
	}
	opened, err := chatwootcfg.OpenToken(cfg.Token, r.tokenKey)
	if err != nil {
		return nil, fmt.Errorf("get chatwoot config: %w", err)
	}
	cfg.Token = opened
	return cfg, nil
}

// Put upserts the Chatwoot config of an instance and returns the stored row
// with database timestamps. The token is always sealed before writing (a
// non-empty token requires a repository key); the returned row carries the
// plaintext back, so the in-memory contract never exposes the storage
// envelope.
func (r *ChatwootConfigRepository) Put(ctx context.Context, cfg model.ChatwootConfig) (*model.ChatwootConfig, error) {
	// pgx encodes a nil slice as NULL, which would violate the
	// ignored_jids NOT NULL constraint; an absent list means "ignore none".
	if cfg.IgnoreJIDs == nil {
		cfg.IgnoreJIDs = []string{}
	}
	sealed := cfg.Token
	if cfg.Token != "" {
		// Tokens persist only as ciphertext: writing a non-empty token
		// without a repository key is refused instead of storing plaintext.
		if r.tokenKey == nil {
			return nil, fmt.Errorf("put chatwoot config: WZAP_CHATWOOT_TOKEN_KEY is required to store a token")
		}
		var err error
		sealed, err = chatwootcfg.SealToken(cfg.Token, r.tokenKey)
		if err != nil {
			return nil, fmt.Errorf("put chatwoot config: %w", err)
		}
	}
	stored, err := scanChatwootConfig(r.pool.QueryRow(ctx, `
		INSERT INTO chatwoot_configs (instance_id, is_enabled, url, account_id, token,
			inbox_name, is_sign_enabled, sign_delimiter, is_reopen_enabled, is_pending_enabled,
			is_merge_enabled, is_import_contacts, is_import_messages, import_days, is_auto_create,
			organization, logo, ignored_jids)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (instance_id) DO UPDATE SET
			is_enabled = EXCLUDED.is_enabled, url = EXCLUDED.url, account_id = EXCLUDED.account_id,
			token = EXCLUDED.token, inbox_name = EXCLUDED.inbox_name, is_sign_enabled = EXCLUDED.is_sign_enabled,
			sign_delimiter = EXCLUDED.sign_delimiter, is_reopen_enabled = EXCLUDED.is_reopen_enabled,
			is_pending_enabled = EXCLUDED.is_pending_enabled,
			is_merge_enabled = EXCLUDED.is_merge_enabled,
			is_import_contacts = EXCLUDED.is_import_contacts, is_import_messages = EXCLUDED.is_import_messages,
			import_days = EXCLUDED.import_days, is_auto_create = EXCLUDED.is_auto_create,
			organization = EXCLUDED.organization, logo = EXCLUDED.logo,
			ignored_jids = EXCLUDED.ignored_jids, updated_at = now()
		RETURNING `+chatwootConfigColumns,
		cfg.InstanceID, cfg.Enabled, cfg.URL, cfg.AccountID, sealed,
		cfg.NameInbox, cfg.SignMsg, cfg.SignDelimiter, cfg.ReopenConversation, cfg.ConversationPending,
		cfg.MergeBrazilContacts, cfg.ImportContacts, cfg.ImportMessages, cfg.DaysLimit, cfg.AutoCreate,
		cfg.Organization, cfg.Logo, cfg.IgnoreJIDs,
	))
	if err != nil {
		return nil, mapChatwootError("put chatwoot config", err)
	}
	stored.Token = cfg.Token
	return stored, nil
}

// Delete removes the Chatwoot config of an instance. It reports
// storage.ErrNotFound when no config exists.
func (r *ChatwootConfigRepository) Delete(ctx context.Context, instanceID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM chatwoot_configs WHERE instance_id = $1`, instanceID)
	if err != nil {
		return fmt.Errorf("delete chatwoot config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete chatwoot config: %w", storage.ErrNotFound)
	}
	return nil
}

// Put upserts the correlation between a WhatsApp key and its Chatwoot mirror
// and returns the stored row.
func (r *ChatwootMessageRepository) Put(ctx context.Context, msg model.ChatwootMessage) (*model.ChatwootMessage, error) {
	stored, err := scanChatwootMessage(r.pool.QueryRow(ctx, `
		INSERT INTO chatwoot_messages (instance_id, message_id, wa_key, cw_id,
			conversation_id, inbox_id, chat_jid, is_read)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (instance_id, wa_key) DO UPDATE SET
			cw_id = EXCLUDED.cw_id, message_id = EXCLUDED.message_id,
			conversation_id = EXCLUDED.conversation_id, inbox_id = EXCLUDED.inbox_id,
			chat_jid = EXCLUDED.chat_jid, is_read = EXCLUDED.is_read, updated_at = now()
		RETURNING `+chatwootMessageColumns,
		msg.InstanceID, msg.MessageID, msg.WAKey, msg.ChatwootMessageID,
		msg.ConversationID, msg.InboxID, msg.ContactSourceID, msg.IsRead,
	))
	if err != nil {
		return nil, mapChatwootError("put chatwoot message", err)
	}
	return stored, nil
}

// PromotePending moves the provisional correlation of a queue row — stored
// under the reserved wa_key "pending:{queue uuid}" — to the real WhatsApp id
// once MarkSent produced it. It returns false when no pending row matches
// (the send went out before the pending rule existed or was already
// promoted). A wa_key already correlated reports a swallowed conflict as a
// successful no-op: the event replay never errors for a duplicate promote.
func (r *ChatwootMessageRepository) PromotePending(ctx context.Context, instanceID, queueID uuid.UUID, waKey string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE chatwoot_messages
		SET wa_key = $3, updated_at = now()
		WHERE instance_id = $1 AND wa_key = 'pending:' || $2::text`,
		instanceID, queueID, waKey)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// The real wa_key is already correlated (mirror or import won
			// first): drop the stale pending row and report promoted.
			if _, derr := r.pool.Exec(ctx,
				`DELETE FROM chatwoot_messages WHERE instance_id = $1 AND wa_key = 'pending:' || $2::text`,
				instanceID, queueID); derr != nil {
				return false, fmt.Errorf("promote pending chatwoot message: %w", derr)
			}
			return true, nil
		}
		return false, fmt.Errorf("promote pending chatwoot message: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// GetByWAKey returns the correlation for a WhatsApp key or
// storage.ErrNotFound.
func (r *ChatwootMessageRepository) GetByWAKey(ctx context.Context, instanceID uuid.UUID, waKey string) (*model.ChatwootMessage, error) {
	msg, err := scanChatwootMessage(r.pool.QueryRow(ctx,
		`SELECT `+chatwootMessageColumns+` FROM chatwoot_messages WHERE instance_id = $1 AND wa_key = $2`,
		instanceID, waKey))
	if err != nil {
		return nil, mapChatwootError("get chatwoot message", err)
	}
	return msg, nil
}

// DeleteByInstance removes every correlation row of an instance and returns
// how many were removed.
func (r *ChatwootMessageRepository) DeleteByInstance(ctx context.Context, instanceID uuid.UUID) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM chatwoot_messages WHERE instance_id = $1`, instanceID)
	if err != nil {
		return 0, fmt.Errorf("delete chatwoot messages: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetByChatwootID returns the correlation holding a Chatwoot message id or
// storage.ErrNotFound. It backs the inbound quoted and reverse-delete
// lookups without changing the storage interface.
func (r *ChatwootMessageRepository) GetByChatwootID(ctx context.Context, instanceID uuid.UUID, chatwootID int64) (*model.ChatwootMessage, error) {
	msg, err := scanChatwootMessage(r.pool.QueryRow(ctx,
		`SELECT `+chatwootMessageColumns+` FROM chatwoot_messages WHERE instance_id = $1 AND cw_id = $2 LIMIT 1`,
		instanceID, chatwootID))
	if err != nil {
		return nil, mapChatwootError("get chatwoot message by chatwoot id", err)
	}
	return msg, nil
}

// LatestByConversation returns the newest correlation of a conversation or
// storage.ErrNotFound, skipping the provisional "pending:{uuid}" rows of
// sends that still have no WhatsApp id: a pending key is never a real id,
// so handing it back would send read markers (and quotes/deletes resolved
// through this lookup) at a synthetic key. It backs the inbound MESSAGE_READ
// marking of the last received message. The tiebreak on cw_id keeps the
// choice deterministic when two rows share created_at (same instant),
// matching the (instance_id, conversation_id, created_at DESC, cw_id DESC)
// covering index from migration 00004.
func (r *ChatwootMessageRepository) LatestByConversation(ctx context.Context, instanceID uuid.UUID, conversationID int64) (*model.ChatwootMessage, error) {
	msg, err := scanChatwootMessage(r.pool.QueryRow(ctx,
		`SELECT `+chatwootMessageColumns+` FROM chatwoot_messages WHERE instance_id = $1 AND conversation_id = $2 AND wa_key NOT LIKE 'pending:%' ORDER BY created_at DESC, cw_id DESC LIMIT 1`,
		instanceID, conversationID))
	if err != nil {
		return nil, mapChatwootError("get latest chatwoot message", err)
	}
	return msg, nil
}

func scanChatwootConfig(scanner rowScanner) (*model.ChatwootConfig, error) {
	var cfg model.ChatwootConfig
	if err := scanner.Scan(
		&cfg.InstanceID, &cfg.Enabled, &cfg.URL, &cfg.AccountID, &cfg.Token,
		&cfg.NameInbox, &cfg.SignMsg, &cfg.SignDelimiter, &cfg.ReopenConversation, &cfg.ConversationPending,
		&cfg.MergeBrazilContacts, &cfg.ImportContacts, &cfg.ImportMessages, &cfg.DaysLimit, &cfg.AutoCreate,
		&cfg.Organization, &cfg.Logo, &cfg.IgnoreJIDs,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func scanChatwootMessage(scanner rowScanner) (*model.ChatwootMessage, error) {
	var msg model.ChatwootMessage
	if err := scanner.Scan(
		&msg.ID, &msg.InstanceID, &msg.MessageID, &msg.WAKey, &msg.ChatwootMessageID,
		&msg.ConversationID, &msg.InboxID, &msg.ContactSourceID, &msg.IsRead,
		&msg.CreatedAt, &msg.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &msg, nil
}

func mapChatwootError(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, storage.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", op, err)
}
