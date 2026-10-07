package instances

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// RejectCallRequest is the POST /instances/{id}/calls/reject payload: the
// active call and its caller. The service never initiates calls; rejection
// is the only call control.
type RejectCallRequest struct {
	CallID string `json:"call_id"`
	From   string `json:"from"`
}

// RejectCallResponse is the 200 answer to a rejected call.
type RejectCallResponse struct {
	Rejected bool `json:"rejected" binding:"required"`
}

// HandleRejectCall rejects the active call and answers 200. It loads the
// target first (404) and authorizes (403) before reading the body or touching
// the session; an offline session answers 409, a missing call id or caller
// 422, and an upstream that cannot reject answers 501 with the stable
// not_supported code (the companion never initiates calls, so there is no
// dial route).
//
// @Summary Reject the active call
// @Tags calls
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body RejectCallRequest true "Reject payload"
// @Success 200 {object} core.Envelope{data=RejectCallResponse} "Rejected, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Missing call id or caller"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 501 {object} core.ErrorEnvelope "Upstream cannot reject the call"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/calls/reject [post]
func HandleRejectCall(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "call-reject").Msg("reject call request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "call-reject").Err(err).Msg("reject call failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request RejectCallRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		callID := strings.TrimSpace(request.CallID)
		if callID == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "call_id is required")
			return
		}
		from := strings.TrimSpace(request.From)
		if from == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "from is required")
			return
		}

		if err := instances.RejectCall(r.Context(), id, from, callID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "call-reject").Err(err).Msg("reject call failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "call-reject").Bool("rejected", true).Msg("reject call result")
		core.JSON(w, r, http.StatusOK, RejectCallResponse{Rejected: true})
	}
}
