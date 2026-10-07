package channels

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
)

const (
	// defaultNewslettersLimit is the page size used when the request omits
	// limit, matching the other collection listings.
	DefaultNewslettersLimit = 50
	// maxNewslettersLimit caps the page size a client can request, matching
	// the other collection listings.
	MaxNewslettersLimit = 100
)

// NewsletterListResponse is one page of followed channels; elements contain channel metadata directly.
type NewsletterListResponse struct {
	Channels   []representation.NewsletterResponse `json:"channels" binding:"required"`
	NextCursor string                              `json:"next_cursor,omitempty"`
}

// FollowNewsletterRequest is the follow/unfollow payload.
type FollowNewsletterRequest struct {
	Channel string `json:"channel"`
}

// NewsletterFollowResponse is the answer to a follow/unfollow.
type NewsletterFollowResponse struct {
	Followed bool `json:"followed" binding:"required"`
}

// NewsletterTarget loads the instance (404) and authorizes (403) for the
// newsletter handlers, returning the instance id.
func NewsletterTarget(w http.ResponseWriter, r *http.Request, instances InstanceService, log zerolog.Logger, op string) (uuid.UUID, bool) {
	id, ok := core.InstanceID(w, r)
	if !ok {
		return uuid.Nil, false
	}
	if core.DenyForeignInstanceKey(w, r, id) {
		return uuid.Nil, false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("newsletter request")

	stored, err := core.LoadInstance(r, instances, id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("newsletter request failed")
		core.WriteInstanceError(w, r, err)
		return uuid.Nil, false
	}
	if err := core.AuthorizeInstance(r, stored); err != nil {
		core.WriteForbidden(w, r)
		return uuid.Nil, false
	}
	return id, true
}

// NewsletterChannel decodes the channel of a follow/unfollow body, answering
// 422 on a malformed body or a blank channel without touching the session.
func NewsletterChannel(w http.ResponseWriter, r *http.Request) (string, bool) {
	var request FollowNewsletterRequest
	if err := core.DecodeJSONBody(w, r, &request); err != nil {
		core.WriteJSONBodyError(w, r, err)
		return "", false
	}
	channel := strings.TrimSpace(request.Channel)
	if channel == "" {
		core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "channel is required")
		return "", false
	}
	return channel, true
}

// HandleFollowNewsletter subscribes the instance to the channel and answers
// 200. An unknown channel answers 404 and a disconnected session 409.
//
// @Summary Follow a newsletter channel
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body FollowNewsletterRequest true "Follow payload"
// @Success 200 {object} core.Envelope{data=NewsletterFollowResponse} "Followed, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid channel"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/follow [post]
func HandleFollowNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-follow")
		if !ok {
			return
		}
		channel, ok := NewsletterChannel(w, r)
		if !ok {
			return
		}

		if err := instances.FollowNewsletter(r.Context(), id, channel); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-follow").Err(err).Msg("follow newsletter failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, NewsletterFollowResponse{Followed: true})
	}
}

// HandleUnfollowNewsletter ends the subscription of the instance to the
// channel and answers 200.
//
// @Summary Unfollow a newsletter channel
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body FollowNewsletterRequest true "Unfollow payload"
// @Success 200 {object} core.Envelope{data=NewsletterFollowResponse} "Unfollowed, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid channel"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/unfollow [post]
func HandleUnfollowNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-unfollow")
		if !ok {
			return
		}
		channel, ok := NewsletterChannel(w, r)
		if !ok {
			return
		}

		if err := instances.UnfollowNewsletter(r.Context(), id, channel); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-unfollow").Err(err).Msg("unfollow newsletter failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, NewsletterFollowResponse{Followed: false})
	}
}

// HandleGetNewsletter answers the live metadata of the channel, refreshed on
// demand. An unknown channel answers 404.
//
// @Summary Get a newsletter channel
// @Tags newsletters
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param channel path string true "Channel JID"
// @Success 200 {object} core.Envelope{data=representation.ChannelEnvelope} "Channel, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel} [get]
func HandleGetNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-get")
		if !ok {
			return
		}
		channel := strings.TrimSpace(core.PathParam(r, "channel"))
		if channel == "" {
			core.Error(w, r, http.StatusNotFound, "not_found", "newsletter not found")
			return
		}

		newsletter, err := instances.GetNewsletter(r.Context(), id, channel)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-get").Err(err).Msg("get newsletter failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.ChannelEnvelope{Channel: representation.NewNewsletterResponse(newsletter)})
	}
}

// HandleListNewsletters answers one page of the followed channels with its
// next cursor. The page size defaults to 50 and caps at 100 like the other
// collection listings.
//
// @Summary List newsletter channels
// @Tags newsletters
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param limit query int false "Page size, default 50, max 100"
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} core.Envelope{data=NewsletterListResponse} "One page, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters [get]
func HandleListNewsletters(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := NewsletterTarget(w, r, instances, log, "newsletter-list")
		if !ok {
			return
		}

		items, next, err := instances.ListNewsletters(r.Context(), id,
			core.ParseLimit(r.URL.Query().Get("limit"), DefaultNewslettersLimit, MaxNewslettersLimit),
			r.URL.Query().Get("cursor"))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-list").Err(err).Msg("list newsletters failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		response := NewsletterListResponse{Channels: make([]representation.NewsletterResponse, 0, len(items)), NextCursor: next}
		for _, item := range items {
			response.Channels = append(response.Channels, representation.NewNewsletterResponse(item))
		}
		core.JSON(w, r, http.StatusOK, response)
	}
}
