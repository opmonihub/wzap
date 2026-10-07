package chatwoot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/chatwoot/client"
	"wzap/internal/chatwoot/config"
	"wzap/internal/chatwoot/inbound"
	cfgpkg "wzap/internal/config"
	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
	"wzap/internal/model"
	"wzap/internal/storage"
)

// ChatwootConfigStore persists the per-instance Chatwoot connector config.
type ChatwootConfigStore interface {
	Get(ctx context.Context, instanceID uuid.UUID) (*model.ChatwootConfig, error)
	Put(ctx context.Context, cfg model.ChatwootConfig) (*model.ChatwootConfig, error)
	Delete(ctx context.Context, instanceID uuid.UUID) error
}

// ChatwootInbound processes the open webhook payload.
type ChatwootInbound interface {
	Handle(ctx context.Context, instanceID uuid.UUID, payload inbound.Payload) (int, error)
	HandleCommand(ctx context.Context, instanceID uuid.UUID, command string, conversationID int64) (int, error)
}

// ChatwootInboxClient provisions the api inbox for auto_create.
type ChatwootInboxClient interface {
	ListInboxes(ctx context.Context) ([]client.Inbox, error)
	CreateInbox(ctx context.Context, req client.CreateInboxRequest) (*client.Inbox, error)
}

// ChatwootClientFor builds the per-instance Chatwoot API for auto_create.
// Nil means provisioning is skipped (tests); production wires client.New.
type ChatwootClientFor func(cfg model.ChatwootConfig) ChatwootInboxClient

// The repositories satisfy the handler contracts; the assertions catch
// signature drift at build time.
var (
	_ ChatwootConfigStore = (storage.ChatwootConfigRepository)(nil)
	_ ChatwootInbound     = (*inbound.Handler)(nil)
)

// ChatwootImporter runs the manual history import of an instance, returning
// how many messages were written. Production wires the chatimport run;
// tests replay a count.
type ChatwootImporter interface {
	ImportHistory(ctx context.Context, instanceID uuid.UUID) (int, error)
}

// ChatwootSetRequest is the PUT /instances/{id}/chatwoot payload. Booleans
// are values (absent means false); sign_msg type errors are mapped to 422 by
// inspecting the decode failure.
type ChatwootSetRequest struct {
	IsEnabled bool   `json:"is_enabled"`
	URL       string `json:"url"`
	AccountID string `json:"account_id"`
	// Token is accepted only on write and never echoed in config responses.
	Token            string   `json:"token"`
	InboxName        string   `json:"inbox_name"`
	IsSignEnabled    bool     `json:"is_sign_enabled"`
	SignDelimiter    string   `json:"sign_delimiter"`
	IsReopenEnabled  bool     `json:"is_reopen_enabled"`
	IsPendingEnabled bool     `json:"is_pending_enabled"`
	IsMergeEnabled   bool     `json:"is_merge_enabled"`
	IsImportContacts bool     `json:"is_import_contacts"`
	IsImportMessages bool     `json:"is_import_messages"`
	ImportDays       int      `json:"import_days"`
	IsAutoCreate     bool     `json:"is_auto_create"`
	Organization     string   `json:"organization"`
	Logo             string   `json:"logo"`
	IgnoredJIDs      []string `json:"ignored_jids"`
}

