package representation

import (
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
)

// LastErrorResponse is the structured failure of a connection or a send:
// code from the closed catalog, message the human-readable cause,
// occurred_at the recorded instant (omitted when unknown).
type LastErrorResponse struct {
	Code       string     `json:"code" binding:"required"`
	Message    string     `json:"message" binding:"required"`
	OccurredAt *time.Time `json:"occurred_at,omitempty"`
}

// NewLastError maps a domain InstanceError to its public form, or nil.
func NewLastError(err *model.InstanceError) *LastErrorResponse {
	if err == nil {
		return nil
	}
	return &LastErrorResponse{Code: err.Code, Message: err.Message, OccurredAt: err.At}
}

// ConnectionResponse is the public connection block of an instance and of
// the connection-scoped routes (status, qr, connect). QR fields only appear
// while a pairing is in progress.
type ConnectionResponse struct {
	Status          string             `json:"status" binding:"required"`
	LastError       *LastErrorResponse `json:"last_error,omitempty"`
	LastConnectedAt *time.Time         `json:"last_connected_at,omitempty"`
	QRCode          string             `json:"qr_code,omitempty"`
	QRExpiresAt     *time.Time         `json:"qr_expires_at,omitempty"`
}

// NewConnectionResponse maps the connection satellite to its public form.
func NewConnectionResponse(conn model.InstanceConnection) ConnectionResponse {
	return ConnectionResponse{
		Status:          conn.Status,
		LastError:       NewLastError(conn.LastError),
		LastConnectedAt: conn.LastConnectedAt,
	}
}

// PairingConnection is the connection subset the connect/qr commands answer:
// status plus the QR pair only while pairing is in progress (matrix §2).
type PairingConnection struct {
	Status      string     `json:"status" binding:"required"`
	QRCode      string     `json:"qr_code,omitempty"`
	QRExpiresAt *time.Time `json:"qr_expires_at,omitempty"`
}

// WebhookResponse is the public webhook block of an instance: enabled flag,
// optional URL and the subscribed event types (always an array).
type WebhookResponse struct {
	Enabled bool     `json:"enabled" binding:"required"`
	URL     *string  `json:"url,omitempty"`
	Events  []string `json:"events" binding:"required"`
}

// NewWebhookResponse maps the webhook satellite to its public form.
func NewWebhookResponse(hook model.InstanceWebhook) WebhookResponse {
	events := hook.Events
	if events == nil {
		events = []string{}
	}
	return WebhookResponse{Enabled: hook.IsEnabled, URL: hook.URL, Events: events}
}

// IntegrationResponse groups the connector configuration of an instance: the
// delivery webhook (always present) and the optional Chatwoot connector
// config (omitted when the instance has no configuration). The nested Chatwoot
// copy drops its instance_id (already at data.instance.id) and never carries
// the write-only token.
type IntegrationResponse struct {
	Webhook        WebhookResponse         `json:"webhook" binding:"required"`
	ChatwootConfig *ChatwootConfigResponse `json:"chatwoot_config,omitempty"`
}

// SettingsResponse is the settings aggregate of an instance. Every block is
// optional on its own: the live blocks (profile, privacy, status_privacy) are
// read only while the instance is connected and are omitted on any read
// failure, and default_disappearing is absent until the default timer was
// configured at least once.
type SettingsResponse struct {
	// DefaultDisappearing is the persisted echo of the last successful
	// default-disappearing write, in the textual form the PUT accepts (a Go
	// duration; canonical spellings 0, 24h, 168h, 2160h). Absence means never
	// configured and is distinct from the stored off value "0".
	DefaultDisappearing *string                `json:"default_disappearing,omitempty"`
	Profile             *ProfileResponse       `json:"profile,omitempty"`
	Privacy             *PrivacyResponse       `json:"privacy,omitempty"`
	StatusPrivacy       *StatusPrivacyResponse `json:"status_privacy,omitempty"`
}

// DefaultDisappearingText renders the persisted default disappearing echo in
// the textual form the PUT accepts (a Go duration). The four allowlist values
// render with their canonical spellings; anything else falls back to the Go
// duration string. Nil (not configured) represents absence.
func DefaultDisappearingText(duration *time.Duration) *string {
	if duration == nil {
		return nil
	}
	var text string
	switch *duration {
	case 0:
		text = "0"
	case 24 * time.Hour:
		text = "24h"
	case 168 * time.Hour:
		text = "168h"
	case 2160 * time.Hour:
		text = "2160h"
	default:
		text = duration.String()
	}
	return &text
}

// Resource envelopes (matrix §1): persisted entities always travel under
// data.<entity> on single reads/writes and directly in their named collection on
// collections. Commands keep their flat data object.
type InstanceEnvelope struct {
	Instance InstanceResponse `json:"instance" binding:"required"`
}
type MessageEnvelope struct {
	Message MessageResponse `json:"message" binding:"required"`
}
type ChatwootConfigEnvelope struct {
	ChatwootConfig ChatwootConfigResponse `json:"chatwoot_config" binding:"required"`
}
type StatsEnvelope struct {
	Stats InstanceStatsResponse `json:"stats" binding:"required"`
}
type GroupEnvelope struct {
	Group GroupResponse `json:"group" binding:"required"`
}
type ChannelEnvelope struct {
	Channel NewsletterResponse `json:"channel" binding:"required"`
}

