package httpapi

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// presenceRequest is the POST /instances/{id}/presence payload. State is one
// of composing, paused (chat presence) or available, unavailable (user
// presence); there is no continuous mode, one call publishes one signal.
type presenceRequest struct {
	Chat  string `json:"chat"`
	State string `json:"state"`
}

// presenceResponse is the 200 answer to a published presence signal.
type presenceResponse struct {
	Sent bool `json:"sent"`
}

// validPresenceState reports whether state is one of the presence signals the
// session accepts, mirroring the adapter allowlist.
func validPresenceState(state string) bool {
	switch state {
	case "composing", "paused", "available", "unavailable":
		return true
	}
	return false
}

// handleSendPresence publishes one presence signal and answers 200. It loads
// the target first (404) and authorizes (403) before reading the body or
// touching the session; an unknown state answers 422 before the session and
// an offline session answers 409.
//
// @Summary Send a presence signal
// @Tags presence
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body presenceRequest true "Presence payload"
// @Success 200 {object} envelope{data=presenceResponse} "Published, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid chat or unknown state"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/presence [post]
func handleSendPresence(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "presence").Msg("presence request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "presence").Err(err).Msg("presence failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request presenceRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		chat := strings.TrimSpace(request.Chat)
		if chat == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}
		state := strings.TrimSpace(request.State)
		if !validPresenceState(state) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "state must be composing, paused, available or unavailable")
			return
		}

		if err := instances.SendPresence(r.Context(), id, chat, state); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "presence").Err(err).Msg("presence failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "presence").Str("state", state).Bool("sent", true).Msg("presence result")
		JSON(w, r, http.StatusOK, presenceResponse{Sent: true})
	}
}