// HandleChatwootSet stores the connector config behind the dual auth. The
// global gate answers 400 when disabled; validation failures answer 422
// without persisting.
//
// @Summary Configure the Chatwoot connector
// @Description Accepts a global key, own instance key or wzap_session cookie; user sessions are limited to owned instances and admin/global scope can access every instance. Token is write-only: it is accepted on PUT and never appears in GET or PUT responses. Enabled configuration is validated before persistence; is_auto_create attempts inbox provisioning.
// @Tags chatwoot
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Accept json
// @Param request body ChatwootSetRequest true "Connector configuration; token is accepted only on write"
// @Success 200 {object} core.Envelope{data=representation.ChatwootConfigEnvelope} "Saved connector config under data.chatwoot_config; token is absent"
// @Failure 400 {object} core.ErrorEnvelope "Connector disabled, malformed request or invalid instance ID"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid configuration or field type"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chatwoot [put]
func HandleChatwootSet(instances InstanceService, configs ChatwootConfigStore, global cfgpkg.Chatwoot, publicURL string, clientFor ChatwootClientFor, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !global.Enabled {
			core.Error(w, r, http.StatusBadRequest, "chatwoot_disabled", "chatwoot connector is disabled")
			return
		}
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}
		var request ChatwootSetRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			var typeErr *json.UnmarshalTypeError
			if errors.As(err, &typeErr) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid chatwoot config: "+typeErr.Field+" has wrong type")
				return
			}
			core.WriteJSONBodyError(w, r, err)
			return
		}
		nameInbox := strings.TrimSpace(request.InboxName)
		if nameInbox == "" {
			nameInbox = config.DefaultInbox(stored.Name)
		}
		delimiter := request.SignDelimiter
		if delimiter == "" {
			delimiter = config.DefaultDelimiter()
		}
		ignoredJIDs := request.IgnoredJIDs
		if ignoredJIDs == nil {
			ignoredJIDs = []string{}
		}
		cfg := model.ChatwootConfig{
			InstanceID:          id,
			Enabled:             request.IsEnabled,
			URL:                 strings.TrimSpace(request.URL),
			AccountID:           strings.TrimSpace(request.AccountID),
			Token:               request.Token,
			NameInbox:           nameInbox,
			SignMsg:             request.IsSignEnabled,
			SignDelimiter:       delimiter,
			ReopenConversation:  request.IsReopenEnabled,
			ConversationPending: request.IsPendingEnabled,
			MergeBrazilContacts: request.IsMergeEnabled,
			ImportContacts:      request.IsImportContacts,
			ImportMessages:      request.IsImportMessages,
			DaysLimit:           request.ImportDays,
			AutoCreate:          request.IsAutoCreate,
			Organization:        strings.TrimSpace(request.Organization),
			Logo:                strings.TrimSpace(request.Logo),
			IgnoreJIDs:          ignoredJIDs,
		}
		if err := config.Validate(cfg); err != nil {
			var field *config.ErrField
			if errors.As(err, &field) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid chatwoot config: "+field.Field+" "+field.Message)
				return
			}
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid chatwoot config")
			return
		}
		saved, err := configs.Put(r.Context(), cfg)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		webhookURL := representation.ChatwootWebhookURL(publicURL, id)
		if saved.Enabled && saved.AutoCreate && clientFor != nil {
			EnsureChatwootInbox(r.Context(), clientFor, *saved, webhookURL, log)
		}
		core.JSON(w, r, http.StatusOK, representation.ChatwootConfigEnvelope{ChatwootConfig: representation.NewChatwootConfigResponse(saved, webhookURL)})
	}
}

// EnsureChatwootInbox guarantees the api inbox for auto_create, reusing it
// by name. Best-effort: failures only warn, never fail the set.
func EnsureChatwootInbox(ctx context.Context, clientFor ChatwootClientFor, cfg model.ChatwootConfig, webhookURL string, log zerolog.Logger) {
	cli := clientFor(cfg)
	if cli == nil {
		return
	}
	inboxes, err := cli.ListInboxes(ctx)
	if err != nil {
		log.Warn().Str("instance_id", cfg.InstanceID.String()).Err(err).Msg("chatwoot auto_create inbox listing failed")
		return
	}
	for _, inbox := range inboxes {
		if inbox.Name == cfg.NameInbox {
			return
		}
	}
	if _, err := cli.CreateInbox(ctx, client.CreateInboxRequest{Name: cfg.NameInbox, WebhookURL: webhookURL}); err != nil {
		log.Warn().Str("instance_id", cfg.InstanceID.String()).Err(err).Msg("chatwoot auto_create inbox creation failed")
	}
}

