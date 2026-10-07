package channels

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
)

// CreateNewsletterRequest is the POST /instances/{id}/newsletters payload.
type CreateNewsletterRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// MuteNewsletterRequest is the POST
// /instances/{id}/newsletters/{channel}/mute payload.
type MuteNewsletterRequest struct {
	Muted bool `json:"muted"`
}

// MuteNewsletterResponse is the answer to a mute/unmute.
type MuteNewsletterResponse struct {
	Muted bool `json:"muted" binding:"required"`
}

// MarkNewsletterViewedRequest is the POST
// /instances/{id}/newsletters/{channel}/viewed payload.
type MarkNewsletterViewedRequest struct {
	ServerIDs []string `json:"server_ids"`
}

// NewsletterViewedResponse is the answer to a mark-viewed batch.
type NewsletterViewedResponse struct {
	Viewed bool `json:"viewed" binding:"required"`
}

// ReactNewsletterRequest is the POST
// /instances/{id}/newsletters/{channel}/reactions payload: an empty reaction
// removes the reaction.
type ReactNewsletterRequest struct {
	ServerID string `json:"server_id"`
	Reaction string `json:"reaction"`
}

// NewsletterReactResponse is the answer to a reaction.
type NewsletterReactResponse struct {
	Reacted bool `json:"reacted" binding:"required"`
}

// HandleCreateNewsletter creates a channel and answers 201 with it. A blank
// title or a title above 100 characters, or a description above 500
// characters, answers 422 before the session is touched.
//
// @Summary Create a newsletter channel
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body CreateNewsletterRequest true "Channel payload, title 1..100, description 0..500"
// @Success 201 {object} core.Envelope{data=representation.ChannelEnvelope} "Created channel, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid title or description"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters [post]
func HandleCreateNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-create")
		if !ok {
			return
		}

		var request CreateNewsletterRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		title := strings.TrimSpace(request.Title)
		if title == "" || utf8.RuneCountInString(title) > 100 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "title is required, max 100 characters")
			return
		}
		if utf8.RuneCountInString(request.Description) > 500 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "description must be at most 500 characters")
			return
		}

		newsletter, err := instances.CreateNewsletter(r.Context(), id, title, strings.TrimSpace(request.Description))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-create").Err(err).Msg("create newsletter failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusCreated, representation.ChannelEnvelope{Channel: representation.NewNewsletterResponse(newsletter)})
	}
}

// HandleMuteNewsletter mutes or unmutes a channel and answers 200 with the
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
// @Param request body MuteNewsletterRequest true "Mute payload"
// @Success 200 {object} core.Envelope{data=MuteNewsletterResponse} "Applied mute, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid channel"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/mute [post]
func HandleMuteNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-mute")
		if !ok {
			return
		}
		channel, ok := core.ParityChannel(w, r)
		if !ok {
			return
		}

		var request MuteNewsletterRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}

		if err := instances.MuteNewsletter(r.Context(), id, channel, request.Muted); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-mute").Err(err).Msg("mute newsletter failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, MuteNewsletterResponse(request))
	}
}

// HandleMarkNewsletterViewed marks a batch of channel messages as viewed and
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
// @Param request body MarkNewsletterViewedRequest true "Viewed payload, 1..100 server ids"
// @Success 200 {object} core.Envelope{data=NewsletterViewedResponse} "Viewed, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Empty batch or above the 100 cap"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/viewed [post]
func HandleMarkNewsletterViewed(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-viewed")
		if !ok {
			return
		}
		channel, ok := core.ParityChannel(w, r)
		if !ok {
			return
		}

		var request MarkNewsletterViewedRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		serverIDs := make([]string, 0, len(request.ServerIDs))
		for _, serverID := range request.ServerIDs {
			trimmed := strings.TrimSpace(serverID)
			if trimmed == "" {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "server_ids must be non-empty ids")
				return
			}
			serverIDs = append(serverIDs, trimmed)
		}
		if len(serverIDs) == 0 || len(serverIDs) > 100 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "server_ids must hold 1..100 ids")
			return
		}

		if err := instances.MarkNewsletterViewed(r.Context(), id, channel, serverIDs); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-viewed").Err(err).Msg("mark newsletter viewed failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, NewsletterViewedResponse{Viewed: true})
	}
}

// HandleReactNewsletter sends a reaction to a channel message and answers 200.
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
// @Param request body ReactNewsletterRequest true "Reaction payload, empty reaction removes it"
// @Success 200 {object} core.Envelope{data=NewsletterReactResponse} "Reacted, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown message"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Missing server id"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/reactions [post]
func HandleReactNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-react")
		if !ok {
			return
		}
		channel, ok := core.ParityChannel(w, r)
		if !ok {
			return
		}

		var request ReactNewsletterRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		serverID := strings.TrimSpace(request.ServerID)
		if serverID == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "server_id is required")
			return
		}

		if err := instances.ReactNewsletter(r.Context(), id, channel, serverID, request.Reaction); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-react").Err(err).Msg("react newsletter failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, NewsletterReactResponse{Reacted: true})
	}
}
