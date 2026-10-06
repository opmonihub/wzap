package httpapi

import (
	"net/http"

	"github.com/rs/zerolog"

	"wzap/internal/instance"
)

// connectResponse wraps the pairing subset under data.connection (matrix
// §2): only the status plus the QR fields while pairing is in progress.
type connectResponse struct {
	Connection pairingConnection `json:"connection"`
}

// statusResponse wraps the connection block for the status route.
type statusResponse struct {
	Connection connectionResponse `json:"connection"`
}

// handleConnectInstance starts pairing and answers 200 with the QR code and its
// validity, or with the status and no QR when the instance is already
// connected. It loads the target first (404) and authorizes (403) before
// touching the session.
//
// @Summary Connect an instance
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} envelope{data=connectResponse} "Pairing result, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance already connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/connect [post]
func handleConnectInstance(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "connect").Msg("connect instance request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "connect").Err(err).Msg("connect instance failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		result, err := instances.Connect(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "connect").Err(err).Msg("connect instance failed")
			writeInstanceError(w, r, err)
			return
		}
		ev := log.Debug().Str("instance_id", id.String()).Str("op", "connect").
			Str("status", string(result.Status)).Bool("qr_present", result.QRCode != "")
		if result.QRExpiresAt != nil {
			ev = ev.Time("expires_at", *result.QRExpiresAt)
		}
		ev.Msg("connect instance result")
		JSON(w, r, http.StatusOK, newConnectResponse(result))
	}
}

// handleQRInstance answers 200 with the current pairing QR of an instance, or
// 409 when its session is already connected. It loads the target first (404)
// and authorizes (403) before touching the session.
//
// @Summary Get the pairing QR
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} envelope{data=connectResponse} "Current QR, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance already connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/qr [get]
func handleQRInstance(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "qr").Msg("qr instance request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "qr").Err(err).Msg("qr instance failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		result, err := instances.QR(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "qr").Err(err).Msg("qr instance failed")
			writeInstanceError(w, r, err)
			return
		}
		ev := log.Debug().Str("instance_id", id.String()).Str("op", "qr").
			Str("status", string(result.Status)).Bool("qr_present", result.QRCode != "")
		if result.QRExpiresAt != nil {
			ev = ev.Time("expires_at", *result.QRExpiresAt)
		}
		ev.Msg("qr instance result")
		JSON(w, r, http.StatusOK, newConnectResponse(result))
	}
}

// handleInstanceStatus answers 200 with the connection status of an instance.
//
// @Summary Get connection status
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} envelope{data=statusResponse} "Connection status, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 409 {object} errorEnvelope "instance_name_ambiguous: legacy name matches multiple instances"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status [get]
func handleInstanceStatus(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status").Msg("instance status request")

		found, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status").Err(err).Msg("instance status failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, found); err != nil {
			writeForbidden(w, r)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status").Str("status", found.Connection.Status).Msg("instance status result")
		JSON(w, r, http.StatusOK, statusResponse{
			Connection: newConnectionResponse(found.Connection),
		})
	}
}

// handleDisconnectInstance ends the session of an instance, clearing its
// paired identity and connection state, and answers 204. It loads the target
// first (404) and authorizes (403) before touching the session.
//
// @Summary Disconnect an instance
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 204 "Disconnected, no body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Failure 409 {object} errorEnvelope "instance_name_ambiguous: legacy name matches multiple instances"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/disconnect [post]
func handleDisconnectInstance(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "disconnect").Msg("disconnect instance request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "disconnect").Err(err).Msg("disconnect instance failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		if err := instances.Disconnect(r.Context(), id); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "disconnect").Err(err).Msg("disconnect instance failed")
			writeInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "disconnect").Str("status", "disconnected").Msg("disconnect instance result")
		JSON(w, r, http.StatusNoContent, nil)
	}
}

// newConnectResponse maps a pairing result to its JSON representation: the
// connection block with the QR fields while pairing.
func newConnectResponse(result instance.ConnectResult) connectResponse {
	return connectResponse{
		Connection: pairingConnection{
			Status:      string(result.Status),
			QRCode:      result.QRCode,
			QRExpiresAt: result.QRExpiresAt,
		},
	}
}