// HandleChatwootGet returns the connector config behind the dual auth. An
// instance that was never configured answers 200 disabled with empty fields.
//
// @Summary Get the Chatwoot connector
// @Description Accepts a global key, own instance key or wzap_session cookie; user sessions are limited to owned instances and admin/global scope can access every instance. A never-configured instance returns a disabled config with empty fields. Token is write-only and absent from the response.
// @Tags chatwoot
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=representation.ChatwootConfigEnvelope} "Connector config under data.chatwoot_config; token is absent"
// @Failure 400 {object} core.ErrorEnvelope "Connector disabled, malformed request or invalid instance ID"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chatwoot [get]
func HandleChatwootGet(instances InstanceService, configs ChatwootConfigStore, global cfgpkg.Chatwoot, publicURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !global.Enabled {
			core.Error(w, r, http.StatusBadRequest, "chatwoot_disabled", "chatwoot connector is disabled")
			return
		}
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}
		cfg, err := configs.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				core.JSON(w, r, http.StatusOK, representation.ChatwootConfigEnvelope{ChatwootConfig: representation.NewChatwootConfigResponse(&model.ChatwootConfig{InstanceID: id, IgnoreJIDs: []string{}}, representation.ChatwootWebhookURL(publicURL, id))})
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		core.JSON(w, r, http.StatusOK, representation.ChatwootConfigEnvelope{ChatwootConfig: representation.NewChatwootConfigResponse(cfg, representation.ChatwootWebhookURL(publicURL, id))})
	}
}

// HandleChatwootWebhook processes POST /chatwoot/webhook/{id} outside auth:
// open by design, the secret is v2. The global gate answers 400 when
// disabled; discards answer 200 with a bot body. O limiter responde 429
// no estouro por instância; comandos operacionais nunca executam aqui
// (descartam 200 no inbound) — usam POST /instances/{id}/chatwoot/command.
//
// @Summary Receive a Chatwoot webhook
// @Description Public by design: no apikey or session cookie is required. The per-instance limiter can return 429. Success and discarded events return the raw {"content":""} acknowledgement, outside the REST data envelope. Operational commands are discarded here and must use the authenticated /instances/{id}/chatwoot/command route.
// @Tags chatwoot
// @Accept json
// @Produce json
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body inbound.Payload true "Chatwoot event subset; unknown fields are ignored"
// @Success 200 {object} object{content=string} "Raw acknowledgement with empty content"
// @Failure 400 {object} core.ErrorEnvelope "Connector disabled or invalid body (including oversized body)"
// @Failure 404 {object} core.ErrorEnvelope "Invalid or missing instance"
// @Failure 429 {object} core.ErrorEnvelope "Per-instance webhook rate limit exceeded"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /chatwoot/webhook/{id} [post]
func HandleChatwootWebhook(instances InstanceService, inb ChatwootInbound, global cfgpkg.Chatwoot, limiter *ChatwootRateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if limiter != nil && !limiter.Allow(id) {
			core.Error(w, r, http.StatusTooManyRequests, "rate_limited", "rate limited")
			return
		}
		if _, err := core.LoadInstance(r, instances, id); err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if !global.Enabled {
			core.Error(w, r, http.StatusBadRequest, "chatwoot_disabled", "chatwoot connector is disabled")
			return
		}
		if inb == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		var payload inbound.Payload
		r.Body = http.MaxBytesReader(w, r.Body, core.MaxJSONBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			core.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
		status, err := inb.Handle(r.Context(), id, payload)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if status != http.StatusOK {
			code, message := ChatwootWebhookError(status)
			core.Error(w, r, status, code, message)
			return
		}
		WriteChatwootBotBody(w)
	}
}

// ChatwootWebhookError maps a non-200 inbound status to its error code.
// chatwoot_disabled is reserved for the 400 global-off gate; every other
// status keeps its own code so callers can tell a disabled connector apart
// from a missing instance or an internal failure.
func ChatwootWebhookError(status int) (code, message string) {
	switch status {
	case http.StatusBadRequest:
		return "chatwoot_disabled", "chatwoot connector is disabled"
	case http.StatusNotFound:
		return "not_found", "instance not found"
	case http.StatusTooManyRequests:
		return "rate_limited", "rate limited"
	case http.StatusInternalServerError:
		return "internal_error", "internal server error"
	default:
		return "chatwoot_error", "chatwoot webhook failed"
	}
}

