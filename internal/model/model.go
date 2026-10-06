// Package model defines the persistence-agnostic types shared by the wzap
// storage and services.
package model

import (
	"time"

	"github.com/google/uuid"
)

// InstanceError is the structured failure recorded on a satellite table:
// code is the typed catalog entry (legacy_error when a pre-remodel row only
// carried free text), message the human-readable cause and at the instant it
// happened (nil for legacy errors whose occurrence time is unknown — never
// inferred from updated_at).
type InstanceError struct {
	Code    string
	Message string
	At      *time.Time
}

// InstanceConnection is the connection state of an instance, persisted in
// instance_connections (one row per instance). DeviceJID is the whatsmeow
// linked-device identity persisted in the session store; it is unique across
// instances when set (instance_connections_device_jid_uidx) and doubles as
// the public WhatsApp JID once paired.
type InstanceConnection struct {
	InstanceID      uuid.UUID
	DeviceJID       string
	Status          string
	LastConnectedAt *time.Time
	LastError       *InstanceError
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InstanceWebhook is the per-instance webhook configuration, persisted in
// instance_webhooks (one row per instance).
type InstanceWebhook struct {
	InstanceID uuid.UUID
	// URL is the webhook endpoint. Nil means unconfigured.
	URL *string
	// IsEnabled toggles delivery to URL. Events lists the subscribed event
	// types.
	IsEnabled bool
	Events    []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Instance is a WhatsApp instance registered with the wzap service. It is an
// aggregate: the row in instances carries the identity (name, external_ref,
// owner, key hash) while Connection and Webhook are the satellite rows read
// together with it.
type Instance struct {
	ID          uuid.UUID
	Name        string
	ExternalRef string
	// OwnerUserID is the manager user owning the instance. It stays nil for
	// legacy rows created before the product migration backfilled owners.
	OwnerUserID *uuid.UUID
	// Connection is the instance_connections satellite: session lifecycle
	// state written only by the connection commands.
	Connection InstanceConnection
	// Webhook is the instance_webhooks satellite: delivery configuration
	// written only by the webhook command. The instance key hash is never
	// exposed here; it stays inside the API key repository methods.
	Webhook   InstanceWebhook
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BoundDeviceJID returns the whatsmeow device identity bound to the
// instance, empty when it was never paired.
func (i Instance) BoundDeviceJID() string {
	return i.Connection.DeviceJID
}

// LastErrorMessage returns the message of the last connection error, empty
// when there is none.
func (i Instance) LastErrorMessage() string {
	if i.Connection.LastError == nil {
		return ""
	}
	return i.Connection.LastError.Message
}

// LastErrorCode returns the typed code of the last connection error, empty
// when there is none.
func (i Instance) LastErrorCode() string {
	if i.Connection.LastError == nil {
		return ""
	}
	return i.Connection.LastError.Code
}

// User is a manager account. PasswordHash is a repo-level credential detail and
// must never be logged or exposed beyond the storage boundary.
type User struct {
	ID            uuid.UUID
	Email         string
	PasswordHash  string
	Role          string
	InstanceQuota int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// OutboundMessage is a message queued for delivery through an instance.
type OutboundMessage struct {
	ID                uuid.UUID
	InstanceID        uuid.UUID
	Type              string
	RecipientJID      string
	Payload           []byte
	MediaID           *uuid.UUID
	Status            string
	WhatsAppMessageID string
	LastError         string
	Attempts          int
	DeliveredAt       *time.Time
	ReadAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Media is a stored media object. Its bytes live in the object store at
// (Bucket, ObjectKey); the row keeps the metadata and the checksum of the
// content. ObjectDeletedAt marks the confirmed remote removal: a non-nil
// value means the object is gone even if the row still exists.
type Media struct {
	ID         uuid.UUID
	InstanceID uuid.UUID
	Direction  string
	MessageID  string
	Mimetype   string
	Filename   string
	SizeBytes  int64
	Bucket     string
	ObjectKey  string
	SHA256     string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	// ObjectDeletedAt is set once the remote object removal is confirmed.
	ObjectDeletedAt *time.Time
}

// IdempotencyRecord is the stored outcome of a request accepted under an
// idempotency key. Status is "in_progress" while the original request runs and
// "completed" once its response was captured. ResponseStatus and ResponseBody
// are only meaningful when completed.
type IdempotencyRecord struct {
	InstanceID     uuid.UUID
	Key            string
	Fingerprint    string
	Status         string
	ResponseStatus int
	ResponseBody   []byte
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// OutboxEvent is an event awaiting publication to the broker. PublishedAt is
// nil until the relay publishes it; Attempts counts failed publish attempts.
type OutboxEvent struct {
	ID          uuid.UUID
	Subject     string
	Envelope    []byte
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	PublishedAt *time.Time
}

// ChatwootConfig is the per-instance Chatwoot connector configuration. Token
// is stored in clear text by design (encryption is a later version).
type ChatwootConfig struct {
	InstanceID          uuid.UUID
	Enabled             bool
	URL                 string
	AccountID           string
	Token               string
	NameInbox           string
	SignMsg             bool
	SignDelimiter       string
	ReopenConversation  bool
	ConversationPending bool
	MergeBrazilContacts bool
	ImportContacts      bool
	ImportMessages      bool
	DaysLimit           int
	AutoCreate          bool
	Organization        string
	Logo                string
	IgnoreJIDs          []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// ChatwootMessage correlates a WhatsApp message key with its Chatwoot mirror.
// Rows are never purged; they disappear on instance DELETE via cascade.
type ChatwootMessage struct {
	InstanceID        uuid.UUID
	WAKey             string
	ChatwootMessageID int64
	ConversationID    int64
	InboxID           int64
	ContactSourceID   string
	IsRead            bool
	CreatedAt         time.Time
}

// GroupMetadata is the cached metadata of a group: refreshed on demand from
// the live upstream view, never the source of truth. Rows disappear on
// instance DELETE via cascade.
type GroupMetadata struct {
	InstanceID       uuid.UUID
	GroupJID         string
	Name             string
	Description      string
	ParticipantCount int
	UpdatedAt        time.Time
}

// NewsletterMetadata is the cached metadata of a channel: refreshed on demand
// from the live upstream view, never the source of truth. Rows disappear on
// instance DELETE via cascade.
type NewsletterMetadata struct {
	InstanceID    uuid.UUID
	ChannelJID    string
	Title         string
	Description   string
	FollowerCount int
	UpdatedAt     time.Time
}
