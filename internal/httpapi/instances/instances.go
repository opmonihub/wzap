package instances

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/auth"
	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/storage"
	"wzap/internal/webhook"
)

// The service satisfies the handler contract; the assertion catches signature
// drift at build time.
var _ InstanceService = (*instance.Service)(nil)

// InstanceAggregateConcurrency bounds how many instances the list endpoint
// aggregates at the same time. The Chatwoot lookup and the live settings
// reads are per-instance calls; the bound keeps one slow instance from
// serializing (or saturating) the whole listing.
const InstanceAggregateConcurrency = 8

// InstanceConfigAggregator assembles the non-identity blocks of the public
// instance DTO from their own sources: integration.chatwoot_config from the
// Chatwoot config store and the live settings blocks (profile, privacy,
// status_privacy) from the session service, which answers only while the
// instance is connected. Every block is omitted on a read failure (a
// warn is logged): the aggregate response never fails because one source is
// down. A nil configs store reads as "never configured" (omitted).
type InstanceConfigAggregator struct {
	instances InstanceService
	configs   ChatwootConfigStore
	publicURL string
	log       zerolog.Logger
}

// build assembles the aggregated public DTO of one stored instance.
func (a InstanceConfigAggregator) build(ctx context.Context, inst *model.Instance) representation.InstanceResponse {
	response := representation.NewInstanceResponse(inst)
	response.Integration.ChatwootConfig = a.chatwootConfig(ctx, inst.ID)

	if inst.Connection.Status != "connected" {
		return response
	}
	var (
		profile       *representation.ProfileResponse
		privacy       *representation.PrivacyResponse
		statusPrivacy *representation.StatusPrivacyResponse
		wg            sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		got, err := a.instances.GetProfile(ctx, inst.ID)
		if err != nil {
			a.warn(inst.ID, "profile", err)
			return
		}
		value := representation.NewProfileResponse(got)
		profile = &value
	}()
	go func() {
		defer wg.Done()
		got, err := a.instances.GetPrivacy(ctx, inst.ID)
		if err != nil {
			a.warn(inst.ID, "privacy", err)
			return
		}
		value := representation.NewPrivacyResponse(got)
		privacy = &value
	}()
	go func() {
		defer wg.Done()
		got, err := a.instances.GetStatusPrivacy(ctx, inst.ID)
		if err != nil {
			a.warn(inst.ID, "status_privacy", err)
			return
		}
		value := representation.NewStatusPrivacyResponse(got)
		statusPrivacy = &value
	}()
	wg.Wait()
	if response.Settings == nil && (profile != nil || privacy != nil || statusPrivacy != nil) {
		response.Settings = &representation.SettingsResponse{}
	}
	if response.Settings == nil {
		return response
	}
	response.Settings.Profile = profile
	response.Settings.Privacy = privacy
	response.Settings.StatusPrivacy = statusPrivacy
	return response
}

// buildAll aggregates every instance with bounded concurrency, preserving
// the input order, so one slow instance cannot serialize the whole list.
func (a InstanceConfigAggregator) buildAll(ctx context.Context, items []model.Instance) []representation.InstanceResponse {
	envelopes := make([]representation.InstanceResponse, len(items))
	sem := make(chan struct{}, InstanceAggregateConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(index int) {
			defer wg.Done()
			defer func() { <-sem }()
			envelopes[index] = a.build(ctx, &items[index])
		}(i)
	}
	wg.Wait()
	return envelopes
}

// chatwootConfig reads the stored Chatwoot connector of an instance as the
// nested integration block: absent when none is stored (the same unset state
// the standalone route answers as a disabled default) and omitted on a read
// failure. The nested copy drops the instance_id — data.instance.id already
// carries it — and never holds the write-only token.
func (a InstanceConfigAggregator) chatwootConfig(ctx context.Context, id uuid.UUID) *representation.ChatwootConfigResponse {
	if a.configs == nil {
		return nil
	}
	cfg, err := a.configs.Get(ctx, id)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			a.warn(id, "chatwoot_config", err)
		}
		return nil
	}
	response := representation.NewChatwootConfigResponse(cfg, representation.ChatwootWebhookURL(a.publicURL, id))
	response.InstanceID = ""
	return &response
}

// warn records a degraded aggregate block: the response still succeeds with
// the block omitted.
func (a InstanceConfigAggregator) warn(id uuid.UUID, block string, err error) {
	a.log.Warn().Str("instance_id", id.String()).Str("block", block).Err(err).
		Msg("instance aggregate block unavailable")
}

