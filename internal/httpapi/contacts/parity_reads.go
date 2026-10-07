package contacts

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
)

// CheckContactsRequest is the POST /instances/{id}/contacts/check payload.
type CheckContactsRequest struct {
	Phones []string `json:"phones"`
}

// ContactCheckItem is the lookup of one phone number: the resolved JID, the
// registration flag and the best-effort last seen.
type ContactCheckItem struct {
	Phone        string     `json:"phone"`
	JID          string     `json:"jid"`
	IsOnWhatsApp bool       `json:"is_on_whatsapp"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
}

// CheckContactsResponse answers a directory batch in input order.
type CheckContactsResponse struct {
	Contacts []ContactCheckItem `json:"contacts" binding:"required"`
}

// ContactDevicesResponse answers the companion device JIDs of a contact.
type ContactDevicesResponse struct {
	JID     string   `json:"jid" binding:"required"`
	Devices []string `json:"devices" binding:"required"`
}

// ContactPhotoResponse answers the picture URL of a contact with its version
// token, empty when the contact carries no picture.
type ContactPhotoResponse struct {
	JID     string `json:"jid" binding:"required"`
	URL     string `json:"url" binding:"required"`
	Version string `json:"version" binding:"required"`
}

// ContactBusinessResponse answers the business profile of a contact.
type ContactBusinessResponse struct {
	JID          string `json:"jid" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description" binding:"required"`
	VerifiedName string `json:"verified_name" binding:"required"`
}

// BlocklistResponse answers the JIDs the instance has blocked.
type BlocklistResponse struct {
	BlockedJIDs []string `json:"blocked_jids" binding:"required"`
}

// HandleCheckContacts resolves up to 50 phones on WhatsApp, one result per
// input in order. An empty batch or more than 50 phones answers 422 before
// the session is touched; a disconnected session answers 409.
//
// @Summary Check contacts on WhatsApp
// @Tags contacts
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body CheckContactsRequest true "Phones batch, 1..50 numbers"
// @Success 200 {object} core.Envelope{data=CheckContactsResponse} "Results in input order, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Empty batch or above the 50 cap"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/check [post]
func HandleCheckContacts(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "contacts-check")
		if !ok {
			return
		}

		var request CheckContactsRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if len(request.Phones) == 0 || len(request.Phones) > core.MaxCheckContactsBatch {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "phones must hold 1..50 numbers")
			return
		}
		phones := make([]string, 0, len(request.Phones))
		for _, phone := range request.Phones {
			trimmed := strings.TrimSpace(phone)
			if trimmed == "" {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "phones must be non-empty numbers")
				return
			}
			phones = append(phones, trimmed)
		}

		results, err := instances.CheckContacts(r.Context(), id, phones)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-check").Err(err).Msg("check contacts failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		items := make([]ContactCheckItem, 0, len(results))
		for _, result := range results {
			items = append(items, ContactCheckItem{
				Phone:        result.Phone,
				JID:          result.JID,
				IsOnWhatsApp: result.IsOnWhatsApp,
				LastSeen:     result.LastSeen,
			})
		}
		core.JSON(w, r, http.StatusOK, CheckContactsResponse{Contacts: items})
	}
}

// HandleContactDevices lists the companion device JIDs of a contact. A blank
// JID answers 422 and an unknown contact 404 via ErrContactNotFound.
//
// @Summary List contact devices
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param jid path string true "Contact JID"
// @Success 200 {object} core.Envelope{data=ContactDevicesResponse} "Devices, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or contact not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Malformed JID"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/devices [get]
func HandleContactDevices(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "contacts-devices")
		if !ok {
			return
		}
		jid, ok := core.ParityContactJID(w, r)
		if !ok {
			return
		}

		devices, err := instances.GetContactDevices(r.Context(), id, jid)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-devices").Err(err).Msg("get contact devices failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if devices == nil {
			devices = []string{}
		}
		core.JSON(w, r, http.StatusOK, ContactDevicesResponse{JID: jid, Devices: devices})
	}
}

// HandleContactPhoto answers the picture URL of a contact with its version
// token, empty when the contact carries no picture. A blank JID answers 422
// and an unknown contact 404 via ErrContactNotFound.
//
// @Summary Get a contact photo
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param jid path string true "Contact JID"
// @Success 200 {object} core.Envelope{data=ContactPhotoResponse} "Photo, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or contact not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Malformed JID"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/photo [get]
func HandleContactPhoto(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "contacts-photo")
		if !ok {
			return
		}
		jid, ok := core.ParityContactJID(w, r)
		if !ok {
			return
		}

		info, err := instances.GetContactPhoto(r.Context(), id, jid)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-photo").Err(err).Msg("get contact photo failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, ContactPhotoResponse{JID: jid, URL: info.URL, Version: info.Version})
	}
}

// HandleContactBusiness answers the business profile of a contact. A blank
// JID answers 422 and a contact without business profile 404 via
// ErrContactNotFound.
//
// @Summary Get a contact business profile
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param jid path string true "Contact JID"
// @Success 200 {object} core.Envelope{data=ContactBusinessResponse} "Business profile, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or contact not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Malformed JID"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/business [get]
func HandleContactBusiness(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "contacts-business")
		if !ok {
			return
		}
		jid, ok := core.ParityContactJID(w, r)
		if !ok {
			return
		}

		profile, err := instances.GetContactBusiness(r.Context(), id, jid)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-business").Err(err).Msg("get contact business failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, ContactBusinessResponse{
			JID:          jid,
			Name:         profile.Name,
			Description:  profile.Description,
			VerifiedName: profile.VerifiedName,
		})
	}
}

// HandleGetBlocklist answers the JIDs the instance has blocked. A
// disconnected session answers 409.
//
// @Summary Get the blocklist
// @Tags blocklist
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Success 200 {object} core.Envelope{data=BlocklistResponse} "Blocked JIDs, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/blocklist [get]
func HandleGetBlocklist(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "blocklist-get")
		if !ok {
			return
		}

		items, err := instances.GetBlocklist(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "blocklist-get").Err(err).Msg("get blocklist failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if items == nil {
			items = []string{}
		}
		core.JSON(w, r, http.StatusOK, BlocklistResponse{BlockedJIDs: representation.NonNilStrings(items)})
	}
}
