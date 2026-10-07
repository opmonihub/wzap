package chats

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// DisappearingRequest is the PUT disappearing timer payload: a string duration
// such as "24h", with "0" or "0s" switching the timer off.
type DisappearingRequest struct {
	Duration string `json:"duration"`
}

// DisappearingUpdatedResponse acknowledges a timer command. Reading the
// applied duration remains the responsibility of the disappearing GET route.
type DisappearingUpdatedResponse struct {
	Updated bool `json:"updated" binding:"required"`
}

// ParseDisappearingDuration parses the raw duration and validates it against
// the upstream timer allowlist of off/24h/168h/2160h. Anything else answers
// 422 without touching the session.
func ParseDisappearingDuration(raw string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, false
	}
	switch duration {
	case 0, 24 * time.Hour, 168 * time.Hour, 2160 * time.Hour:
		return duration, true
	}
	return 0, false
}

// HandleSetDisappearing sets the disappearing timer of a chat and answers 200
// with the command acknowledgement. A duration outside the 0/24h/168h/2160h allowlist
// answers 422 before the session is touched.
//
// @Summary Set the disappearing timer of a chat
// @Tags chats
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param chat path string true "Chat JID"
// @Param request body DisappearingRequest true "Timer payload, one of 0, 24h, 168h, 2160h"
// @Success 200 {object} core.Envelope{data=DisappearingUpdatedResponse} "Timer updated, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid chat or duration outside the allowlist"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/{chat}/disappearing [put]
func HandleSetDisappearing(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "chats-disappearing-set")
		if !ok {
			return
		}
		chat := strings.TrimSpace(core.PathParam(r, "chat"))
		if chat == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}

		var request DisappearingRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		duration, ok := ParseDisappearingDuration(request.Duration)
		if !ok {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "duration must be one of 0, 24h, 168h, 2160h")
			return
		}

		if err := instances.SetDisappearingTimer(r.Context(), id, chat, duration); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "chats-disappearing-set").Err(err).Msg("set disappearing timer failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, DisappearingUpdatedResponse{Updated: true})
	}
}

// HandleSetDefaultDisappearing sets the default disappearing timer for new
// chats and answers 200 with the command acknowledgement. A duration outside the
// 0/24h/168h/2160h allowlist answers 422 before the session is touched.
//
// @Summary Set the default disappearing timer
// @Tags chats
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body DisappearingRequest true "Timer payload, one of 0, 24h, 168h, 2160h"
// @Success 200 {object} core.Envelope{data=DisappearingUpdatedResponse} "Default timer updated, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Duration outside the allowlist"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/default-disappearing [put]
func HandleSetDefaultDisappearing(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "chats-disappearing-default")
		if !ok {
			return
		}

		var request DisappearingRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		duration, ok := ParseDisappearingDuration(request.Duration)
		if !ok {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "duration must be one of 0, 24h, 168h, 2160h")
			return
		}

		if err := instances.SetDefaultDisappearingTimer(r.Context(), id, duration); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "chats-disappearing-default").Err(err).Msg("set default disappearing timer failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, DisappearingUpdatedResponse{Updated: true})
	}
}