// WebhookInput is the nested webhook block of the instance write payloads
// (matrix PATCH/POST /instances: the request mirrors the public
// instance.webhook sub-object). Pointers keep an omitted field distinct from
// an explicit value: a missing URL stays unset, url:"" clears it and
// events:[] clears the subscription.
type WebhookInput struct {
	URL     *string   `json:"url"`
	Enabled *bool     `json:"enabled"`
	Events  *[]string `json:"events"`
}

// CreateInstanceRequest is the POST /instances payload. OwnerUserID is a
// pointer so an absent field is distinct from an explicit value: only the
// global scope and admin sessions may send it. The webhook block follows the
// same absent-versus-explicit rule: an omitted webhook keeps every default,
// while an omitted events field defaults to every type.
type CreateInstanceRequest struct {
	// Globally unique, exact ASCII name: 1-64 letters/digits/hyphens/underscores, alphanumeric ends; stats and every UUID-parseable string are reserved.
	Name        string        `json:"name"`
	ExternalRef string        `json:"external_ref"`
	OwnerUserID *string       `json:"owner_user_id"`
	Webhook     *WebhookInput `json:"webhook"`
}

// UpdateInstanceRequest uses pointers so an omitted field keeps its stored
// value while an explicit empty external_ref clears it. The webhook block
// behaves the same: an omitted webhook keeps the whole stored configuration,
// an explicit empty URL unsets it, and an explicit empty events list clears
// the subscription.
type UpdateInstanceRequest struct {
	// Actual renames follow the create name grammar and uniqueness rule; an exactly unchanged stored name is accepted.
	Name        *string       `json:"name"`
	ExternalRef *string       `json:"external_ref"`
	Webhook     *WebhookInput `json:"webhook"`
}

// HandleCreateInstance registers an instance, assigns its owner and answers
// 201 with the instance plus its one-time plaintext key. The collection is
// outside every instance key scope, so instance credentials answer 403 before
// the body is read. Owner resolution is by scope: a user session always owns
// what it creates (sending owner_user_id is an admin/global privilege and
// answers 403); the global scope and admin sessions create for the requested
// owner_user_id when given and for the oldest admin otherwise (500 when no
// admin exists). An override matching no user answers 422. After owner
// resolution and before Create, a non-admin user session faces the quota
// pre-checks: the global ceiling (CountAll >= MaxInstances when MaxInstances
// is positive) then the owner's stored per-user quota (CountByOwner >=
// quota when the stored quota is positive; a stored 0 means unlimited and
// WZAP_DEFAULT_USER_INSTANCE_QUOTA is never consulted here). Either breach
// answers 403 quota_exceeded. The global scope and admin sessions bypass both
// checks. The check-then-insert race is accepted at product scale (R19): two
// concurrent creates may both pass and both insert; no locking is built.
//
// @Summary Create an instance
// @Description Answers with the aggregated instance DTO: the webhook block nests under integration and the optional settings blocks under settings (optional blocks omitted on an unconfigured instance except the persisted default_disappearing echo).
// @Tags instances
// @Accept json
// @Produce json
// @Security apikey
// @Param request body CreateInstanceRequest true "Instance payload"
// @Success 201 {object} core.Envelope{data=representation.CreateInstanceResponse} "Created instance plus its one-time key, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Forbidden or quota exceeded"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already taken, or external ref already taken"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "invalid_instance_name: invalid/reserved name; unknown owner or invalid webhook config"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances [post]
func HandleCreateInstance(agg InstanceConfigAggregator, users storage.UserRepository, keys storage.APIKeyRepository, maxInstances int) http.HandlerFunc {
	instances := agg.instances
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.AuthorizeCollection(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request CreateInstanceRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}

		scope, ok := auth.ScopeFromContext(r.Context())
		if !ok {
			core.WriteForbidden(w, r)
			return
		}

		var owner uuid.UUID
		if err := auth.RequireRole(scope, "admin"); err == nil {
			if request.OwnerUserID != nil {
				id, err := uuid.Parse(*request.OwnerUserID)
				if err != nil {
					core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid owner_user_id")
					return
				}
				owner = id
			} else {
				admin, err := instances.OldestAdmin(r.Context())
				if err != nil {
					if errors.Is(err, instance.ErrNoAdmin) {
						core.Error(w, r, http.StatusInternalServerError, "internal_error", "no admin user exists")
						return
					}
					core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
					return
				}
				owner = admin
			}
		} else if scope.Kind == auth.ScopeUser {
			if request.OwnerUserID != nil {
				core.WriteForbidden(w, r)
				return
			}
			owner = scope.UserID
		} else {
			core.WriteForbidden(w, r)
			return
		}

		if err := CheckCreateQuotas(r, scope, owner, users, keys, maxInstances); err != nil {
			WriteQuotaError(w, r, err)
			return
		}

		var webhookURL *string
		var webhookEnabled *bool
		var webhookEvents *[]string
		if request.Webhook != nil {
			webhookURL = request.Webhook.URL
			webhookEnabled = request.Webhook.Enabled
			webhookEvents = request.Webhook.Events
		}
		created, key, err := instances.Create(r.Context(), instance.CreateInput{
			Name:           request.Name,
			ExternalRef:    request.ExternalRef,
			OwnerUserID:    &owner,
			WebhookURL:     webhookURL,
			WebhookEnabled: webhookEnabled,
			WebhookEvents:  webhookEvents,
		})
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		webhook.Keys.Store(created.ID, key)
		core.JSON(w, r, http.StatusCreated, representation.NewCreateInstanceResponse(agg.build(r.Context(), created), key))
	}
}

