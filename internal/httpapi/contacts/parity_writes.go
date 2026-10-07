package contacts

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
)

// UpdateBlocklistRequest is the POST /instances/{id}/blocklist payload.
type UpdateBlocklistRequest struct {
	Action string `json:"action"`
	JID    string `json:"jid"`
}

// BlocklistUpdateResponse is the answer to a block/unblock.
type BlocklistUpdateResponse struct {
	Updated bool `json:"updated" binding:"required"`
}

// SubscribePresenceResponse answers a single presence subscription signal.
type SubscribePresenceResponse struct {
	Subscribed bool `json:"subscribed" binding:"required"`
}

// ContactLinkResponse answers the own contact QR link.
type ContactLinkResponse struct {
	Link string `json:"link" binding:"required"`
}

// HandleUpdateBlocklist blocks or unblocks a JID and answers 200. A bad action
// or a blank JID answers 422 before the session is touched.
//
// @Summary Block or unblock a JID
// @Tags blocklist
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body UpdateBlocklistRequest true "Blocklist payload"
// @Success 200 {object} core.Envelope{data=BlocklistUpdateResponse} "Applied, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Unknown action or invalid JID"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/blocklist [post]
func HandleUpdateBlocklist(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "blocklist-update")
		if !ok {
			return
		}

		var request UpdateBlocklistRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		action := strings.TrimSpace(request.Action)
		if action != "block" && action != "unblock" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "action must be block or unblock")
			return
		}
		jid := strings.TrimSpace(request.JID)
		if jid == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "jid is required")
			return
		}

		if err := instances.UpdateBlocklist(r.Context(), id, jid, action); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "blocklist-update").Err(err).Msg("update blocklist failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, BlocklistUpdateResponse{Updated: true})
	}
}

// HandleSubscribePresence sends a single presence subscription signal for a
// contact and answers 200. There is no heartbeat: the subscription carries the
// one signal only. A blank JID answers 422 and an unknown contact 404 through
// the existing error mapping.
//
// @Summary Subscribe to a contact presence (single signal, no heartbeat)
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param jid path string true "Contact JID"
// @Success 200 {object} core.Envelope{data=SubscribePresenceResponse} "Subscribed, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or contact not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Malformed JID"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/subscribe [post]
func HandleSubscribePresence(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "contacts-subscribe")
		if !ok {
			return
		}
		jid, ok := core.ParityContactJID(w, r)
		if !ok {
			return
		}

		if err := instances.SubscribePresence(r.Context(), id, jid); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-subscribe").Err(err).Msg("subscribe presence failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, SubscribePresenceResponse{Subscribed: true})
	}
}

// HandleContactLink answers the own contact QR link. revoke=true invalidates
// the previous link; the adapter owns the rotation.
//
// @Summary Get the own contact link
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param revoke query bool false "Revoke the previous link"
// @Success 200 {object} core.Envelope{data=ContactLinkResponse} "Link, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contact-link [get]
func HandleContactLink(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "contacts-link")
		if !ok {
			return
		}

		revoke := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("revoke")), "true")
		link, err := instances.GetContactQRLink(r.Context(), id, revoke)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-link").Err(err).Msg("get contact link failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, ContactLinkResponse{Link: link})
	}
}
