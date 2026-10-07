// Package storage declares the persistence contracts consumed by the wzap
// services.
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
)

var (
	// ErrInvalidInstanceName reports an actual rename to an unsafe or reserved name.
	ErrInvalidInstanceName = errors.New("invalid instance name")
	// ErrInstanceNameTaken reports a globally occupied exact instance name.
	ErrInstanceNameTaken = errors.New("instance name already taken")
	// ErrNotFound reports that the requested record does not exist.
	ErrNotFound = errors.New("record not found")
	// ErrExternalRefTaken reports that an instance external_ref is already in use.
	ErrExternalRefTaken = errors.New("external ref already taken")
	// ErrDeviceJIDTaken reports that a whatsmeow device JID is already bound to
	// another instance.
	ErrDeviceJIDTaken = errors.New("device jid already taken")
	// ErrEmailTaken reports that a user email is already in use
	// (case-insensitive, matching the lower(email) unique index).
	ErrEmailTaken = errors.New("email already taken")
	// ErrInvalidCursor reports that a pagination cursor is not a valid identifier.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrOwnerRequired reports that an instance write carried no owner:
	// ownership is NOT NULL and every instance must reference an existing
	// user.
	ErrOwnerRequired = errors.New("owner is required")
	// ErrFingerprintMismatch reports that an idempotency key was reused with a
	// different request fingerprint.
	ErrFingerprintMismatch = errors.New("idempotency fingerprint mismatch")
	// ErrInProgress reports that an idempotency key is owned by a request that
	// has not completed yet.
	ErrInProgress = errors.New("idempotency key in progress")
)

