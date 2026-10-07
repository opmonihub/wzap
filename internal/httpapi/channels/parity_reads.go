package channels

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/session"
)

// NewsletterMessageResponse is one channel message: the upstream server id,
// its text snapshot and the publish moment.
type NewsletterMessageResponse struct {
	ServerID  string    `json:"server_id" binding:"required"`
	Content   string    `json:"content" binding:"required"`
	Timestamp time.Time `json:"timestamp" binding:"required"`
}

// NewsletterMessagesResponse is one page of channel messages.
type NewsletterMessagesResponse struct {
	Messages   []NewsletterMessageResponse `json:"messages" binding:"required"`
	NextCursor string                      `json:"next_cursor,omitempty"`
}

// NewsletterUpdatesResponse answers the pending message updates of a channel.
type NewsletterUpdatesResponse struct {
	Messages []NewsletterMessageResponse `json:"messages" binding:"required"`
}

// NewNewsletterMessageResponse maps one session channel message to its JSON
// representation.
func NewNewsletterMessageResponse(msg session.NewsletterMessage) NewsletterMessageResponse {
	return NewsletterMessageResponse{
		ServerID:  msg.ServerID,
		Content:   msg.Content,
		Timestamp: msg.Timestamp,
	}
}

// HandleGetNewsletterMessages pages the messages of a channel with the
// upstream cursor. A blank channel answers 422 and an unknown channel 404 via
// ErrNewsletterNotFound.
//
// @Summary List newsletter messages
// @Tags newsletters
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param channel path string true "Channel JID"
// @Param limit query int false "Page size, default 50, max 100"
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} core.Envelope{data=NewsletterMessagesResponse} "One page, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Invalid channel"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/messages [get]
func HandleGetNewsletterMessages(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "newsletter-messages")
		if !ok {
			return
		}
		channel, ok := core.ParityChannel(w, r)
		if !ok {
			return
		}

		items, next, err := instances.GetNewsletterMessages(r.Context(), id, channel,
			r.URL.Query().Get("cursor"),
			core.ParseLimit(r.URL.Query().Get("limit"), core.DefaultParityLimit, core.MaxParityLimit))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-messages").Err(err).Msg("get newsletter messages failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		response := NewsletterMessagesResponse{Messages: make([]NewsletterMessageResponse, 0, len(items)), NextCursor: next}
		for _, item := range items {
			response.Messages = append(response.Messages, NewNewsletterMessageResponse(item))
		}
		core.JSON(w, r, http.StatusOK, response)
	}
}

// HandleGetNewsletterUpdates answers the pending message updates of a channel.
// A blank channel answers 422 and an unknown channel 404 via
// ErrNewsletterNotFound.
//
// @Summary List newsletter updates
// @Tags newsletters
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param channel path string true "Channel JID"
// @Success 200 {object} core.Envelope{data=NewsletterUpdatesResponse} "Updates, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Invalid channel"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel}/updates [get]
func HandleGetNewsletterUpdates(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "newsletter-updates")
		if !ok {
			return
		}
		channel, ok := core.ParityChannel(w, r)
		if !ok {
			return
		}

		items, err := instances.GetNewsletterUpdates(r.Context(), id, channel)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-updates").Err(err).Msg("get newsletter updates failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		response := NewsletterUpdatesResponse{Messages: make([]NewsletterMessageResponse, 0, len(items))}
		for _, item := range items {
			response.Messages = append(response.Messages, NewNewsletterMessageResponse(item))
		}
		core.JSON(w, r, http.StatusOK, response)
	}
}
