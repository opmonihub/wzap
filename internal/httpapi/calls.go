package httpapi

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// rejectCallRequest is the POST /instances/{id}/calls/reject payload: the
// active call and its caller. The service never initiates calls; rejection
// is the only call control.
type rejectCallRequest struct {
	CallID string `json:"call_id"`
	From   string `json:"from"`
}

// rejectCallResponse is the 200 answer to a rejected call.
type rejectCallResponse struct {
	Rejected bool `json:"rejected"`
}

// handleRejectCall rejects the active call and answers 200. It loads the
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
// @Param apikey header string true "Global, owning user, or own instance key"
// @Param X-Request-Id header string false "Correlation id, echoed back"
// @Param id path string true "Instance ID (UUID)"
// @Param request body rejectCallRequest true "Reject payload"
// @Success 200 {object} rejectCallResponse "Rejected, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Missing call id or caller"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 501 {object} errorEnvelope "Upstream cannot reject the call"
// @Router /instances/{id}/calls/reject [post]
func handleRejectCall(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "call-reject").Msg("reject call request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "call-reject").Err(err).Msg("reject call failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request rejectCallRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		callID := strings.TrimSpace(request.CallID)
		if callID == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "call_id is required")
			return
		}
		from := strings.TrimSpace(request.From)
		if from == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "from is required")
			return
		}

		if err := instances.RejectCall(r.Context(), id, from, callID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "call-reject").Err(err).Msg("reject call failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "call-reject").Bool("rejected", true).Msg("reject call result")
		JSON(w, r, http.StatusOK, rejectCallResponse{Rejected: true})
	}
}