// InstanceRepository persists WhatsApp instances. The aggregate reads join the
// identity row (instances) with its satellites (instance_connections,
// instance_webhooks and the small instance_chat_settings), all created
// together with the identity or on first write of their concern. Writes are
// split per concern so an identity edit can never overwrite a concurrent
// connection transition (the lost-update bug of the snapshot UPDATE). List
// returns every instance ordered by created_at descending, then id
// descending.
type InstanceRepository interface {
	// Create persists the identity row plus its connection and webhook
	// satellites in one transaction.
	Create(ctx context.Context, instance model.Instance) (*model.Instance, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)
	GetByName(ctx context.Context, name string) (*model.Instance, error)
	GetByExternalRef(ctx context.Context, externalRef string) (*model.Instance, error)
	// GetByDeviceJID returns the instance bound to deviceJID or ErrNotFound.
	GetByDeviceJID(ctx context.Context, deviceJID string) (*model.Instance, error)
	List(ctx context.Context) ([]model.Instance, error)
	// UpdateIdentity rewrites only the identity columns (name, external_ref)
	// and returns the re-read aggregate. It never touches the connection or
	// webhook satellites, so a name edit cannot resurrect a stale session
	// state.
	UpdateIdentity(ctx context.Context, id uuid.UUID, name, externalRef string) (*model.Instance, error)
	// SetConnection updates the connection row of an instance in place:
	// status always, device_jid bound to deviceJID (cleared when empty) and
	// the error trio cleared when the JID is cleared. It never touches the
	// identity or webhook columns.
	SetConnection(ctx context.Context, id uuid.UUID, status, deviceJID string) error
	// SetConnectionState records a connection transition in place: status and
	// the error trio always (empty lastError clears them, a message stores
	// its classified code and the current instant), device_jid when it is
	// not empty (keeping the stored one otherwise) and last_connected_at
	// when connectedAt is set. Like SetConnection it never touches the other
	// tables.
	SetConnectionState(ctx context.Context, id uuid.UUID, status, deviceJID, lastError string, connectedAt *time.Time) error
	// SetWebhook replaces the webhook configuration of an instance in place,
	// never touching identity or connection columns.
	SetWebhook(ctx context.Context, id uuid.UUID, url *string, enabled bool, events []string) error
	// SetDefaultDisappearing persists the echo of the default disappearing
	// timer on the instance_chat_settings satellite (created on first write),
	// never touching identity, connection or webhook columns. A zero duration
	// stores the off value and stays distinct from NULL, which means never
	// configured. It reports ErrNotFound when the instance does not exist.
	SetDefaultDisappearing(ctx context.Context, id uuid.UUID, duration time.Duration) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// MessageRepository persists outbound messages. ListByInstance pages backwards
// by created_at and returns the cursor of the next page, empty when the page is
// the last one.
type MessageRepository interface {
	Create(ctx context.Context, message model.OutboundMessage) (*model.OutboundMessage, error)
	Get(ctx context.Context, id uuid.UUID) (*model.OutboundMessage, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID, limit int, cursor string) ([]model.OutboundMessage, string, error)
	// ClaimQueued selects queued messages whose next_attempt_at is due, oldest
	// first, and atomically moves them to sending with FOR UPDATE SKIP LOCKED.
	ClaimQueued(ctx context.Context, limit int) ([]model.OutboundMessage, error)
	MarkSent(ctx context.Context, id uuid.UUID, whatsAppMessageID string) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error
	// MarkRetrying moves a message back to queued, incrementing attempts.
	MarkRetrying(ctx context.Context, id uuid.UUID, errMsg string, nextAttemptAt time.Time) error
	// UpdateReceipt maps delivered to delivered_at and read/played to read_at.
	// It reports false when no message matches or the status is unknown.
	UpdateReceipt(ctx context.Context, whatsAppMessageID, status string, at time.Time) (bool, error)
	// RequeueStuck moves sending messages updated before olderThan back to queued.
	RequeueStuck(ctx context.Context, olderThan time.Time) (int64, error)
}

// MediaRepository persists media metadata. The content itself lives in an
// object store addressed by the row's Bucket and ObjectKey; ObjectDeletedAt
// marks the confirmed remote removal. ListByInstance and ListExpired return
// the rows so the caller can delete the objects first.
type MediaRepository interface {
	// Create persists media with the provided ExpiresAt and returns the stored
	// row with the database created_at. It returns ErrNotFound when the
	// instance does not exist.
	Create(ctx context.Context, media model.Media) (*model.Media, error)
	// Get returns the media with the given id or ErrNotFound.
	Get(ctx context.Context, id uuid.UUID) (*model.Media, error)
	// ListByInstance returns every media of instanceID ordered by created_at.
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]model.Media, error)
	// ListExpired returns the media whose expires_at is due and whose object
	// is not yet confirmed deleted, oldest first.
	ListExpired(ctx context.Context, now time.Time) ([]model.Media, error)
	// MarkObjectDeleted sets object_deleted_at on the row after the remote
	// removal is confirmed, preserving the metadata. It returns ErrNotFound
	// when the row is absent.
	MarkObjectDeleted(ctx context.Context, id uuid.UUID, at time.Time) error
	// SetBucket rewrites the bucket a media object lives in, so a row keeps
	// recording where the content actually is (e.g. after MigrateLocalFiles
	// uploads a local file into the configured bucket). It returns
	// ErrNotFound when the row is absent.
	SetBucket(ctx context.Context, id uuid.UUID, bucket string) error
	// ListInstancesWithMedia returns the distinct instance ids owning at
	// least one media row, for migrations and reconciliations.
	ListInstancesWithMedia(ctx context.Context) ([]uuid.UUID, error)
	// Delete removes one media row. It returns ErrNotFound when it is absent.
	Delete(ctx context.Context, id uuid.UUID) error
	// DeleteByInstance removes every media row of instanceID.
	DeleteByInstance(ctx context.Context, instanceID uuid.UUID) (int64, error)
}

