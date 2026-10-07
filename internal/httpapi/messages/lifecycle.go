package messages

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// RevokeRequest is the POST /instances/{instance}/messages/revoke payload.
type RevokeRequest struct {
	Chat      string `json:"chat"`
	MessageID string `json:"message_id"`
}

// RevokeResponse is the 200 answer to a sent revocation. Revoked is always
// true here: the protocol revoke is fire-and-forget and the adapter reports
// no out-of-window signal, so a failure to revoke surfaces as 409 (offline)
// or 422 (invalid target) instead of revoked:false.
type RevokeResponse struct {
	Revoked bool `json:"revoked" binding:"required"`
}

// HandleRevokeMessage revokes a sent message for everyone and answers 200
// with revoked:true. It loads the target first (404) and authorizes (403)
// before reading the body or touching the session; an offline session answers
// 409 and an invalid target 422.
//
// @Summary Revoke a sent message
// @Tags messages
// @Accept json
// @Produce json
// @Security apikey
// @Param instance path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body RevokeRequest true "Revoke payload"
// @Success 200 {object} core.Envelope{data=RevokeResponse} "Revoked, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid chat or message id"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{instance}/messages/revoke [post]
func HandleRevokeMessage(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "revoke").Msg("revoke message request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "revoke").Err(err).Msg("revoke message failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request RevokeRequest
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

		if err := instances.RevokeMessage(r.Context(), id, chat, request.MessageID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "revoke").Err(err).Msg("revoke message failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "revoke").Bool("revoked", true).Msg("revoke message result")
		core.JSON(w, r, http.StatusOK, RevokeResponse{Revoked: true})
	}
}
