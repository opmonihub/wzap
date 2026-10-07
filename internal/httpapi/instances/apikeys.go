package instances

import (
	"errors"
	"net/http"

	"wzap/internal/auth"
	"wzap/internal/httpapi/core"
	"wzap/internal/instance"
	"wzap/internal/storage"
	"wzap/internal/webhook"
)

// RotateAPIKeyResponse is the 200 answer to a rotation: the instance id with
// its one-time plaintext key. The key travels in this response only and is
// never persisted; only its hash reaches the repository.
type RotateAPIKeyResponse struct {
	ID             string `json:"id" binding:"required"`
	InstanceAPIKey string `json:"instance_api_key" binding:"required"`
}

// HandleRotateAPIKey mints a fresh instance key and stores its hash,
// overwriting any prior hash so the old key dies at write. It loads the
// target first (404) and then requires the global scope or an admin session
// (403), so an instance key — even the instance's own — cannot rotate its
// own key. Rotating a keyless instance issues its first key through
// the same path; no rotation history is kept.
//
// @Summary Rotate an instance API key
// @Tags apikeys
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=RotateAPIKeyResponse} "Fresh key, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Requires global or admin scope"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/apikey/rotate [post]
func HandleRotateAPIKey(instances InstanceService, keys storage.APIKeyRepository) http.HandlerFunc {
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
		if err := core.RequireAdminScope(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		key, hash, err := auth.MintAPIKey()
		if err != nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if keys == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if err := keys.SetHash(r.Context(), stored.ID, hash); err != nil {
			WriteKeyError(w, r, err)
			return
		}
		webhook.Keys.Store(stored.ID, key)
		core.JSON(w, r, http.StatusOK, RotateAPIKeyResponse{ID: stored.ID.String(), InstanceAPIKey: key})
	}
}

// HandleRevokeAPIKey revokes the instance key and answers 204. It loads the
// target first (404) and then requires the global scope or an admin session
// (403). After revoke the instance answers only to the global key and admin
// sessions until the next rotation. Revoking an already-keyless instance
// still succeeds.
//
// @Summary Revoke an instance API key
// @Tags apikeys
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 204 "Revoked, no body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Requires global or admin scope"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/apikey [delete]
func HandleRevokeAPIKey(instances InstanceService, keys storage.APIKeyRepository) http.HandlerFunc {
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
		if err := core.RequireAdminScope(r); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		if keys == nil {
			core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if err := keys.ClearHash(r.Context(), stored.ID); err != nil {
			WriteKeyError(w, r, err)
			return
		}
		webhook.Keys.Clear(stored.ID)
		core.JSON(w, r, http.StatusNoContent, nil)
	}
}

// requireAdminScope reports whether the request acts as the global scope or
// an admin session. Instance scopes and non-admin user sessions are denied.

// WriteKeyError maps a key repository failure to its HTTP status: a missing
// instance answers 404, anything else a 500 without leaking its cause.
func WriteKeyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, instance.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		core.Error(w, r, http.StatusNotFound, "not_found", "instance not found")
	default:
		core.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