// IdempotencyRepository persists request outcomes keyed by
// (instance, Idempotency-Key).
type IdempotencyRepository interface {
	// Acquire tries to own key. It returns acquired true with the fresh record
	// when it won the key. A key that expired is treated as absent and
	// reacquired. When another request already completed the key, it returns
	// the stored record with acquired false. Reusing a key with a different
	// fingerprint returns ErrFingerprintMismatch; an in-flight key returns
	// ErrInProgress.
	Acquire(ctx context.Context, instanceID uuid.UUID, key, fingerprint string, expiresAt time.Time) (*model.IdempotencyRecord, bool, error)
	// Complete stores the original response under key. It returns ErrNotFound
	// when the key does not exist.
	Complete(ctx context.Context, instanceID uuid.UUID, key string, status int, body []byte) error
	// Release frees key so a corrected request can retry under it. Releasing an
	// absent key is a no-op.
	Release(ctx context.Context, instanceID uuid.UUID, key string) error
	// DeleteExpired removes keys whose expires_at is in the past.
	DeleteExpired(ctx context.Context) (int64, error)
}

// JIDCacheRepository persists phone to JID resolutions until their expiry.
type JIDCacheRepository interface {
	// Get returns the cached JID for phone. Expired entries count as a miss.
	Get(ctx context.Context, phone string) (jid string, ok bool, err error)
	// Put upserts the JID resolved for phone.
	Put(ctx context.Context, phone, jid string, expiresAt time.Time) error
	// DeleteExpired removes entries whose expires_at is in the past.
	DeleteExpired(ctx context.Context) (int64, error)
}

// EventOutboxRepository persists events until the relay publishes them.
type EventOutboxRepository interface {
	Enqueue(ctx context.Context, id uuid.UUID, subject string, envelope []byte) error
	// ClaimPending returns up to limit unpublished events, oldest first.
	ClaimPending(ctx context.Context, limit int) ([]model.OutboxEvent, error)
	// MarkPublished stamps the event as published. It returns ErrNotFound when
	// the event does not exist.
	MarkPublished(ctx context.Context, id uuid.UUID) error
	// MarkAttempt records a failed publish attempt. It returns ErrNotFound when
	// the event does not exist.
	MarkAttempt(ctx context.Context, id uuid.UUID, errMsg string) error
}