// InstanceResponse is the public representation of an instance: identity,
// the nested connection block, the integration block (webhook plus optional
// Chatwoot config), the optional settings blocks and the timestamps.
// Internal fields (device_jid, whatsapp_jid, external_ref, owner_user_id, key
// hashes, Chatwoot token) are excluded. The webhook belongs to integration.
type InstanceResponse struct {
	ID          string              `json:"id" binding:"required"`
	Name        string              `json:"name" binding:"required"`
	CreatedAt   time.Time           `json:"created_at" binding:"required"`
	UpdatedAt   time.Time           `json:"updated_at" binding:"required"`
	Connection  ConnectionResponse  `json:"connection" binding:"required"`
	Integration IntegrationResponse `json:"integration" binding:"required"`
	Settings    *SettingsResponse   `json:"settings,omitempty"`
}

// NewInstanceResponse maps a stored instance to the pure part of its public
// DTO: identity, connection, integration.webhook and the persisted
// settings.default_disappearing echo. The remaining blocks are omitted until
// the handler aggregation fills them (integration.chatwoot_config from the
// config store; the live settings blocks only while connected).
func NewInstanceResponse(inst *model.Instance) InstanceResponse {
	response := InstanceResponse{
		ID:         inst.ID.String(),
		Name:       inst.Name,
		Connection: NewConnectionResponse(inst.Connection),
		Integration: IntegrationResponse{
			Webhook: NewWebhookResponse(inst.Webhook),
		},
		Settings: &SettingsResponse{
			DefaultDisappearing: DefaultDisappearingText(inst.DefaultDisappearing),
		},
		CreatedAt: inst.CreatedAt,
		UpdatedAt: inst.UpdatedAt,
	}
	if response.Settings.DefaultDisappearing == nil {
		response.Settings = nil
	}
	return response
}

// CreateInstanceResponse is the 201 answer: the instance plus its one-time
// plaintext key delivered as a sibling field.
type CreateInstanceResponse struct {
	Instance       InstanceResponse `json:"instance" binding:"required"`
	InstanceAPIKey string           `json:"instance_api_key" binding:"required"`
}

// InstanceListResponse is the full authorized collection: elements contain the instance DTO directly.
type InstanceListResponse struct {
	Instances []InstanceResponse `json:"instances" binding:"required"`
}

// NewCreateInstanceResponse maps an already-aggregated instance DTO and its
// one-time plaintext key to the 201 body. The key travels in this response
// only.
func NewCreateInstanceResponse(inst InstanceResponse, key string) CreateInstanceResponse {
	return CreateInstanceResponse{
		Instance:       inst,
		InstanceAPIKey: key,
	}
}

// MessageResponse is the public representation of a queued or delivered
// message, using the public field names.
type MessageResponse struct {
	ID            string             `json:"id" binding:"required"`
	InstanceID    string             `json:"instance_id" binding:"required"`
	MessageType   string             `json:"message_type" binding:"required"`
	RecipientJID  string             `json:"recipient_jid" binding:"required"`
	SendStatus    string             `json:"send_status" binding:"required"`
	WAID          *string            `json:"wa_id,omitempty"`
	MediaID       *uuid.UUID         `json:"media_id,omitempty"`
	RetryCount    int                `json:"retry_count" binding:"required"`
	LastError     *LastErrorResponse `json:"last_error,omitempty"`
	NextAttemptAt *time.Time         `json:"next_attempt_at,omitempty"`
	DeliveredAt   *time.Time         `json:"delivered_at,omitempty"`
	ReadAt        *time.Time         `json:"read_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at" binding:"required"`
	UpdatedAt     time.Time          `json:"updated_at" binding:"required"`
}

// AcceptedMessageResponse is the queue message known at accept time: only its
// real fields, never fabricated zero values. media_id is absent except for
// media uploads, whose stored media is known when the send is enqueued.
type AcceptedMessageResponse struct {
	ID         string     `json:"id" binding:"required"`
	InstanceID string     `json:"instance_id" binding:"required"`
	SendStatus string     `json:"send_status" binding:"required"`
	MediaID    *uuid.UUID `json:"media_id,omitempty"`
}

// MessageAcceptedResponse is the 202 answer to an accepted send: the queue
// message with its id and the queued status.
type MessageAcceptedResponse struct {
	Message AcceptedMessageResponse `json:"message" binding:"required"`
}

// MessageListResponse is one page of messages plus its optional next cursor; elements contain messages directly.
type MessageListResponse struct {
	Messages   []MessageResponse `json:"messages" binding:"required"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// NewMessageResponse maps a stored message to its public DTO. wa_id is absent
// until the upstream confirms; last_error is absent unless a failure was
// recorded.
func NewMessageResponse(msg *model.OutboundMessage) MessageResponse {
	var waID *string
	if msg.WhatsAppMessageID != "" {
		waID = &msg.WhatsAppMessageID
	}
	var lastErr *LastErrorResponse
	if msg.LastErrorCode != "" {
		lastErr = &LastErrorResponse{Code: msg.LastErrorCode, Message: msg.LastError, OccurredAt: msg.LastErrorAt}
	}
	return MessageResponse{
		ID:            msg.ID.String(),
		InstanceID:    msg.InstanceID.String(),
		MessageType:   msg.Type,
		RecipientJID:  msg.RecipientJID,
		SendStatus:    msg.Status,
		WAID:          waID,
		MediaID:       msg.MediaID,
		RetryCount:    msg.Attempts,
		LastError:     lastErr,
		NextAttemptAt: msg.NextAttemptAt,
		DeliveredAt:   msg.DeliveredAt,
		ReadAt:        msg.ReadAt,
		CreatedAt:     msg.CreatedAt,
		UpdatedAt:     msg.UpdatedAt,
	}
}
func OptionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
func NonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
