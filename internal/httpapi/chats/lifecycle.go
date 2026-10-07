package chats

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// MarkReadRequest is the POST /instances/{id}/chats/mark-read payload. Sender
// is the message author: required in group chats, completed with the chat
// itself in direct chats when absent.
type MarkReadRequest struct {
	Chat      string `json:"chat"`
	Sender    string `json:"sender,omitempty"`
	MessageID string `json:"message_id"`
}

// MarkReadResponse is the 200 answer to a sent read receipt.
type MarkReadResponse struct {
	MarkedRead bool `json:"marked_read" binding:"required"`
}

// HandleMarkRead sends a read receipt and answers 200. It loads the target
// first (404) and authorizes (403) before reading the body or touching the
// session; an offline session answers 409, a group read without author 422.
//
// @Summary Send a read receipt
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body MarkReadRequest true "Mark-read payload"
// @Success 200 {object} core.Envelope{data=MarkReadResponse} "Receipt sent, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid chat, message id, or missing group sender"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/mark-read [post]
func HandleMarkRead(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "mark-read").Msg("mark read request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "mark-read").Err(err).Msg("mark read failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request MarkReadRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		chat := strings.TrimSpace(request.Chat)
		if chat == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}
		if strings.TrimSpace(request.MessageID) == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "message_id is required")
			return
		}
		sender := strings.TrimSpace(request.Sender)
		if sender == "" {
			if IsGroupJID(chat) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "sender is required in group chats")
				return
			}
			sender = chat
		}

		if err := instances.MarkRead(r.Context(), id, chat, sender, request.MessageID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "mark-read").Err(err).Msg("mark read failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "mark-read").Bool("marked_read", true).Msg("mark read result")
		core.JSON(w, r, http.StatusOK, MarkReadResponse{MarkedRead: true})
	}
}

// IsGroupJID reports whether jid names a group chat: the server part is the
// group domain, the same heuristic the session adapter routes on.
func IsGroupJID(jid string) bool {
	_, server, ok := strings.Cut(jid, "@")
	return ok && server == "g.us"
}
