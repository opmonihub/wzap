package representation

import (
	"strings"

	"github.com/google/uuid"

	"wzap/internal/model"
)

// ChatwootConfigResponse is the GET/PUT /instances/{id}/chatwoot body (the
// public §6 shape): the stored connector with the public flag names plus
// the computed webhook_url. The token field never exists on reads — it is
// write-only, so it is not serialized at all. instance_id is present on the
// standalone route and omitted (omitempty) in the nested
// integration.chatwoot_config copy of an instance, where data.instance.id
// already carries it.
type ChatwootConfigResponse struct {
	InstanceID       string   `json:"instance_id,omitempty"`
	IsEnabled        bool     `json:"is_enabled" binding:"required"`
	URL              string   `json:"url,omitempty"`
	AccountID        string   `json:"account_id,omitempty"`
	InboxName        string   `json:"inbox_name" binding:"required"`
	IsSignEnabled    bool     `json:"is_sign_enabled" binding:"required"`
	SignDelimiter    string   `json:"sign_delimiter" binding:"required"`
	IsReopenEnabled  bool     `json:"is_reopen_enabled" binding:"required"`
	IsPendingEnabled bool     `json:"is_pending_enabled" binding:"required"`
	IsMergeEnabled   bool     `json:"is_merge_enabled" binding:"required"`
	IsImportContacts bool     `json:"is_import_contacts" binding:"required"`
	IsImportMessages bool     `json:"is_import_messages" binding:"required"`
	ImportDays       int      `json:"import_days" binding:"required"`
	IsAutoCreate     bool     `json:"is_auto_create" binding:"required"`
	Organization     string   `json:"organization,omitempty"`
	Logo             string   `json:"logo,omitempty"`
	IgnoredJIDs      []string `json:"ignored_jids" binding:"required"`
	WebhookURL       string   `json:"webhook_url" binding:"required"`
}

// NewChatwootConfigResponse maps a stored config plus its webhook URL to the
// public §6 shape. The token is accepted on write only and is never part of
// the read representation — the field does not exist in the DTO.
func NewChatwootConfigResponse(cfg *model.ChatwootConfig, webhookURL string) ChatwootConfigResponse {
	ignoredJIDs := cfg.IgnoreJIDs
	if ignoredJIDs == nil {
		ignoredJIDs = []string{}
	}
	return ChatwootConfigResponse{
		InstanceID:       cfg.InstanceID.String(),
		IsEnabled:        cfg.Enabled,
		URL:              cfg.URL,
		AccountID:        cfg.AccountID,
		InboxName:        cfg.NameInbox,
		IsSignEnabled:    cfg.SignMsg,
		SignDelimiter:    cfg.SignDelimiter,
		IsReopenEnabled:  cfg.ReopenConversation,
		IsPendingEnabled: cfg.ConversationPending,
		IsMergeEnabled:   cfg.MergeBrazilContacts,
		IsImportContacts: cfg.ImportContacts,
		IsImportMessages: cfg.ImportMessages,
		ImportDays:       cfg.DaysLimit,
		IsAutoCreate:     cfg.AutoCreate,
		Organization:     cfg.Organization,
		Logo:             cfg.Logo,
		IgnoredJIDs:      ignoredJIDs,
		WebhookURL:       webhookURL,
	}
}

// ChatwootWebhookURL builds the webhook URL to register in Chatwoot: the
// public base plus the open route, or the relative route without a base.
func ChatwootWebhookURL(publicURL string, instanceID uuid.UUID) string {
	path := "/chatwoot/webhook/" + instanceID.String()
	base := strings.TrimSuffix(strings.TrimSpace(publicURL), "/")
	if base == "" {
		return path
	}
	return base + path
}
