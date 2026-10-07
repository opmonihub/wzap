package messages

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// EditMessageRequest is the POST /instances/{instance}/messages/edit payload.
type EditMessageRequest struct {
	Chat      string `json:"chat"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

// EditMessageResponse is the answer to an applied edit: the upstream id of
// the edited message.
type EditMessageResponse struct {
	MessageID string `json:"message_id" binding:"required"`
}

// HandleEditMessage applies a text edit to a message sent by the instance and
// answers 200 with the upstream id. It loads the target first (404) and
// authorizes (403) before reading the body or touching the session; text
// outside 1..4096 characters or a blank chat/message id answers 422 without
// reaching the session, an offline session answers 409 and an unknown message
// 404.
//
// @Summary Edit a sent text message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body EditMessageRequest true "Edit payload"
// @Header 200 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 200 {object} core.Envelope{data=EditMessageResponse} "Edited, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or message not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid text, target, or reused key"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages/edit [post]
func HandleEditMessage(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "message-edit").Msg("edit message request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "message-edit").Err(err).Msg("edit message failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request EditMessageRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		chat := strings.TrimSpace(request.Chat)
		if chat == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}
		messageID := strings.TrimSpace(request.MessageID)
		if messageID == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "message_id is required")
			return
		}
		if utf8.RuneCountInString(strings.TrimSpace(request.Text)) == 0 || utf8.RuneCountInString(request.Text) > 4096 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "text is required, max 4096 characters")
			return
		}

		out, err := instances.EditMessage(r.Context(), id, chat, messageID, request.Text)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "message-edit").Err(err).Msg("edit message failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, EditMessageResponse{MessageID: out})
	}
}
