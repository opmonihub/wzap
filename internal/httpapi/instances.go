package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"wzap/internal/auth"
	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/session"
	"wzap/internal/storage"
	"wzap/internal/webhook"
)

const (
	// maxJSONBodyBytes caps the JSON request bodies every handler decodes.
	maxJSONBodyBytes = 1 << 20
)

// InstanceService is the instance management contract consumed by the handlers.
type InstanceService interface {
	Create(ctx context.Context, input instance.CreateInput) (*model.Instance, string, error)
	// OldestAdmin resolves the owner of an instance created by the global key
	// or an admin session without an explicit owner.
	OldestAdmin(ctx context.Context) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)
	GetByName(ctx context.Context, name string) (*model.Instance, error)
	List(ctx context.Context) ([]model.Instance, error)
	Update(ctx context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Disconnect(ctx context.Context, id uuid.UUID) error
	Connect(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)
	QR(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)
	RevokeMessage(ctx context.Context, id uuid.UUID, chatJID, messageID string) error
	MarkRead(ctx context.Context, id uuid.UUID, chatJID, senderJID, messageID string) error
	SendPresence(ctx context.Context, id uuid.UUID, chatJID, state string) error
	PairPhone(ctx context.Context, id uuid.UUID, phone string) (instance.PairPhoneResult, error)
	CreateGroup(ctx context.Context, id uuid.UUID, input instance.CreateGroupInput) (instance.Group, error)
	GetGroup(ctx context.Context, id uuid.UUID, groupJID string) (instance.Group, error)
	UpdateGroup(ctx context.Context, id uuid.UUID, groupJID string, input instance.UpdateGroupInput) (instance.Group, error)
	SetGroupPhoto(ctx context.Context, id uuid.UUID, groupJID string, image []byte) error
	UpdateGroupParticipants(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error
	GetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	ResetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	JoinGroup(ctx context.Context, id uuid.UUID, inviteCode string) (string, error)
	LeaveGroup(ctx context.Context, id uuid.UUID, groupJID string) error
	FollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error
	UnfollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error
	GetNewsletter(ctx context.Context, id uuid.UUID, channelJID string) (instance.Newsletter, error)
	ListNewsletters(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Newsletter, string, error)
	PublishStatus(ctx context.Context, id uuid.UUID, input session.StatusInput) (string, error)
	ListStatuses(ctx context.Context, id uuid.UUID) ([]session.StatusInfo, error)
	DeleteStatus(ctx context.Context, id uuid.UUID, statusID string) error
	RejectCall(ctx context.Context, id uuid.UUID, fromJID, callID string) error
	GetProfile(ctx context.Context, id uuid.UUID) (session.Profile, error)
	SetProfileName(ctx context.Context, id uuid.UUID, name string) error
	SetProfileStatusText(ctx context.Context, id uuid.UUID, text string) error
	SetProfilePhoto(ctx context.Context, id uuid.UUID, image []byte) error
	GetPrivacy(ctx context.Context, id uuid.UUID) (session.Privacy, error)
	SetPrivacy(ctx context.Context, id uuid.UUID, input session.Privacy) (session.Privacy, error)
	// Fase 1 parity reads and edit (whatsmeow-parity-routes): the service
	// signatures mirror internal/instance parity_edits.go and parity_reads.go
	// so the handlers below stay thin transport adapters.
	EditMessage(ctx context.Context, id uuid.UUID, chatJID, messageID, text string) (string, error)
	GetJoinedGroups(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Group, string, error)
	GetGroupInvitePreview(ctx context.Context, id uuid.UUID, inviteCode string) (instance.Group, error)
	CheckContacts(ctx context.Context, id uuid.UUID, phones []string) ([]session.ContactCheckResult, error)
	GetContactDevices(ctx context.Context, id uuid.UUID, jid string) ([]string, error)
	GetContactPhoto(ctx context.Context, id uuid.UUID, jid string) (session.ProfilePictureInfo, error)
	GetContactBusiness(ctx context.Context, id uuid.UUID, jid string) (session.BusinessProfile, error)
	GetBlocklist(ctx context.Context, id uuid.UUID) ([]string, error)
	GetStatusPrivacy(ctx context.Context, id uuid.UUID) (session.StatusPrivacy, error)
	GetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string) (time.Duration, bool, error)
	GetNewsletterMessages(ctx context.Context, id uuid.UUID, channel, cursor string, limit int) ([]session.NewsletterMessage, string, error)
	GetNewsletterUpdates(ctx context.Context, id uuid.UUID, channel string) ([]session.NewsletterMessage, error)
	// Fase 2-3 parity moderation and writes (whatsmeow-parity-routes): the
	// service signatures mirror internal/instance parity_moderation.go and
	// parity_writes.go so the handlers stay thin transport adapters.
	GetGroupRequests(ctx context.Context, id uuid.UUID, groupJID string) ([]instance.GroupParticipant, error)
	UpdateGroupRequests(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error
	UpdateGroupSettings(ctx context.Context, id uuid.UUID, groupJID string, announce, locked *bool, joinApproval, memberAddMode *string) (instance.Group, error)
	UpdateBlocklist(ctx context.Context, id uuid.UUID, jid, action string) error
	SetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string, duration time.Duration) error
	SetDefaultDisappearingTimer(ctx context.Context, id uuid.UUID, duration time.Duration) error
	SubscribePresence(ctx context.Context, id uuid.UUID, jid string) error
	GetContactQRLink(ctx context.Context, id uuid.UUID, revoke bool) (string, error)
	CreateNewsletter(ctx context.Context, id uuid.UUID, title, description string) (instance.Newsletter, error)
	MuteNewsletter(ctx context.Context, id uuid.UUID, channel string, muted bool) error
	MarkNewsletterViewed(ctx context.Context, id uuid.UUID, channel string, serverIDs []string) error
	ReactNewsletter(ctx context.Context, id uuid.UUID, channel, serverID, reaction string) error
}

