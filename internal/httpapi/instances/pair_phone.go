package instances

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// PairPhoneRequest is the POST /instances/{id}/pair-phone payload.
type PairPhoneRequest struct {
	Phone string `json:"phone"`
}

// PairPhoneResponse is the 200 answer with the 8-digit code. ExpiresAt is the
// expiry of the pairing channel the code was issued on, the same instant the
// QR of the channel expires.
type PairPhoneResponse struct {
	PairingCode string    `json:"pairing_code" binding:"required"`
	ExpiresAt   time.Time `json:"expires_at" binding:"required"`
}

// HandlePairPhone issues the 8-digit pairing code of the open pairing channel
// and answers 200. It loads the target first (404) and authorizes (403)
// before reading the body or touching the session; without a prior Connect
// the answer is 409 and an invalid number a generic 422 that reveals nothing
// about the number. The channel is never opened implicitly. The number is
// never logged.
//
// @Summary Pair by phone code
// @Tags connection
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body PairPhoneRequest true "Phone payload"
// @Success 200 {object} core.Envelope{data=PairPhoneResponse} "Pairing code, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "No open pairing channel, or already connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid phone number"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/pair-phone [post]
func HandlePairPhone(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}

		log.Debug().Str("instance_id", id.String()).Str("op", "pair-phone").Msg("pair phone request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "pair-phone").Err(err).Msg("pair phone failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request PairPhoneRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		phone := strings.TrimSpace(request.Phone)
		if phone == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid phone number")
			return
		}

		result, err := instances.PairPhone(r.Context(), id, phone)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "pair-phone").Err(err).Msg("pair phone failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "pair-phone").
			Time("expires_at", result.ExpiresAt).Msg("pair phone result")
		core.JSON(w, r, http.StatusOK, PairPhoneResponse{PairingCode: result.Code, ExpiresAt: result.ExpiresAt})
	}
}
