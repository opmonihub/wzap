package instances

import (
	"net/http"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
	"wzap/internal/instance"
)

// ConnectResponse wraps the pairing subset under data.connection (matrix
// §2): only the status plus the QR fields while pairing is in progress.
type ConnectResponse struct {
	Connection representation.PairingConnection `json:"connection" binding:"required"`
}

// StatusResponse wraps the connection block for the status route.
type StatusResponse struct {
	Connection representation.ConnectionResponse `json:"connection" binding:"required"`
}

// HandleConnectInstance starts pairing and answers 200 with the QR code and its
// validity, or with the status and no QR when the instance is already
// connected. It loads the target first (404) and authorizes (403) before
// touching the session.
//
// @Summary Connect an instance
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=ConnectResponse} "Pairing result, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance already connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/connect [post]
func HandleConnectInstance(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "connect").Msg("connect instance request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "connect").Err(err).Msg("connect instance failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		result, err := instances.Connect(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "connect").Err(err).Msg("connect instance failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		ev := log.Debug().Str("instance_id", id.String()).Str("op", "connect").
			Str("status", string(result.Status)).Bool("qr_present", result.QRCode != "")
		if result.QRExpiresAt != nil {
			ev = ev.Time("expires_at", *result.QRExpiresAt)
		}
		ev.Msg("connect instance result")
		core.JSON(w, r, http.StatusOK, NewConnectResponse(result))
	}
}

// HandleQRInstance answers 200 with the current pairing QR of an instance, or
// 409 when its session is already connected. It loads the target first (404)
// and authorizes (403) before touching the session.
//
// @Summary Get the pairing QR
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=ConnectResponse} "Current QR, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance already connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/qr [get]
func HandleQRInstance(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "qr").Msg("qr instance request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "qr").Err(err).Msg("qr instance failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		result, err := instances.QR(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "qr").Err(err).Msg("qr instance failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		ev := log.Debug().Str("instance_id", id.String()).Str("op", "qr").
			Str("status", string(result.Status)).Bool("qr_present", result.QRCode != "")
		if result.QRExpiresAt != nil {
			ev = ev.Time("expires_at", *result.QRExpiresAt)
		}
		ev.Msg("qr instance result")
		core.JSON(w, r, http.StatusOK, NewConnectResponse(result))
	}
}

// HandleInstanceStatus answers 200 with the connection status of an instance.
//
// @Summary Get connection status
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=StatusResponse} "Connection status, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status [get]
func HandleInstanceStatus(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status").Msg("instance status request")

		found, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status").Err(err).Msg("instance status failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, found); err != nil {
			core.WriteForbidden(w, r)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "status").Str("status", found.Connection.Status).Msg("instance status result")
		core.JSON(w, r, http.StatusOK, StatusResponse{
			Connection: representation.NewConnectionResponse(found.Connection),
		})
	}
}

// HandleDisconnectInstance ends the session of an instance, clearing its
// paired identity and connection state, and answers 204. It loads the target
// first (404) and authorizes (403) before touching the session.
//
// @Summary Disconnect an instance
// @Tags connection
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 204 "Disconnected, no body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Failure 409 {object} core.ErrorEnvelope "instance_name_taken: name already in use"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/disconnect [post]
func HandleDisconnectInstance(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "disconnect").Msg("disconnect instance request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "disconnect").Err(err).Msg("disconnect instance failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		if err := instances.Disconnect(r.Context(), id); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "disconnect").Err(err).Msg("disconnect instance failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "disconnect").Str("status", "disconnected").Msg("disconnect instance result")
		core.JSON(w, r, http.StatusNoContent, nil)
	}
}

// NewConnectResponse maps a pairing result to its JSON representation: the
// connection block with the QR fields while pairing.
func NewConnectResponse(result instance.ConnectResult) ConnectResponse {
	response := ConnectResponse{Connection: representation.PairingConnection{Status: string(result.Status)}}
	if result.Status == "pairing" {
		response.Connection.QRCode = result.QRCode
		response.Connection.QRExpiresAt = result.QRExpiresAt
	}
	return response
}
