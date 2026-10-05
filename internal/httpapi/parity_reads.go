package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/session"
)

const (
	// defaultParityLimit is the page size the Fase-1 reads use when the
	// request omits limit, matching the other collection listings.
	defaultParityLimit = 50
	// maxParityLimit caps the page size a client can request on the Fase-1
	// reads, matching the other collection listings.
	maxParityLimit = 100
	// maxCheckContactsBatch caps the directory lookup batch: larger batches
	// answer 422 before the session is touched.
	maxCheckContactsBatch = 50
)

// joinedGroupsResponse is one page of the groups the instance belongs to.
type joinedGroupsResponse struct {
	Items      []groupResponse `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

// checkContactsRequest is the POST /instances/{id}/contacts/check payload.
type checkContactsRequest struct {
	Phones []string `json:"phones"`
}

// contactCheckItem is the lookup of one phone number: the resolved JID, the
// registration flag and the best-effort last seen.
type contactCheckItem struct {
	Phone        string     `json:"phone"`
	JID          string     `json:"jid"`
	IsOnWhatsApp bool       `json:"is_on_whatsapp"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
}

// checkContactsResponse answers a directory batch in input order.
type checkContactsResponse struct {
	Items []contactCheckItem `json:"items"`
}

// contactDevicesResponse answers the companion device JIDs of a contact.
type contactDevicesResponse struct {
	JID     string   `json:"jid"`
	Devices []string `json:"devices"`
}

// contactPhotoResponse answers the picture URL of a contact with its version
// token, empty when the contact carries no picture.
type contactPhotoResponse struct {
	JID     string `json:"jid"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

// contactBusinessResponse answers the business profile of a contact.
type contactBusinessResponse struct {
	JID          string `json:"jid"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	VerifiedName string `json:"verified_name"`
}

// blocklistResponse answers the JIDs the instance has blocked.
type blocklistResponse struct {
	Items []string `json:"items"`
}

// statusPrivacyResponse answers the own status audience.
type statusPrivacyResponse struct {
	Mode string   `json:"mode"`
	JIDs []string `json:"jids"`
}

// disappearingResponse answers the disappearing timer of a chat: the duration
// in seconds with found false when the chat carries no timer.
type disappearingResponse struct {
	Chat            string `json:"chat"`
	DurationSeconds int64  `json:"duration_seconds"`
	Found           bool   `json:"found"`
}