// The service satisfies the handler contract; the assertion catches signature
// drift at build time.
var _ InstanceService = (*instance.Service)(nil)

// webhookInput is the nested webhook block of the instance write payloads
// (matrix PATCH/POST /instances: the request mirrors the public
// instance.webhook sub-object). Pointers keep an omitted field distinct from
// an explicit value: a missing URL stays unset, url:"" clears it and
// events:[] clears the subscription.
type webhookInput struct {
	URL     *string   `json:"url"`
	Enabled *bool     `json:"enabled"`
	Events  *[]string `json:"events"`
}

// createInstanceRequest is the POST /instances payload. OwnerUserID is a
// pointer so an absent field is distinct from an explicit value: only the
// global scope and admin sessions may send it. The webhook block follows the
// same absent-versus-explicit rule: an omitted webhook keeps every default,
// while an omitted events field defaults to every type.
type createInstanceRequest struct {
	// Globally unique, exact ASCII name: 1-64 letters/digits/hyphens/underscores, alphanumeric ends; stats and every UUID-parseable string are reserved.
	Name        string        `json:"name"`
	ExternalRef string        `json:"external_ref"`
	OwnerUserID *string       `json:"owner_user_id"`
	Webhook     *webhookInput `json:"webhook"`
}

// updateInstanceRequest uses pointers so an omitted field keeps its stored
// value while an explicit empty external_ref clears it. The webhook block
// behaves the same: an omitted webhook keeps the whole stored configuration,
// an explicit empty URL unsets it, and an explicit empty events list clears
// the subscription.
type updateInstanceRequest struct {
	// Actual renames follow the create name grammar and uniqueness rule; an exactly unchanged legacy name is accepted.
	Name        *string       `json:"name"`
	ExternalRef *string       `json:"external_ref"`
	Webhook     *webhookInput `json:"webhook"`
}

