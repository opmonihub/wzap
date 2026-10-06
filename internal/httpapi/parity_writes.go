package httpapi

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"
)

// updateBlocklistRequest is the POST /instances/{id}/blocklist payload.
type updateBlocklistRequest struct {
	Action string `json:"action"`
	JID    string `json:"jid"`
}

// blocklistUpdateResponse is the answer to a block/unblock.
type blocklistUpdateResponse struct {
	Updated bool `json:"updated"`
}

// disappearingRequest is the PUT disappearing timer payload: a string duration
// such as "24h", with "0" or "0s" switching the timer off.
type disappearingRequest struct {
	Duration string `json:"duration"`
}

// disappearingUpdatedResponse acknowledges a timer command. Reading the
// applied duration remains the responsibility of the disappearing GET route.
type disappearingUpdatedResponse struct {
	Updated bool `json:"updated"`
}

// subscribePresenceResponse answers a single presence subscription signal.
type subscribePresenceResponse struct {
	Subscribed bool `json:"subscribed"`
}

// contactLinkResponse answers the own contact QR link.
type contactLinkResponse struct {
	Link string `json:"link"`
}

// createNewsletterRequest is the POST /instances/{id}/newsletters payload.
type createNewsletterRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// muteNewsletterRequest is the POST
// /instances/{id}/newsletters/{channel}/mute payload.
type muteNewsletterRequest struct {
	Muted bool `json:"muted"`
}

// muteNewsletterResponse is the answer to a mute/unmute.
type muteNewsletterResponse struct {
	Muted bool `json:"muted"`
}

// markNewsletterViewedRequest is the POST
// /instances/{id}/newsletters/{channel}/viewed payload.
type markNewsletterViewedRequest struct {
	ServerIDs []string `json:"server_ids"`
}

// newsletterViewedResponse is the answer to a mark-viewed batch.
type newsletterViewedResponse struct {
	Viewed bool `json:"viewed"`
}

// reactNewsletterRequest is the POST
// /instances/{id}/newsletters/{channel}/reactions payload: an empty reaction
// removes the reaction.
type reactNewsletterRequest struct {
	ServerID string `json:"server_id"`
	Reaction string `json:"reaction"`
}

// newsletterReactResponse is the answer to a reaction.
type newsletterReactResponse struct {
	Reacted bool `json:"reacted"`
}

// parseDisappearingDuration parses the raw duration and validates it against
// the upstream timer allowlist of off/24h/168h/2160h. Anything else answers
// 422 without touching the session.
func parseDisappearingDuration(raw string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, false
	}
	switch duration {
	case 0, 24 * time.Hour, 168 * time.Hour, 2160 * time.Hour:
		return duration, true
	}
	return 0, false
}

// handleUpdateBlocklist blocks or unblocks a JID and answers 200. A bad action
// or a blank JID answers 422 before the session is touched.
//
// @Summary Block or unblock a JID
// @Tags blocklist
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body updateBlocklistRequest true "Blocklist payload"
// @Success 200 {object} envelope{data=blocklistUpdateResponse} "Applied, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Unknown action or invalid JID"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/blocklist [post]
func handleUpdateBlocklist(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "blocklist-update")
		if !ok {
			return
		}

		var request updateBlocklistRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		action := strings.TrimSpace(request.Action)
		if action != "block" && action != "unblock" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "action must be block or unblock")
			return
		}
		jid := strings.TrimSpace(request.JID)
		if jid == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "jid is required")
			return
		}

		if err := instances.UpdateBlocklist(r.Context(), id, jid, action); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "blocklist-update").Err(err).Msg("update blocklist failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, blocklistUpdateResponse{Updated: true})
	}
}

// handleSetDisappearing sets the disappearing timer of a chat and answers 200
// with the command acknowledgement. A duration outside the 0/24h/168h/2160h allowlist
// answers 422 before the session is touched.
//
// @Summary Set the disappearing timer of a chat
// @Tags chats
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param chat path string true "Chat JID"
// @Param request body disappearingRequest true "Timer payload, one of 0, 24h, 168h, 2160h"
// @Success 200 {object} envelope{data=disappearingUpdatedResponse} "Timer updated, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid chat or duration outside the allowlist"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/{chat}/disappearing [put]
func handleSetDisappearing(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "chats-disappearing-set")
		if !ok {
			return
		}
		chat := strings.TrimSpace(r.PathValue("chat"))
		if chat == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "chat is required")
			return
		}

		var request disappearingRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		duration, ok := parseDisappearingDuration(request.Duration)
		if !ok {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "duration must be one of 0, 24h, 168h, 2160h")
			return
		}

		if err := instances.SetDisappearingTimer(r.Context(), id, chat, duration); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "chats-disappearing-set").Err(err).Msg("set disappearing timer failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, disappearingUpdatedResponse{Updated: true})
	}
}

// handleSetDefaultDisappearing sets the default disappearing timer for new
// chats and answers 200 with the command acknowledgement. A duration outside the
// 0/24h/168h/2160h allowlist answers 422 before the session is touched.
//
// @Summary Set the default disappearing timer
// @Tags chats
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body disappearingRequest true "Timer payload, one of 0, 24h, 168h, 2160h"
// @Success 200 {object} envelope{data=disappearingUpdatedResponse} "Default timer updated, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Duration outside the allowlist"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/chats/default-disappearing [put]
func handleSetDefaultDisappearing(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "chats-disappearing-default")
		if !ok {
			return
		}

		var request disappearingRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		duration, ok := parseDisappearingDuration(request.Duration)
		if !ok {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "duration must be one of 0, 24h, 168h, 2160h")
			return
		}

		if err := instances.SetDefaultDisappearingTimer(r.Context(), id, duration); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "chats-disappearing-default").Err(err).Msg("set default disappearing timer failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, disappearingUpdatedResponse{Updated: true})
	}
}

