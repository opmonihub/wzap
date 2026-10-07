package httpapi

// Public DTOs of the remodeled contract (response-matrix rules §3–§7).
// Resource entities nest under data.<entity>; commands keep their flat
// result objects. last_error is either a structured object or null —
// legacy rows surface as {code:"legacy_error", message, occurred_at:null}.

import (
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
)

// lastErrorResponse is the structured failure of a connection or a send:
// code from the closed catalog (legacy_error for migrated free text),
// message the human-readable cause, occurred_at the recorded instant
// (null for legacy errors whose time is unknown).
type lastErrorResponse struct {
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	OccurredAt *time.Time `json:"occurred_at"`
}

// newLastError maps a domain InstanceError to its public form, or nil.
func newLastError(err *model.InstanceError) *lastErrorResponse {
	if err == nil {
		return nil
	}
	return &lastErrorResponse{Code: err.Code, Message: err.Message, OccurredAt: err.At}
}

// connectionResponse is the public connection block of an instance and of
// the connection-scoped routes (status, qr, connect). QR fields only appear
// while a pairing is in progress.
type connectionResponse struct {
	Status          string             `json:"status"`
	LastError       *lastErrorResponse `json:"last_error"`
	LastConnectedAt *time.Time         `json:"last_connected_at"`
	QRCode          string             `json:"qr_code,omitempty"`
	QRExpiresAt     *time.Time         `json:"qr_expires_at,omitempty"`
}

// newConnectionResponse maps the connection satellite to its public form.
func newConnectionResponse(conn model.InstanceConnection) connectionResponse {
	return connectionResponse{
		Status:          conn.Status,
		LastError:       newLastError(conn.LastError),
		LastConnectedAt: conn.LastConnectedAt,
	}
}

// pairingConnection is the connection subset the connect/qr commands answer:
// status plus the QR pair only while pairing is in progress (matrix §2).
type pairingConnection struct {
	Status      string     `json:"status"`
	QRCode      string     `json:"qr_code,omitempty"`
	QRExpiresAt *time.Time `json:"qr_expires_at,omitempty"`
}

// webhookResponse is the public webhook block of an instance: enabled flag,
// optional URL and the subscribed event types (always an array).
type webhookResponse struct {
	Enabled bool     `json:"enabled"`
	URL     *string  `json:"url"`
	Events  []string `json:"events"`
}

// newWebhookResponse maps the webhook satellite to its public form.
func newWebhookResponse(hook model.InstanceWebhook) webhookResponse {
	events := hook.Events
	if events == nil {
		events = []string{}
	}
	return webhookResponse{Enabled: hook.IsEnabled, URL: hook.URL, Events: events}
}

// integrationResponse groups the connector configuration of an instance: the
// delivery webhook (always present) and the optional Chatwoot connector
// config (null when the instance was never configured). The nested Chatwoot
// copy drops its instance_id (already at data.instance.id) and never carries
// the write-only token.
type integrationResponse struct {
	Webhook        webhookResponse         `json:"webhook"`
	ChatwootConfig *chatwootConfigResponse `json:"chatwoot_config"`
}

// settingsResponse is the settings aggregate of an instance. Every block is
// nullable on its own: the live blocks (profile, privacy, status_privacy) are
// read only while the instance is connected and degrade to null on any read
// failure, and default_disappearing is null until the default timer was
// configured at least once.
type settingsResponse struct {
	// DefaultDisappearing is the persisted echo of the last successful
	// default-disappearing write, in the textual form the PUT accepts (a Go
	// duration; canonical spellings 0, 24h, 168h, 2160h). Null means never
	// configured and is distinct from the stored off value "0".
	DefaultDisappearing *string                `json:"default_disappearing"`
	Profile             *profileResponse       `json:"profile"`
	Privacy             *privacyResponse       `json:"privacy"`
	StatusPrivacy       *statusPrivacyResponse `json:"status_privacy"`
}