// handleCreateInstance registers an instance, assigns its owner and answers
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
// @Tags instances
// @Accept json
// @Produce json
// @Security apikey
// @Param request body createInstanceRequest true "Instance payload"
// @Success 201 {object} envelope{data=createInstanceResponse} "Created, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Forbidden or quota exceeded"
// @Failure 409 {object} errorEnvelope "instance_name_taken: name already taken, or external ref already taken"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "invalid_instance_name: invalid/reserved name; unknown owner or invalid webhook config"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances [post]
func handleCreateInstance(instances InstanceService, users storage.UserRepository, keys storage.APIKeyRepository, maxInstances int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := authorizeCollection(r); err != nil {
			writeForbidden(w, r)
			return
		}

		var request createInstanceRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}

		scope, ok := auth.ScopeFromContext(r.Context())
		if !ok {
			writeForbidden(w, r)
			return
		}

		var owner uuid.UUID
		if err := auth.RequireRole(scope, "admin"); err == nil {
			if request.OwnerUserID != nil {
				id, err := uuid.Parse(*request.OwnerUserID)
				if err != nil {
					Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid owner_user_id")
					return
				}
				owner = id
			} else {
				admin, err := instances.OldestAdmin(r.Context())
				if err != nil {
					if errors.Is(err, instance.ErrNoAdmin) {
						Error(w, r, http.StatusInternalServerError, "internal_error", "no admin user exists")
						return
					}
					Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
					return
				}
				owner = admin
			}
		} else if scope.Kind == auth.ScopeUser {
			if request.OwnerUserID != nil {
				writeForbidden(w, r)
				return
			}
			owner = scope.UserID
		} else {
			writeForbidden(w, r)
			return
		}

		if err := checkCreateQuotas(r, scope, owner, users, keys, maxInstances); err != nil {
			writeQuotaError(w, r, err)
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
			writeInstanceError(w, r, err)
			return
		}
		webhook.Keys.Store(created.ID, key)
		JSON(w, r, http.StatusCreated, newCreateInstanceResponse(created, key))
	}
}

// errQuotaExceeded reports that a create-time quota check denied the request.
// The handler maps it to 403 quota_exceeded.
var errQuotaExceeded = errors.New("quota exceeded")

// checkCreateQuotas enforces the global and per-user instance quotas for a
// non-admin user session creating for owner. Admin-equivalent creators (the
// global scope or an admin session) skip both checks entirely. The counts are
// unfiltered: every existing instance counts, any state. A non-positive
// MaxInstances disables the global check and a stored per-user quota of 0
// means unlimited for that check; the creation-time default quota is never
// consulted here. The checks are fail-closed: a nil keys or users dependency
// is a wiring error that answers 500 instead of silently disabling
// enforcement (production always wires both).
func checkCreateQuotas(r *http.Request, scope auth.Scope, owner uuid.UUID, users storage.UserRepository, keys storage.APIKeyRepository, maxInstances int) error {
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
			return errQuotaExceeded
		}
	}

	if users == nil {
		return fmt.Errorf("check create quotas: users repository is not configured")
	}
	ownerUser, err := users.GetByID(r.Context(), owner)
	if err != nil {
		// An unknown owner stays the service's 422: skip the per-user check
		// and let Create validate the owner.
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
		return errQuotaExceeded
	}
	return nil
}

// writeQuotaError maps a quota pre-check failure to its HTTP status: a breach
// answers 403 quota_exceeded, anything else a 500 without leaking its cause.
func writeQuotaError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errQuotaExceeded) {
		Error(w, r, http.StatusForbidden, "quota_exceeded", "quota exceeded")
		return
	}
	Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
}

// instanceStatsResponse is the JSON representation of GET /instances/stats:
// the total in scope plus the breakdown by connection status. Unknown
// statuses fold into disconnected so the four buckets always sum to total.
type instanceStatsResponse struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

