package httpapi

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// revokeRequest is the POST /instances/{id}/messages/revoke payload.
type revokeRequest struct {
	Chat      string `json:"chat"`
	MessageID string `json:"message_id"`
}

// revokeResponse is the 200 answer to a sent revocation. Revoked is always
// true here: the protocol revoke is fire-and-forget and the adapter reports
// no out-of-window signal, so a failure to revoke surfaces as 409 (offline)
// or 422 (invalid target) instead of revoked:false. The reason field stays
// for forward compatibility with an upstream window report.
type revokeResponse struct {
	Revoked bool   `json:"revoked"`
	Reason  string `json:"reason,omitempty"`
}

// markReadRequest is the POST /instances/{id}/chats/mark-read payload. Sender
// is the message author: required in group chats, completed with the chat
// itself in direct chats when absent.
type markReadRequest struct {
	Chat      string `json:"chat"`
	Sender    string `json:"sender,omitempty"`
	MessageID string `json:"message_id"`
}

// markReadResponse is the 200 answer to a sent read receipt.
type markReadResponse struct {
	MarkedRead bool `json:"marked_read"`
}

// handleRevokeMessage revokes a sent message for everyone and answers 200
// with revoked:true. It loads the target first (404) and authorizes (403)
// before reading the body or touching the session; an offline session answers
// 409 and an invalid target 422.
//
// @Summary Revoke a sent message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body revokeRequest true "Revoke payload"
// @Success 200 {object} envelope{data=revokeResponse} "Revoked, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid chat or message id"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/messages/revoke [post]
func handleRevokeMessage(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "revoke").Msg("revoke message request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "revoke").Err(err).Msg("revoke message failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request revokeRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		chat := strings.TrimSpace(request.Chat)
		if chat == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}
		if strings.TrimSpace(request.MessageID) == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "message_id is required")
			return
		}

		if err := instances.RevokeMessage(r.Context(), id, chat, request.MessageID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "revoke").Err(err).Msg("revoke message failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "revoke").Bool("revoked", true).Msg("revoke message result")
		JSON(w, r, http.StatusOK, revokeResponse{Revoked: true})
	}
}

// handleMarkRead sends a read receipt and answers 200. It loads the target
// first (404) and authorizes (403) before reading the body or touching the
// session; an offline session answers 409, a group read without author 422.
//
// @Summary Send a read receipt
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body markReadRequest true "Mark-read payload"
// @Success 200 {object} envelope{data=markReadResponse} "Receipt sent, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid chat, message id, or missing group sender"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/mark-read [post]
func handleMarkRead(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "mark-read").Msg("mark read request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "mark-read").Err(err).Msg("mark read failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request markReadRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		chat := strings.TrimSpace(request.Chat)
		if chat == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}
		if strings.TrimSpace(request.MessageID) == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "message_id is required")
			return
		}
		sender := strings.TrimSpace(request.Sender)
		if sender == "" {
			if isGroupJID(chat) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "sender is required in group chats")
				return
			}
			sender = chat
		}

		if err := instances.MarkRead(r.Context(), id, chat, sender, request.MessageID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "mark-read").Err(err).Msg("mark read failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "mark-read").Bool("marked_read", true).Msg("mark read result")
		JSON(w, r, http.StatusOK, markReadResponse{MarkedRead: true})
	}
}

// isGroupJID reports whether jid names a group chat: the server part is the
// group domain, the same heuristic the session adapter routes on.
func isGroupJID(jid string) bool {
	_, server, ok := strings.Cut(jid, "@")
	return ok && server == "g.us"
}
