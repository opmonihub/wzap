package instances

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// PresenceRequest is the POST /instances/{id}/presence payload. State is one
// of composing, paused (chat presence) or available, unavailable (user
// presence); there is no continuous mode, one call publishes one signal.
type PresenceRequest struct {
	Chat  string `json:"chat"`
	State string `json:"state"`
}

// PresenceResponse is the 200 answer to a published presence signal.
type PresenceResponse struct {
	Sent bool `json:"sent" binding:"required"`
}

// ValidPresenceState reports whether state is one of the presence signals the
// session accepts, mirroring the adapter allowlist.
func ValidPresenceState(state string) bool {
	switch state {
	case "composing", "paused", "available", "unavailable":
		return true
	}
	return false
}

// HandleSendPresence publishes one presence signal and answers 200. It loads
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
// @Param request body PresenceRequest true "Presence payload"
// @Success 200 {object} core.Envelope{data=PresenceResponse} "Published, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid chat or unknown state"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/presence [post]
func HandleSendPresence(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "presence").Msg("presence request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "presence").Err(err).Msg("presence failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request PresenceRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		chat := strings.TrimSpace(request.Chat)
		if chat == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}
		state := strings.TrimSpace(request.State)
		if !ValidPresenceState(state) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "state must be composing, paused, available or unavailable")
			return
		}

		if err := instances.SendPresence(r.Context(), id, chat, state); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "presence").Err(err).Msg("presence failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "presence").Str("state", state).Bool("sent", true).Msg("presence result")
		core.JSON(w, r, http.StatusOK, PresenceResponse{Sent: true})
	}
}