// handleListInstances answers every authorized instance. An
// instance key owns no collection view and answers 403; a user session sees
// exactly its own rows while the global scope and admin sessions see all.
//
// @Summary List instances
// @Tags instances
// @Produce json
// @Security apikey
// @Success 200 {object} envelope{data=instanceListResponse} "All authorized instances, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Instance keys own no collection view"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances [get]
func handleListInstances(instances InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := authorizeCollection(r); err != nil {
			writeForbidden(w, r)
			return
		}

		items, err := instances.List(r.Context())
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}

		if scope, ok := auth.ScopeFromContext(r.Context()); ok {
			items = filterInstancesByOwner(scope, items)
		} else {
			items = nil
		}

		response := instanceListResponse{Items: make([]instanceEnvelope, 0, len(items))}
		for i := range items {
			response.Items = append(response.Items, instanceEnvelope{Instance: newInstanceResponse(&items[i])})
		}
		JSON(w, r, http.StatusOK, response)
	}
}

// handleInstanceStats answers the scoped instance counts for the overview
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
// @Success 200 {object} envelope{data=statsEnvelope} "Totals in scope, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Instance keys own no collection view, or target not owned"
// @Failure 404 {object} errorEnvelope "Target instance not found"
// @Failure 409 {object} errorEnvelope "instance_name_ambiguous: legacy name matches multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/stats [get]
func handleInstanceStats(instances InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := authorizeCollection(r); err != nil {
			writeForbidden(w, r)
			return
		}
		scope, ok := auth.ScopeFromContext(r.Context())
		if !ok {
			writeForbidden(w, r)
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
			target, err := resolveInstanceReference(r, instances, reference, true)
			if err != nil {
				writeInstanceError(w, r, err)
				return
			}
			if err := authorizeInstance(r, target); err != nil {
				writeForbidden(w, r)
				return
			}
			items = []model.Instance{*target}
		} else {
			var err error
			items, err = instances.List(r.Context())
			if err != nil {
				writeInstanceError(w, r, err)
				return
			}
			items = filterInstancesByOwner(scope, items)
		}
		for _, item := range items {
			if _, known := byStatus[item.Connection.Status]; known {
				byStatus[item.Connection.Status]++
			} else {
				byStatus["disconnected"]++
			}
		}
		JSON(w, r, http.StatusOK, statsEnvelope{Stats: instanceStatsResponse{Total: len(items), ByStatus: byStatus}})
	}
}

// handleGetInstance answers one instance by id. The target loads first so a
// missing id answers 404 before the ownership check can deny with 403.
//
// @Summary Get an instance
// @Tags instances
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} envelope{data=instanceEnvelope} "Instance, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 409 {object} errorEnvelope "instance_name_ambiguous: legacy name matches multiple instances"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id} [get]
func handleGetInstance(instances InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		found, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, found); err != nil {
			writeForbidden(w, r)
			return
		}
		JSON(w, r, http.StatusOK, instanceEnvelope{Instance: newInstanceResponse(found)})
	}
}

// handleUpdateInstance applies a partial update and answers 200 with the stored
// instance. It loads the target first (404) and authorizes (403) before
// reading the body or writing anything.
//
// @Summary Update an instance
// @Tags instances
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body updateInstanceRequest true "Partial update payload"
// @Success 200 {object} envelope{data=instanceEnvelope} "Updated instance, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body or invalid cursor"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "instance_name_taken: name already taken, or external ref already taken; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "invalid_instance_name: invalid/reserved changed name, or invalid webhook config"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id} [patch]
func handleUpdateInstance(instances InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request updateInstanceRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
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
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, instanceEnvelope{Instance: newInstanceResponse(updated)})
	}
}

// handleDeleteInstance removes an instance and answers 204. It loads the
// target first (404) and authorizes (403) before removing anything.
//
// @Summary Delete an instance
// @Tags instances
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 204 "Deleted, no body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 409 {object} errorEnvelope "instance_name_ambiguous: legacy name matches multiple instances"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id} [delete]
func handleDeleteInstance(instances InstanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		if err := instances.Delete(r.Context(), id); err != nil {
			writeInstanceError(w, r, err)
			return
		}
		webhook.Keys.Clear(id)
		JSON(w, r, http.StatusNoContent, nil)
	}
}