// handleSubscribePresence sends a single presence subscription signal for a
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
// @Success 200 {object} envelope{data=subscribePresenceResponse} "Subscribed, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or contact not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 422 {object} errorEnvelope "Malformed JID"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contacts/{jid}/subscribe [post]
func handleSubscribePresence(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "contacts-subscribe")
		if !ok {
			return
		}
		jid, ok := parityContactJID(w, r)
		if !ok {
			return
		}

		if err := instances.SubscribePresence(r.Context(), id, jid); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-subscribe").Err(err).Msg("subscribe presence failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, subscribePresenceResponse{Subscribed: true})
	}
}

// handleContactLink answers the own contact QR link. revoke=true invalidates
// the previous link; the adapter owns the rotation.
//
// @Summary Get the own contact link
// @Tags contacts
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param revoke query bool false "Revoke the previous link"
// @Success 200 {object} envelope{data=contactLinkResponse} "Link, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/contact-link [get]
func handleContactLink(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parityTarget(w, r, instances, log, "contacts-link")
		if !ok {
			return
		}

		revoke := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("revoke")), "true")
		link, err := instances.GetContactQRLink(r.Context(), id, revoke)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "contacts-link").Err(err).Msg("get contact link failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, contactLinkResponse{Link: link})
	}
}

// handleCreateNewsletter creates a channel and answers 201 with it. A blank
// title or a title above 100 characters, or a description above 500
// characters, answers 422 before the session is touched.
//
// @Summary Create a newsletter channel
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body createNewsletterRequest true "Channel payload, title 1..100, description 0..500"
// @Success 201 {object} envelope{data=channelEnvelope} "Created channel, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid title or description"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters [post]
func handleCreateNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-create")
		if !ok {
			return
		}

		var request createNewsletterRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		title := strings.TrimSpace(request.Title)
		if title == "" || utf8.RuneCountInString(title) > 100 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "title is required, max 100 characters")
			return
		}
		if utf8.RuneCountInString(request.Description) > 500 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "description must be at most 500 characters")
			return
		}

		newsletter, err := instances.CreateNewsletter(r.Context(), id, title, strings.TrimSpace(request.Description))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-create").Err(err).Msg("create newsletter failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusCreated, channelEnvelope{Channel: newNewsletterResponse(newsletter)})
	}
}

// handleMuteNewsletter mutes or unmutes a channel and answers 200 with the
// applied state. A blank channel answers 422 and an unknown channel 404 via
// ErrNewsletterNotFound.
//
// @Summary Mute or unmute a newsletter channel
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param channel path string true "Channel JID"
// @Param request body muteNewsletterRequest true "Mute payload"
// @Success 200 {object} envelope{data=muteNewsletterResponse} "Applied mute, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid channel"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/mute [post]
func handleMuteNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-mute")
		if !ok {
			return
		}
		channel, ok := parityChannel(w, r)
		if !ok {
			return
		}

		var request muteNewsletterRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}

		if err := instances.MuteNewsletter(r.Context(), id, channel, request.Muted); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-mute").Err(err).Msg("mute newsletter failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, muteNewsletterResponse(request))
	}
}

// handleMarkNewsletterViewed marks a batch of channel messages as viewed and
// answers 200. An empty batch or more than 100 server ids answers 422 before
// the session is touched.
//
// @Summary Mark newsletter messages as viewed
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param channel path string true "Channel JID"
// @Param request body markNewsletterViewedRequest true "Viewed payload, 1..100 server ids"
// @Success 200 {object} envelope{data=newsletterViewedResponse} "Viewed, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Empty batch or above the 100 cap"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/viewed [post]
func handleMarkNewsletterViewed(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-viewed")
		if !ok {
			return
		}
		channel, ok := parityChannel(w, r)
		if !ok {
			return
		}

		var request markNewsletterViewedRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		serverIDs := make([]string, 0, len(request.ServerIDs))
		for _, serverID := range request.ServerIDs {
			trimmed := strings.TrimSpace(serverID)
			if trimmed == "" {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "server_ids must be non-empty ids")
				return
			}
			serverIDs = append(serverIDs, trimmed)
		}
		if len(serverIDs) == 0 || len(serverIDs) > 100 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "server_ids must hold 1..100 ids")
			return
		}

		if err := instances.MarkNewsletterViewed(r.Context(), id, channel, serverIDs); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-viewed").Err(err).Msg("mark newsletter viewed failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newsletterViewedResponse{Viewed: true})
	}
}

// handleReactNewsletter sends a reaction to a channel message and answers 200.
// An empty reaction removes it. A missing server id answers 422 and an unknown
// message 404 via ErrNewsletterNotFound.
//
// @Summary React to a newsletter message
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param channel path string true "Channel JID"
// @Param request body reactNewsletterRequest true "Reaction payload, empty reaction removes it"
// @Success 200 {object} envelope{data=newsletterReactResponse} "Reacted, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown message"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Missing server id"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/reactions [post]
func handleReactNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-react")
		if !ok {
			return
		}
		channel, ok := parityChannel(w, r)
		if !ok {
			return
		}

		var request reactNewsletterRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		serverID := strings.TrimSpace(request.ServerID)
		if serverID == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "server_id is required")
			return
		}

		if err := instances.ReactNewsletter(r.Context(), id, channel, serverID, request.Reaction); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-react").Err(err).Msg("react newsletter failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newsletterReactResponse{Reacted: true})
	}
}