// ErrQuotaExceeded reports that a create-time quota check denied the request.
// The handler maps it to 403 quota_exceeded.
var ErrQuotaExceeded = errors.New("quota exceeded")

// CheckCreateQuotas enforces the global and per-user instance quotas for a
// non-admin user session creating for owner. Admin-equivalent creators (the
// global scope or an admin session) skip both checks entirely. The counts are
// unfiltered: every existing instance counts, any state. A non-positive
// MaxInstances disables the global check and a stored per-user quota of 0
// means unlimited for that check; the creation-time default quota is never
// consulted here. The checks are fail-closed: a nil keys or users dependency
// is a wiring error that answers 500 instead of silently disabling
// enforcement (production always wires both).
func CheckCreateQuotas(r *http.Request, scope auth.Scope, owner uuid.UUID, users storage.UserRepository, keys storage.APIKeyRepository, maxInstances int) error {
	if err := auth.RequireRole(scope, "admin"); err == nil {
		return nil
	}
	if scope.Kind != auth.ScopeUser {
		return nil
	}

	if keys == nil {
		return fmt.Errorf("check create quotas: keys repository is not configured")
	}
	if maxInstances > 0 {
		total, err := keys.CountAll(r.Context())
		if err != nil {
			return err
		}
		if total >= maxInstances {
			return ErrQuotaExceeded
		}
	}

	if users == nil {
		return fmt.Errorf("check create quotas: users repository is not configured")
	}
	ownerUser, err := users.GetByID(r.Context(), owner)
	if err != nil {

		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return err
	}
	if ownerUser.InstanceQuota <= 0 {
		return nil
	}
	owned, err := keys.CountByOwner(r.Context(), owner)
	if err != nil {
		return err
	}
	if owned >= ownerUser.InstanceQuota {
		return ErrQuotaExceeded
	}
	return nil
}

// WriteQuotaError maps a quota pre-check failure to its HTTP status: a breach
// answers 403 quota_exceeded, anything else a 500 without leaking its cause.
func WriteQuotaError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrQuotaExceeded) {
		core.Error(w, r, http.StatusForbidden, "quota_exceeded", "quota exceeded")
		return
	}
	core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
}

// HandleListInstances answers every authorized instance. An
// instance key owns no collection view and answers 403; a user session sees
// exactly its own rows while the global scope and admin sessions see all.
// The aggregated DTO enriches the live settings blocks concurrently with a
// bounded worker pool, omitting unavailable blocks.
//
// @Summary List instances
// @Tags instances
// @Produce json
// @Security apikey
// @Success 200 {object} core.Envelope{data=representation.InstanceListResponse} "All authorized instances, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Instance keys own no collection view"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances [get]
func HandleListInstances(agg InstanceConfigAggregator) http.HandlerFunc {
	instances := agg.instances
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.AuthorizeCollection(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		items, err := instances.List(r.Context())
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}

		if scope, ok := auth.ScopeFromContext(r.Context()); ok {
			items = core.FilterInstancesByOwner(scope, items)
		} else {
			items = nil
		}

		response := representation.InstanceListResponse{Instances: agg.buildAll(r.Context(), items)}
		core.JSON(w, r, http.StatusOK, response)
	}
}