// WriteChatwootBotBody answers the open webhook with the bot body Chatwoot
// expects: 200 with a JSON content envelope, empty when nothing was done.
func WriteChatwootBotBody(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"content": ""})
}

// HandleChatwootImport runs the manual history import behind the dual auth.
// The global gate answers 400 when disabled; an unconfigured instance
// answers 404; success answers 202 with the imported message count.
//
// @Summary Import Chatwoot message history
// @Description Accepts a global key, own instance key or wzap_session cookie; user sessions are limited to owned instances and admin/global scope can access every instance. Runs the configured history import before returning. The 202 data.imported value counts messages already imported; it is not a background job identifier. No request body is required.
// @Tags chatwoot
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 202 {object} core.Envelope{data=object{imported=int}} "Number of messages already imported"
// @Failure 400 {object} core.ErrorEnvelope "Connector disabled, malformed request or invalid instance ID"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or required connector config not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chatwoot/import [post]
func HandleChatwootImport(instances InstanceService, configs ChatwootConfigStore, global cfgpkg.Chatwoot, importer ChatwootImporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !global.Enabled {
			core.Error(w, r, http.StatusBadRequest, "chatwoot_disabled", "chatwoot connector is disabled")
			return
		}
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}
		if _, err := configs.Get(r.Context(), id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				core.Error(w, r, http.StatusNotFound, "not_found", "chatwoot not configured")
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if importer == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		imported, err := importer.ImportHistory(r.Context(), id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				core.Error(w, r, http.StatusNotFound, "not_found", "chatwoot not configured")
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		core.JSON(w, r, http.StatusAccepted, map[string]any{"imported": imported})
	}
}

// ChatwootCommandRequest is the POST /instances/{id}/chatwoot/command payload.
type ChatwootCommandRequest struct {
	Command        string `json:"command"`
	ConversationID int64  `json:"conversation_id"`
}

// HandleChatwootCommand runs operational commands behind the dual auth
// (status, init[:number], clearcache, disconnect). O webhook aberto nunca
// executa comandos; esta rota autenticada é o único caminho.
//
// @Summary Run an authenticated Chatwoot command
// @Description Accepts a global key, own instance key or wzap_session cookie; user sessions are limited to owned instances and admin/global scope can access every instance. Runs status, init[:number], clearcache or disconnect using the supplied conversation_id. Operational commands are authenticated here; the open webhook never executes them.
// @Tags chatwoot
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Accept json
// @Param request body ChatwootCommandRequest true "Operational command and Chatwoot conversation ID"
// @Success 200 {object} core.Envelope{data=object{ok=bool}} "Command processed"
// @Failure 400 {object} core.ErrorEnvelope "Connector disabled, malformed request or invalid instance ID"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or required connector config not found"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chatwoot/command [post]
func HandleChatwootCommand(instances InstanceService, configs ChatwootConfigStore, global cfgpkg.Chatwoot, inb ChatwootInbound) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !global.Enabled {
			core.Error(w, r, http.StatusBadRequest, "chatwoot_disabled", "chatwoot connector is disabled")
			return
		}
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}
		if _, err := configs.Get(r.Context(), id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				core.Error(w, r, http.StatusNotFound, "not_found", "chatwoot not configured")
				return
			}
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		var request ChatwootCommandRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if strings.TrimSpace(request.Command) == "" {
			core.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
		if inb == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		status, err := inb.HandleCommand(r.Context(), id, request.Command, request.ConversationID)
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if status != http.StatusOK {
			code, message := ChatwootWebhookError(status)
			core.Error(w, r, status, code, message)
			return
		}
		core.JSON(w, r, http.StatusOK, map[string]any{"ok": true})
	}
}
