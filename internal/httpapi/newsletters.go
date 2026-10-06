package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/instance"
)

const (
	// defaultNewslettersLimit is the page size used when the request omits
	// limit, matching the other collection listings.
	defaultNewslettersLimit = 50
	// maxNewslettersLimit caps the page size a client can request, matching
	// the other collection listings.
	maxNewslettersLimit = 100
)

// newsletterResponse is the JSON representation of a channel. UpdatedAt is the
// last metadata refresh (zero when no metadata store is wired).
type newsletterResponse struct {
	Channel       string    `json:"channel"`
	Title         string    `json:"title"`
	Description   string    `json:"description,omitempty"`
	FollowerCount int       `json:"follower_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// newsletterListResponse is one page of followed channels; each element
// nests the channel DTO under its own key (matrix §1).
type newsletterListResponse struct {
	Items      []channelEnvelope `json:"items"`
	NextCursor string            `json:"next_cursor"`
}

// followNewsletterRequest is the follow/unfollow payload.
type followNewsletterRequest struct {
	Channel string `json:"channel"`
}

// newsletterFollowResponse is the answer to a follow/unfollow.
type newsletterFollowResponse struct {
	Followed bool `json:"followed"`
}

// newNewsletterResponse maps a stored channel to its JSON representation.
func newNewsletterResponse(newsletter instance.Newsletter) newsletterResponse {
	return newsletterResponse{
		Channel:       newsletter.ChannelJID,
		Title:         newsletter.Title,
		Description:   newsletter.Description,
		FollowerCount: newsletter.FollowerCount,
		UpdatedAt:     newsletter.UpdatedAt,
	}
}

// newsletterTarget loads the instance (404) and authorizes (403) for the
// newsletter handlers, returning the instance id.
func newsletterTarget(w http.ResponseWriter, r *http.Request, instances InstanceService, log zerolog.Logger, op string) (uuid.UUID, bool) {
	id, ok := instanceID(w, r)
	if !ok {
		return uuid.Nil, false
	}
	if denyForeignInstanceKey(w, r, id) {
		return uuid.Nil, false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("newsletter request")

	stored, err := instances.Get(r.Context(), id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("newsletter request failed")
		writeInstanceError(w, r, err)
		return uuid.Nil, false
	}
	if err := authorizeInstance(r, stored); err != nil {
		writeForbidden(w, r)
		return uuid.Nil, false
	}
	return id, true
}

// newsletterChannel decodes the channel of a follow/unfollow body, answering
// 422 on a malformed body or a blank channel without touching the session.
func newsletterChannel(w http.ResponseWriter, r *http.Request) (string, bool) {
	var request followNewsletterRequest
	if err := decodeJSONBody(w, r, &request); err != nil {
		writeJSONBodyError(w, r, err)
		return "", false
	}
	channel := strings.TrimSpace(request.Channel)
	if channel == "" {
		Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "channel is required")
		return "", false
	}
	return channel, true
}

// handleFollowNewsletter subscribes the instance to the channel and answers
// 200. An unknown channel answers 404 and a disconnected session 409.
//
// @Summary Follow a newsletter channel
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body followNewsletterRequest true "Follow payload"
// @Success 200 {object} envelope{data=newsletterFollowResponse} "Followed, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid channel"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/follow [post]
func handleFollowNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-follow")
		if !ok {
			return
		}
		channel, ok := newsletterChannel(w, r)
		if !ok {
			return
		}

		if err := instances.FollowNewsletter(r.Context(), id, channel); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-follow").Err(err).Msg("follow newsletter failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newsletterFollowResponse{Followed: true})
	}
}

// handleUnfollowNewsletter ends the subscription of the instance to the
// channel and answers 200.
//
// @Summary Unfollow a newsletter channel
// @Tags newsletters
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body followNewsletterRequest true "Unfollow payload"
// @Success 200 {object} envelope{data=newsletterFollowResponse} "Unfollowed, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid channel"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/unfollow [post]
func handleUnfollowNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-unfollow")
		if !ok {
			return
		}
		channel, ok := newsletterChannel(w, r)
		if !ok {
			return
		}

		if err := instances.UnfollowNewsletter(r.Context(), id, channel); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-unfollow").Err(err).Msg("unfollow newsletter failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, newsletterFollowResponse{Followed: false})
	}
}

// handleGetNewsletter answers the live metadata of the channel, refreshed on
// demand. An unknown channel answers 404.
//
// @Summary Get a newsletter channel
// @Tags newsletters
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param channel path string true "Channel JID"
// @Success 200 {object} envelope{data=channelEnvelope} "Channel, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown channel"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters/{channel} [get]
func handleGetNewsletter(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-get")
		if !ok {
			return
		}
		channel := strings.TrimSpace(r.PathValue("channel"))
		if channel == "" {
			Error(w, r, http.StatusNotFound, "not_found", "newsletter not found")
			return
		}

		newsletter, err := instances.GetNewsletter(r.Context(), id, channel)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-get").Err(err).Msg("get newsletter failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, channelEnvelope{Channel: newNewsletterResponse(newsletter)})
	}
}

// handleListNewsletters answers one page of the followed channels with its
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
// @Success 200 {object} envelope{data=newsletterListResponse} "One page, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/newsletters [get]
func handleListNewsletters(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := newsletterTarget(w, r, instances, log, "newsletter-list")
		if !ok {
			return
		}

		items, next, err := instances.ListNewsletters(r.Context(), id,
			parseLimit(r.URL.Query().Get("limit"), defaultNewslettersLimit, maxNewslettersLimit),
			r.URL.Query().Get("cursor"))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "newsletter-list").Err(err).Msg("list newsletters failed")
			writeInstanceError(w, r, err)
			return
		}
		response := newsletterListResponse{Items: make([]channelEnvelope, 0, len(items)), NextCursor: next}
		for _, item := range items {
			response.Items = append(response.Items, channelEnvelope{Channel: newNewsletterResponse(item)})
		}
		JSON(w, r, http.StatusOK, response)
	}
}