// HandleInstanceStats answers the scoped instance counts for the overview
// Home: the total plus the breakdown by connection status. An instance key
// owns no collection view and answers 403 like the list; a user session
// counts exactly its own rows while the global scope and admin sessions count
// all. A service failure answers 500.
//
// @Summary Instance stats
// @Tags instances
// @Produce json
// @Security apikey
// @Param instance query string false "Optional instance UUID or name (exact, case-sensitive); omitted counts the authorized collection"
// @Success 200 {object} core.Envelope{data=representation.StatsEnvelope} "Totals in scope, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Instance keys own no collection view, or target not owned"
// @Failure 404 {object} core.ErrorEnvelope "Target instance not found"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/stats [get]
func HandleInstanceStats(instances InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := core.AuthorizeCollection(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}
		scope, ok := auth.ScopeFromContext(r.Context())
		if !ok {
			core.WriteForbidden(w, r)
			return
		}

		byStatus := map[string]int{
			"connected":    0,
			"disconnected": 0,
			"pairing":      0,
			"error":        0,
		}
		var items []model.Instance
		if reference := r.URL.Query().Get("instance"); reference != "" {
			target, err := core.ResolveInstanceReference(r, instances, reference, true)
			if err != nil {
				core.WriteInstanceError(w, r, err)
				return
			}
			if err := core.AuthorizeInstance(r, target); err != nil {
				core.WriteForbidden(w, r)
				return
			}
			items = []model.Instance{*target}
		} else {
			var err error
			items, err = instances.List(r.Context())
			if err != nil {
				core.WriteInstanceError(w, r, err)
				return
			}
			items = core.FilterInstancesByOwner(scope, items)
		}
		for _, item := range items {
			if _, known := byStatus[item.Connection.Status]; known {
				byStatus[item.Connection.Status]++
			} else {
				byStatus["disconnected"]++
			}
		}
		core.JSON(w, r, http.StatusOK, representation.StatsEnvelope{Stats: representation.InstanceStatsResponse{Total: len(items), ByStatus: byStatus}})
	}
}

// HandleGetInstance answers one instance by id. The target loads first so a
// missing id answers 404 before the ownership check can deny with 403.
//
// @Summary Get an instance
// @Tags instances
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=representation.InstanceEnvelope} "Instance, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id} [get]
func HandleGetInstance(agg InstanceConfigAggregator) http.HandlerFunc {
	instances := agg.instances
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		found, err := core.LoadInstance(r, instances, id)
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, found); err != nil {
			core.WriteForbidden(w, r)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.InstanceEnvelope{Instance: agg.build(r.Context(), found)})
	}
}

// HandleUpdateInstance applies a partial update and answers 200 with the stored
// instance. It loads the target first (404) and authorizes (403) before
// reading the body or writing anything.
//
// @Summary Update an instance
// @Tags instances
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body UpdateInstanceRequest true "Partial update payload"
// @Success 200 {object} core.Envelope{data=representation.InstanceEnvelope} "Updated instance, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body or invalid cursor"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already taken, or external ref already taken; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "invalid_instance_name: invalid/reserved changed name, or invalid webhook config"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id} [patch]
func HandleUpdateInstance(agg InstanceConfigAggregator) http.HandlerFunc {
	instances := agg.instances
	return func(w http.ResponseWriter, r *http.Request) {
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

		var request UpdateInstanceRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}

		var webhookURL *string
		var webhookEnabled *bool
		var webhookEvents *[]string
		if request.Webhook != nil {
			webhookURL = request.Webhook.URL
			webhookEnabled = request.Webhook.Enabled
			webhookEvents = request.Webhook.Events
		}
		updated, err := instances.Update(r.Context(), id, instance.UpdateInput{
			Name:           request.Name,
			ExternalRef:    request.ExternalRef,
			WebhookURL:     webhookURL,
			WebhookEnabled: webhookEnabled,
			WebhookEvents:  webhookEvents,
		})
		if err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.InstanceEnvelope{Instance: agg.build(r.Context(), updated)})
	}
}

// HandleDeleteInstance removes an instance and answers 204. It loads the
// target first (404) and authorizes (403) before removing anything.
//
// @Summary Delete an instance
// @Tags instances
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 204 "Deleted, no body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id} [delete]
func HandleDeleteInstance(instances InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if err := instances.Delete(r.Context(), id); err != nil {
			core.WriteInstanceError(w, r, err)
			return
		}
		webhook.Keys.Clear(id)
		core.JSON(w, r, http.StatusNoContent, nil)
	}
}

type ChatwootConfigStore interface {
	Get(context.Context, uuid.UUID) (*model.ChatwootConfig, error)
}