// defaultDisappearingText renders the persisted default disappearing echo in
// the textual form the PUT accepts (a Go duration). The four allowlist values
// render with their canonical spellings; anything else falls back to the Go
// duration string. Nil (never configured) renders as null.
func defaultDisappearingText(duration *time.Duration) *string {
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
// data.<entity> on single reads/writes and under data.items[] on
// collections. Commands keep their flat data object.
type instanceEnvelope struct {
	Instance instanceResponse `json:"instance"`
}

type messageEnvelope struct {
	Message messageResponse `json:"message"`
}

type chatwootConfigEnvelope struct {
	ChatwootConfig chatwootConfigResponse `json:"chatwoot_config"`
}

type statsEnvelope struct {
	Stats instanceStatsResponse `json:"stats"`
}

type groupEnvelope struct {
	Group groupResponse `json:"group"`
}

type channelEnvelope struct {
	Channel newsletterResponse `json:"channel"`
}

// instanceResponse is the public representation of an instance: identity,
// the nested connection block, the integration block (webhook plus optional
// Chatwoot config), the nullable settings blocks and the timestamps.
// Internal fields (device_jid, whatsapp_jid, external_ref, owner_user_id, key
// hashes, Chatwoot token) never leave the service. The webhook block moved
// under integration (BREAKING); it is no longer an instance root key.
type instanceResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Connection  connectionResponse  `json:"connection"`
	Integration integrationResponse `json:"integration"`
	Settings    settingsResponse    `json:"settings"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// newInstanceResponse maps a stored instance to the pure part of its public
// DTO: identity, connection, integration.webhook and the persisted
// settings.default_disappearing echo. The remaining blocks stay null until
// the handler aggregation fills them (integration.chatwoot_config from the
// config store; the live settings blocks only while connected).
func newInstanceResponse(inst *model.Instance) instanceResponse {
	return instanceResponse{
		ID:         inst.ID.String(),
		Name:       inst.Name,
		Connection: newConnectionResponse(inst.Connection),
		Integration: integrationResponse{
			Webhook: newWebhookResponse(inst.Webhook),
		},
		Settings: settingsResponse{
			DefaultDisappearing: defaultDisappearingText(inst.DefaultDisappearing),
		},
		CreatedAt: inst.CreatedAt,
		UpdatedAt: inst.UpdatedAt,
	}
}

// createInstanceResponse is the 201 answer: the instance plus its one-time
// plaintext key delivered as a sibling field.
type createInstanceResponse struct {
	Instance       instanceResponse `json:"instance"`
	InstanceAPIKey string           `json:"instance_api_key"`
}

// instanceListResponse is the full authorized collection: every element
// nests the instance DTO under its own key (matrix §1, design JSON example).
type instanceListResponse struct {
	Items []instanceEnvelope `json:"items"`
}

// newCreateInstanceResponse maps an already-aggregated instance DTO and its
// one-time plaintext key to the 201 body. The key travels in this response
// only.
func newCreateInstanceResponse(inst instanceResponse, key string) createInstanceResponse {
	return createInstanceResponse{
		Instance:       inst,
		InstanceAPIKey: key,
	}
}

// messageResponse is the public representation of a queued or delivered
// message, using the remodeled column names.
type messageResponse struct {
	ID            string             `json:"id"`
	InstanceID    string             `json:"instance_id"`
	MessageType   string             `json:"message_type"`
	RecipientJID  string             `json:"recipient_jid"`
	SendStatus    string             `json:"send_status"`
	WAID          *string            `json:"wa_id"`
	MediaID       *uuid.UUID         `json:"media_id"`
	RetryCount    int                `json:"retry_count"`
	LastError     *lastErrorResponse `json:"last_error"`
	NextAttemptAt *time.Time         `json:"next_attempt_at"`
	DeliveredAt   *time.Time         `json:"delivered_at"`
	ReadAt        *time.Time         `json:"read_at"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// acceptedMessageResponse is the queue message known at accept time: only its
// real fields, never fabricated zero values. media_id is null except for
// media uploads, whose stored media is known when the send is enqueued.
type acceptedMessageResponse struct {
	ID         string     `json:"id"`
	InstanceID string     `json:"instance_id"`
	SendStatus string     `json:"send_status"`
	MediaID    *uuid.UUID `json:"media_id"`
}

// messageAcceptedResponse is the 202 answer to an accepted send: the queue
// message with its id and the queued status.
type messageAcceptedResponse struct {
	Message acceptedMessageResponse `json:"message"`
}

// messageListResponse is one page of messages plus its next cursor; each
// element nests the message DTO under its own key.
type messageListResponse struct {
	Items      []messageEnvelope `json:"items"`
	NextCursor string            `json:"next_cursor"`
}

// newMessageResponse maps a stored message to its public DTO. wa_id is null
// until the upstream confirms; last_error is null unless a failure was
// recorded.
func newMessageResponse(msg *model.OutboundMessage) messageResponse {
	var waID *string
	if msg.WhatsAppMessageID != "" {
		waID = &msg.WhatsAppMessageID
	}
	var lastErr *lastErrorResponse
	if msg.LastError != "" || msg.LastErrorCode != "" {
		code := msg.LastErrorCode
		if code == "" {
			code = "legacy_error"
		}
		lastErr = &lastErrorResponse{Code: code, Message: msg.LastError, OccurredAt: msg.LastErrorAt}
	}
	return messageResponse{
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