// UserRepository persists manager users. GetByEmail matches case-insensitively
// and Create reports ErrEmailTaken when the email is already in use.
type UserRepository interface {
	Create(ctx context.Context, user model.User) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	// List returns every user ordered by created_at.
	List(ctx context.Context) ([]model.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Count(ctx context.Context) (int, error)
	// UpdateQuota stores a new per-user instance quota. A stored 0 means
	// unlimited. It reports ErrNotFound when the user does not exist.
	UpdateQuota(ctx context.Context, id uuid.UUID, quota int) error
}

// APIKeyRepository manages the instance key hashes used to authenticate
// instance-scoped requests. Hashes never leave the repository: callers pass
// hashes in and get instance ids back.
type APIKeyRepository interface {
	// SetHash stores (or replaces) the key hash of an instance. It returns
	// ErrNotFound when the instance does not exist.
	SetHash(ctx context.Context, instanceID uuid.UUID, hash string) error
	// InstanceByHash resolves the instance id holding hash or ErrNotFound.
	InstanceByHash(ctx context.Context, hash string) (uuid.UUID, error)
	// ClearHash revokes the instance key by NULLing its hash. It returns
	// ErrNotFound when the instance does not exist.
	ClearHash(ctx context.Context, instanceID uuid.UUID) error
	// CountByOwner counts the instances owned by owner. CountAll counts every
	// instance, any state; both feed the quota checks.
	CountByOwner(ctx context.Context, owner uuid.UUID) (int, error)
	CountAll(ctx context.Context) (int, error)
}

// ChatwootConfigRepository persists the per-instance Chatwoot connector
// configuration. Get reports ErrNotFound when the instance was never
// configured.
type ChatwootConfigRepository interface {
	Get(ctx context.Context, instanceID uuid.UUID) (*model.ChatwootConfig, error)
	Put(ctx context.Context, cfg model.ChatwootConfig) (*model.ChatwootConfig, error)
	Delete(ctx context.Context, instanceID uuid.UUID) error
}

// ChatwootMessageRepository persists WhatsApp to Chatwoot ID correlations.
// Rows are never purged; DeleteByInstance removes every row of an instance.
type ChatwootMessageRepository interface {
	Put(ctx context.Context, msg model.ChatwootMessage) (*model.ChatwootMessage, error)
	GetByWAKey(ctx context.Context, instanceID uuid.UUID, waKey string) (*model.ChatwootMessage, error)
	// GetByChatwootID resolves a Chatwoot message back to its WhatsApp key
	// for quoting, reverse delete and read markers. It reports ErrNotFound
	// without a correlation.
	GetByChatwootID(ctx context.Context, instanceID uuid.UUID, chatwootID int64) (*model.ChatwootMessage, error)
	// LatestByConversation returns the newest correlation of a conversation,
	// skipping the provisional "pending:{uuid}" rows of sends that have no
	// WhatsApp id yet, so a pending key is never handed back as a real id. It
	// backs recipient resolution and mark-read. It reports ErrNotFound
	// without one.
	LatestByConversation(ctx context.Context, instanceID uuid.UUID, conversationID int64) (*model.ChatwootMessage, error)
	// PromotePending rewrites the provisional wa_key "pending:{queueID}" of an
	// outbound correlation to the real WhatsApp id. It returns false when no
	// pending row matches; an already-correlated wa_key is a successful
	// no-op so replayed status events never fail.
	PromotePending(ctx context.Context, instanceID, queueID uuid.UUID, waKey string) (bool, error)
	DeleteByInstance(ctx context.Context, instanceID uuid.UUID) (int64, error)
}

// DeadLetterRepository persists exhausted webhook deliveries for operator
// inspection. The NATS outbox stays the source of truth for redelivery; the
// dead-letter table is the durable counterpart of the dead-letter log.
type DeadLetterRepository interface {
	// RecordDeadLetter stores one exhausted delivery, deduplicated by
	// event_id (a replayed event re-records silently). Rows per instance
	// are trimmed to a bounded tail so the table cannot grow without bound.
	RecordDeadLetter(ctx context.Context, instanceID, eventID uuid.UUID, eventType string, payload []byte, attempts int, lastError string) error
}

// GroupMetadataRepository persists the on-demand refreshed group metadata
// cache. The live upstream view stays the source of truth; the rows only
// carry the last refresh with its updated_at.
type GroupMetadataRepository interface {
	// Upsert stores the refreshed metadata of a group, refreshing updated_at.
	Upsert(ctx context.Context, meta model.GroupMetadata) (model.GroupMetadata, error)
	// Get returns the cached metadata of a group or ErrNotFound.
	Get(ctx context.Context, instanceID uuid.UUID, groupJID string) (model.GroupMetadata, error)
	// DeleteByInstance removes every cached row of an instance.
	DeleteByInstance(ctx context.Context, instanceID uuid.UUID) (int64, error)
}

// NewsletterMetadataRepository persists the on-demand refreshed channel
// metadata cache, following the same cache contract as the groups.
type NewsletterMetadataRepository interface {
	// Upsert stores the refreshed metadata of a channel, refreshing
	// updated_at.
	Upsert(ctx context.Context, meta model.NewsletterMetadata) (model.NewsletterMetadata, error)
	// Get returns the cached metadata of a channel or ErrNotFound.
	Get(ctx context.Context, instanceID uuid.UUID, channelJID string) (model.NewsletterMetadata, error)
	// ListByInstance returns every cached row of an instance ordered by
	// channel JID.
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]model.NewsletterMetadata, error)
	// DeleteByInstance removes every cached row of an instance.
	DeleteByInstance(ctx context.Context, instanceID uuid.UUID) (int64, error)
}