// newsletterMessageResponse is one channel message: the upstream server id,
// its text snapshot and the publish moment.
type newsletterMessageResponse struct {
	ServerID  string    `json:"server_id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// newsletterMessagesResponse is one page of channel messages.
type newsletterMessagesResponse struct {
	Items      []newsletterMessageResponse `json:"items"`
	NextCursor string                      `json:"next_cursor"`
}

// newsletterUpdatesResponse answers the pending message updates of a channel.
type newsletterUpdatesResponse struct {
	Items []newsletterMessageResponse `json:"items"`
}

// parityTarget loads the instance (404) and authorizes (403) for the Fase-1
// parity handlers, returning the instance id.
func parityTarget(w http.ResponseWriter, r *http.Request, instances InstanceService, log zerolog.Logger, op string) (uuid.UUID, bool) {
	id, ok := instanceID(w, r)
	if !ok {
		return uuid.Nil, false
	}
	if denyForeignInstanceKey(w, r, id) {
		return uuid.Nil, false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("parity request")

	stored, err := instances.Get(r.Context(), id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("parity request failed")
		writeInstanceError(w, r, err)
		return uuid.Nil, false
	}
	if err := authorizeInstance(r, stored); err != nil {
		writeForbidden(w, r)
		return uuid.Nil, false
	}
	return id, true
}

// parityContactJID reads the {jid} path value. A blank value answers 422: it
// is a malformed address, while an unknown contact surfaces as 404 via
// ErrContactNotFound from the service.
func parityContactJID(w http.ResponseWriter, r *http.Request) (string, bool) {
	jid := strings.TrimSpace(r.PathValue("jid"))
	if jid == "" {
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "jid is required")
		return "", false
	}
	return jid, true
}

// parityChannel reads the {channel} path value. A blank value answers 422
// like the service ErrInvalidInput for a blank channel; an unknown channel
// surfaces as 404 via ErrNewsletterNotFound.
func parityChannel(w http.ResponseWriter, r *http.Request) (string, bool) {
	channel := strings.TrimSpace(r.PathValue("channel"))
	if channel == "" {
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "channel is required")
		return "", false
	}
	return channel, true
}

// newNewsletterMessageResponse maps one session channel message to its JSON
// representation.
func newNewsletterMessageResponse(msg session.NewsletterMessage) newsletterMessageResponse {
	return newsletterMessageResponse{
		ServerID:  msg.ServerID,
		Content:   msg.Content,
		Timestamp: msg.Timestamp,
	}
}

// handleListJoinedGroups answers one page of the groups the instance belongs
// to with its next cursor. A disconnected session answers 409.
//
// @Summary List joined groups
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param limit query int false "Page size, default 50, max 100"
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} envelope{data=joinedGroupsResponse} "One page, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups [get]
func handleListJoinedGroups(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "groups-list")
		if !ok {
			return
		}

		items, next, err := instances.GetJoinedGroups(r.Context(), id,
			parseLimit(r.URL.Query().Get("limit"), defaultParityLimit, maxParityLimit),
			r.URL.Query().Get("cursor"))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "groups-list").Err(err).Msg("list joined groups failed")
			writeInstanceError(w, r, err)
			return
		}
		response := joinedGroupsResponse{Items: make([]groupResponse, 0, len(items)), NextCursor: next}
		for _, item := range items {
			response.Items = append(response.Items, newGroupResponse(item))
		}
		JSON(w, r, http.StatusOK, response)
	}
}

// handleInvitePreview resolves an invite code to the group preview without
// entering the group. A blank or unknown code answers 422: a preview is
// addressable only by its exact code, never a 404 listing probe.
//
// @Summary Preview a group invite
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param code query string true "Invite code or link"
// @Success 200 {object} envelope{data=groupResponse} "Preview, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 422 {object} errorEnvelope "Invalid or expired invite"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/invite-preview [get]
func handleInvitePreview(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "groups-invite-preview")
		if !ok {
			return
		}

		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			code = strings.TrimSpace(r.URL.Query().Get("invite_code"))
		}
		if code == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "code is required")
			return
		}

		group, err := instances.GetGroupInvitePreview(r.Context(), id, code)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "groups-invite-preview").Err(err).Msg("get invite preview failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newGroupResponse(group))
	}
}

// handleCheckContacts resolves up to 50 phones on WhatsApp, one result per
// input in order. An empty batch or more than 50 phones answers 422 before
// the session is touched; a disconnected session answers 409.
//
// @Summary Check contacts on WhatsApp
// @Tags contacts
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param request body checkContactsRequest true "Phones batch, 1..50 numbers"
// @Success 200 {object} envelope{data=checkContactsResponse} "Results in input order, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Empty batch or above the 50 cap"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/check [post]
func handleCheckContacts(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "contacts-check")
		if !ok {
			return
		}

		var request checkContactsRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if len(request.Phones) == 0 || len(request.Phones) > maxCheckContactsBatch {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "phones must hold 1..50 numbers")
			return
		}
		phones := make([]string, 0, len(request.Phones))
		for _, phone := range request.Phones {
			trimmed := strings.TrimSpace(phone)
			if trimmed == "" {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "phones must be non-empty numbers")
				return
			}
			phones = append(phones, trimmed)
		}

		results, err := instances.CheckContacts(r.Context(), id, phones)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-check").Err(err).Msg("check contacts failed")
			writeInstanceError(w, r, err)
			return
		}
		items := make([]contactCheckItem, 0, len(results))
		for _, result := range results {
			items = append(items, contactCheckItem{
				Phone:        result.Phone,
				JID:          result.JID,
				IsOnWhatsApp: result.IsOnWhatsApp,
				LastSeen:     result.LastSeen,
			})
		}
		JSON(w, r, http.StatusOK, checkContactsResponse{Items: items})
	}
}

// handleContactDevices lists the companion device JIDs of a contact. A blank
// JID answers 422 and an unknown contact 404 via ErrContactNotFound.
//
// @Summary List contact devices
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param jid path string true "Contact JID"
// @Success 200 {object} envelope{data=contactDevicesResponse} "Devices, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or contact not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 422 {object} errorEnvelope "Malformed JID"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/devices [get]
func handleContactDevices(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "contacts-devices")
		if !ok {
			return
		}
		jid, ok := parityContactJID(w, r)
		if !ok {
			return
		}

		devices, err := instances.GetContactDevices(r.Context(), id, jid)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-devices").Err(err).Msg("get contact devices failed")
			writeInstanceError(w, r, err)
			return
		}
		if devices == nil {
			devices = []string{}
		}
		JSON(w, r, http.StatusOK, contactDevicesResponse{JID: jid, Devices: devices})
	}
}

// handleContactPhoto answers the picture URL of a contact with its version
// token, empty when the contact carries no picture. A blank JID answers 422
// and an unknown contact 404 via ErrContactNotFound.
//
// @Summary Get a contact photo
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param jid path string true "Contact JID"
// @Success 200 {object} envelope{data=contactPhotoResponse} "Photo, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or contact not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 422 {object} errorEnvelope "Malformed JID"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/photo [get]
func handleContactPhoto(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "contacts-photo")
		if !ok {
			return
		}
		jid, ok := parityContactJID(w, r)
		if !ok {
			return
		}

		info, err := instances.GetContactPhoto(r.Context(), id, jid)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-photo").Err(err).Msg("get contact photo failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, contactPhotoResponse{JID: jid, URL: info.URL, Version: info.Version})
	}
}

// handleContactBusiness answers the business profile of a contact. A blank
// JID answers 422 and a contact without business profile 404 via
// ErrContactNotFound.
//
// @Summary Get a contact business profile
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param jid path string true "Contact JID"
// @Success 200 {object} envelope{data=contactBusinessResponse} "Business profile, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or contact not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 422 {object} errorEnvelope "Malformed JID"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/business [get]
func handleContactBusiness(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "contacts-business")
		if !ok {
			return
		}
		jid, ok := parityContactJID(w, r)
		if !ok {
			return
		}

		profile, err := instances.GetContactBusiness(r.Context(), id, jid)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-business").Err(err).Msg("get contact business failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, contactBusinessResponse{
			JID:          jid,
			Name:         profile.Name,
			Description:  profile.Description,
			VerifiedName: profile.VerifiedName,
		})
	}
}

// handleGetBlocklist answers the JIDs the instance has blocked. A
// disconnected session answers 409.
//
// @Summary Get the blocklist
// @Tags blocklist
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Success 200 {object} envelope{data=blocklistResponse} "Blocked JIDs, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/blocklist [get]
func handleGetBlocklist(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "blocklist-get")
		if !ok {
			return
		}

		items, err := instances.GetBlocklist(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "blocklist-get").Err(err).Msg("get blocklist failed")
			writeInstanceError(w, r, err)
			return
		}
		if items == nil {
			items = []string{}
		}
		JSON(w, r, http.StatusOK, blocklistResponse{Items: items})
	}
}

// handleGetStatusPrivacy answers the own status audience. A disconnected
// session answers 409.
//
// @Summary Get the status privacy
// @Tags status
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Success 200 {object} envelope{data=statusPrivacyResponse} "Audience, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/status/privacy [get]
func handleGetStatusPrivacy(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "status-privacy-get")
		if !ok {
			return
		}

		privacy, err := instances.GetStatusPrivacy(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "status-privacy-get").Err(err).Msg("get status privacy failed")
			writeInstanceError(w, r, err)
			return
		}
		jids := privacy.JIDs
		if jids == nil {
			jids = []string{}
		}
		JSON(w, r, http.StatusOK, statusPrivacyResponse{Mode: privacy.Mode, JIDs: jids})
	}
}

// handleGetDisappearing answers the disappearing timer of a chat, with found
// false when the chat carries no timer. A blank chat answers 422 and a
// disconnected session 409.
//
// @Summary Get the disappearing timer of a chat
// @Tags chats
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param chat path string true "Chat JID"
// @Success 200 {object} envelope{data=disappearingResponse} "Timer, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 422 {object} errorEnvelope "Invalid chat"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/{chat}/disappearing [get]
func handleGetDisappearing(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "chats-disappearing-get")
		if !ok {
			return
		}
		chat := strings.TrimSpace(r.PathValue("chat"))
		if chat == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}

		duration, found, err := instances.GetDisappearingTimer(r.Context(), id, chat)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "chats-disappearing-get").Err(err).Msg("get disappearing timer failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, disappearingResponse{
			Chat:            chat,
			DurationSeconds: int64(duration / time.Second),
			Found:           found,
		})
	}
}

// handleGetNewsletterMessages pages the messages of a channel with the
// upstream cursor. A blank channel answers 422 and an unknown channel 404 via
// ErrNewsletterNotFound.
//
// @Summary List newsletter messages
// @Tags newsletters
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param channel path string true "Channel JID"
// @Param limit query int false "Page size, default 50, max 100"
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} envelope{data=newsletterMessagesResponse} "One page, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 422 {object} errorEnvelope "Invalid channel"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/messages [get]
func handleGetNewsletterMessages(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "newsletter-messages")
		if !ok {
			return
		}
		channel, ok := parityChannel(w, r)
		if !ok {
			return
		}

		items, next, err := instances.GetNewsletterMessages(r.Context(), id, channel,
			r.URL.Query().Get("cursor"),
			parseLimit(r.URL.Query().Get("limit"), defaultParityLimit, maxParityLimit))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-messages").Err(err).Msg("get newsletter messages failed")
			writeInstanceError(w, r, err)
			return
		}
		response := newsletterMessagesResponse{Items: make([]newsletterMessageResponse, 0, len(items)), NextCursor: next}
		for _, item := range items {
			response.Items = append(response.Items, newNewsletterMessageResponse(item))
		}
		JSON(w, r, http.StatusOK, response)
	}
}

// handleGetNewsletterUpdates answers the pending message updates of a channel.
// A blank channel answers 422 and an unknown channel 404 via
// ErrNewsletterNotFound.
//
// @Summary List newsletter updates
// @Tags newsletters
// @Produce json
// @Security apikey
// @Param id path string true "Instance ID (UUID)"
// @Param channel path string true "Channel JID"
// @Success 200 {object} envelope{data=newsletterUpdatesResponse} "Updates, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} errorEnvelope "Instance not connected"
// @Failure 422 {object} errorEnvelope "Invalid channel"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/updates [get]
func handleGetNewsletterUpdates(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "newsletter-updates")
		if !ok {
			return
		}
		channel, ok := parityChannel(w, r)
		if !ok {
			return
		}

		items, err := instances.GetNewsletterUpdates(r.Context(), id, channel)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-updates").Err(err).Msg("get newsletter updates failed")
			writeInstanceError(w, r, err)
			return
		}
		response := newsletterUpdatesResponse{Items: make([]newsletterMessageResponse, 0, len(items))}
		for _, item := range items {
			response.Items = append(response.Items, newNewsletterMessageResponse(item))
		}
		JSON(w, r, http.StatusOK, response)
	}
}
