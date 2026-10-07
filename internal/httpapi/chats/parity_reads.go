package chats

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// DisappearingResponse answers the disappearing timer of a chat: the duration
// in seconds with found false when the chat carries no timer.
type DisappearingResponse struct {
	Chat            string `json:"chat" binding:"required"`
	DurationSeconds int64  `json:"duration_seconds" binding:"required"`
	Found           bool   `json:"found" binding:"required"`
}

// HandleGetDisappearing answers the disappearing timer of a chat, with found
// false when the chat carries no timer. A blank chat answers 422 and a
// disconnected session 409.
//
// @Summary Get the disappearing timer of a chat
// @Tags chats
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param chat path string true "Chat JID"
// @Success 200 {object} core.Envelope{data=DisappearingResponse} "Timer, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Invalid chat"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/{chat}/disappearing [get]
func HandleGetDisappearing(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "chats-disappearing-get")
		if !ok {
			return
		}
		chat := strings.TrimSpace(core.PathParam(r, "chat"))
		if chat == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}

		duration, found, err := instances.GetDisappearingTimer(r.Context(), id, chat)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "chats-disappearing-get").Err(err).Msg("get disappearing timer failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, DisappearingResponse{
			Chat:            chat,
			DurationSeconds: int64(duration / time.Second),
			Found:           found,
		})
	}
}
