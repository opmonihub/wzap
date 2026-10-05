package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// pairPhoneRequest is the POST /instances/{id}/pair-phone payload.
type pairPhoneRequest struct {
	Phone string `json:"phone"`
}

// pairPhoneResponse is the 200 answer with the 8-digit code. ExpiresAt is the
// expiry of the pairing channel the code was issued on, the same instant the
// QR of the channel expires.
type pairPhoneResponse struct {
	PairingCode string    `json:"pairing_code"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// handlePairPhone issues the 8-digit pairing code of the open pairing channel
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
// @Param apikey header string false "Global key or own instance key; alternatively use the owning user/admin session cookie"
// @Param X-Request-Id header string false "Correlation id, echoed back"
// @Param id path string true "Instance ID (UUID)"
// @Param request body pairPhoneRequest true "Phone payload"
// @Success 200 {object} envelope{data=pairPhoneResponse} "Pairing code, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "No open pairing channel, or already connected"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid phone number"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/pair-phone [post]
func handlePairPhone(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		// The number never reaches the logs: only the operation is recorded.
		log.Debug().Str("instance_id", id.String()).Str("op", "pair-phone").Msg("pair phone request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "pair-phone").Err(err).Msg("pair phone failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request pairPhoneRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		phone := strings.TrimSpace(request.Phone)
		if phone == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invalid phone number")
			return
		}

		result, err := instances.PairPhone(r.Context(), id, phone)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "pair-phone").Err(err).Msg("pair phone failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "pair-phone").
			Time("expires_at", result.ExpiresAt).Msg("pair phone result")
		JSON(w, r, http.StatusOK, pairPhoneResponse{PairingCode: result.Code, ExpiresAt: result.ExpiresAt})
	}
}
