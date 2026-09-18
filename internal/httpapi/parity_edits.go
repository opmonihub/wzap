package httpapi

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog"
)

// editMessageRequest is the POST /instances/{id}/messages/edit payload.
type editMessageRequest struct {
	Chat      string `json:"chat"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

// editMessageResponse is the answer to an applied edit: the upstream id of
// the edited message.
type editMessageResponse struct {
	MessageID string `json:"message_id"`
}

// handleEditMessage applies a text edit to a message sent by the instance and
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
// @Param apikey header string true "Global, owning user, or own instance key"
// @Param X-Request-Id header string false "Correlation id, echoed back"
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param id path string true "Instance ID (UUID)"
// @Param request body editMessageRequest true "Edit payload"
// @Header 200 {string} X-Idempotent-Replay "true when replayed from a previous call"
// @Success 200 {object} editMessageResponse "Edited, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or message not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid text, target, or reused key"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Router /instances/{id}/messages/edit [post]
func handleEditMessage(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "message-edit").Msg("edit message request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "message-edit").Err(err).Msg("edit message failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request editMessageRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		chat := strings.TrimSpace(request.Chat)
		if chat == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}
		messageID := strings.TrimSpace(request.MessageID)
		if messageID == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "message_id is required")
			return
		}
		if utf8.RuneCountInString(strings.TrimSpace(request.Text)) == 0 || utf8.RuneCountInString(request.Text) > 4096 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "text is required, max 4096 characters")
			return
		}

		out, err := instances.EditMessage(r.Context(), id, chat, messageID, request.Text)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "message-edit").Err(err).Msg("edit message failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, editMessageResponse{MessageID: out})
	}
}
