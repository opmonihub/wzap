package groups

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
)

// JoinedGroupsResponse is one page of the groups the instance belongs to;
// elements contain group DTOs directly.
type JoinedGroupsResponse struct {
	Groups     []representation.GroupResponse `json:"groups" binding:"required"`
	NextCursor string                         `json:"next_cursor,omitempty"`
}

// HandleListJoinedGroups answers one page of the groups the instance belongs
// to with its next cursor. A disconnected session answers 409.
//
// @Summary List joined groups
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param limit query int false "Page size, default 50, max 100"
// @Param cursor query string false "Opaque pagination cursor"
// @Success 200 {object} core.Envelope{data=JoinedGroupsResponse} "One page, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups [get]
func HandleListJoinedGroups(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "groups-list")
		if !ok {
			return
		}

		items, next, err := instances.GetJoinedGroups(r.Context(), id,
			core.ParseLimit(r.URL.Query().Get("limit"), core.DefaultParityLimit, core.MaxParityLimit),
			r.URL.Query().Get("cursor"))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "groups-list").Err(err).Msg("list joined groups failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		response := JoinedGroupsResponse{Groups: make([]representation.GroupResponse, 0, len(items)), NextCursor: next}
		for _, item := range items {
			response.Groups = append(response.Groups, representation.NewGroupResponse(item))
		}
		core.JSON(w, r, http.StatusOK, response)
	}
}

// HandleInvitePreview resolves an invite code to the group preview without
// entering the group. A blank or unknown code answers 422: a preview is
// addressable only by its exact code, never a 404 listing probe.
//
// @Summary Preview a group invite
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param code query string true "Invite code or link"
// @Success 200 {object} core.Envelope{data=representation.GroupEnvelope} "Preview, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 422 {object} core.ErrorEnvelope "Invalid or expired invite"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/invite-preview [get]
func HandleInvitePreview(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.ParityTarget(w, r, instances, log, "groups-invite-preview")
		if !ok {
			return
		}

		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			code = strings.TrimSpace(r.URL.Query().Get("invite_code"))
		}
		if code == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "code is required")
			return
		}

		group, err := instances.GetGroupInvitePreview(r.Context(), id, code)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "groups-invite-preview").Err(err).Msg("get invite preview failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.GroupEnvelope{Group: representation.NewGroupResponse(group)})
	}
}