// instanceID parses the canonical UUID path value supplied by the resolver.
// A malformed value answers 404 for handlers used outside the registered mux.
func instanceID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		Error(w, r, http.StatusNotFound, "not_found", "instance not found")
		return uuid.Nil, false
	}
	return id, true
}

// parseLimit reads a limit query parameter, falling back to fallback when it is
// missing or malformed and capping the page size at maxLimit.
func parseLimit(raw string, fallback, maxLimit int) int {
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return fallback
	}
	return min(limit, maxLimit)
}

// decodeJSONBody decodes the request body into target, refusing bodies above
// maxJSONBodyBytes before they are buffered.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	return json.NewDecoder(r.Body).Decode(target)
}

// writeJSONBodyError maps a body decoding failure to its HTTP status: an
// oversized body answers 413 and anything else a malformed 400.
func writeJSONBodyError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large",
			"request body exceeds the 1 MiB limit")
		return
	}
	Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
}

// writeInstanceError maps a service error to its HTTP status and error
// envelope. Scope denials answer 403, unknown owner overrides and invalid
// webhook configurations answer 422, a missing oldest admin answers 500 with
// a clear server-state message; unknown failures answer 500 without leaking
// their cause.
func writeInstanceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		Error(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, instance.ErrNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "instance not found")
	case errors.Is(err, instance.ErrInvalidInstanceName):
		Error(w, r, http.StatusUnprocessableEntity, "invalid_instance_name", "invalid instance name")
	case errors.Is(err, instance.ErrInstanceNameTaken):
		Error(w, r, http.StatusConflict, "instance_name_taken", "instance name already taken")
	case errors.Is(err, instance.ErrInstanceNameAmbiguous):
		Error(w, r, http.StatusConflict, "instance_name_ambiguous", "instance name matches multiple instances")
	case errors.Is(err, instance.ErrExternalRefTaken):
		Error(w, r, http.StatusConflict, "conflict", "external ref already taken")
	case errors.Is(err, instance.ErrInvalidCursor):
		Error(w, r, http.StatusBadRequest, "invalid_request", "invalid cursor")
	case errors.Is(err, instance.ErrAlreadyConnected):
		Error(w, r, http.StatusConflict, "conflict", "instance already connected")
	case errors.Is(err, instance.ErrNotConnected):
		Error(w, r, http.StatusConflict, "conflict", "instance not connected")
	// Um socket morto pode vazar como session.ErrNotConnected sem passar
	// pelo mapSessionError do serviço (os caminhos Connect/QR embrulham o
	// erro da sessão direto): ele também é 409, nunca 500.
	case errors.Is(err, session.ErrNotConnected):
		Error(w, r, http.StatusConflict, "conflict", "instance not connected")
	case errors.Is(err, instance.ErrNoPairingChannel):
		Error(w, r, http.StatusConflict, "conflict", "no open pairing channel")
	case errors.Is(err, instance.ErrInvalidInput):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid operation input")
	case errors.Is(err, instance.ErrGroupNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "group not found")
	// ErrContactNotFound stays distinct from ErrGroupNotFound so a missing
	// contact never surfaces with a misleading group message. The transversal
	// wiring in Task 9 keeps this mapping; it lives here already so the
	// Fase-1 directory reads answer 404 before the routes land.
	case errors.Is(err, instance.ErrContactNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "contact not found")
	case errors.Is(err, instance.ErrNewsletterNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "newsletter not found")
	case errors.Is(err, instance.ErrStatusNotFound):
		Error(w, r, http.StatusNotFound, "not_found", "status not found")
	case errors.Is(err, instance.ErrUnsupported):
		Error(w, r, http.StatusNotImplemented, "not_supported", "operation not supported by the upstream")
	case errors.Is(err, instance.ErrForbidden):
		Error(w, r, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, instance.ErrOwnerNotFound):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "unknown owner")
	case errors.Is(err, instance.ErrInvalidWebhook):
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid webhook config")
	case errors.Is(err, instance.ErrNoAdmin):
		Error(w, r, http.StatusInternalServerError, "internal_error", "no admin user exists")
	default:
		Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
