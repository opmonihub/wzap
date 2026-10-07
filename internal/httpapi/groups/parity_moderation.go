package groups

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
)

// GroupRequestsResponse answers the pending join requests of a group.
type GroupRequestsResponse struct {
	Participants []representation.GroupParticipantResponse `json:"participants" binding:"required"`
}

// UpdateGroupRequestsRequest is the POST
// /instances/{id}/groups/{group_id}/requests payload.
type UpdateGroupRequestsRequest struct {
	Action       string   `json:"action"`
	Participants []string `json:"participants"`
}

// UpdateGroupSettingsRequest is the PATCH
// /instances/{id}/groups/{group_id}/settings payload: nil fields keep their
// upstream value.
type UpdateGroupSettingsRequest struct {
	Announce      *bool   `json:"announce"`
	Locked        *bool   `json:"locked"`
	JoinApproval  *string `json:"join_approval"`
	MemberAddMode *string `json:"member_add_mode"`
}

// ValidGroupRequestAction reports whether action is one of the join-request
// operations the session accepts.
func ValidGroupRequestAction(action string) bool {
	switch action {
	case "approve", "decline":
		return true
	}
	return false
}

// HandleGroupRequests answers the pending join requests of a group. An unknown
// group answers 404 and a disconnected session 409.
//
// @Summary List group join requests
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} core.Envelope{data=GroupRequestsResponse} "Pending requests, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/requests [get]
func HandleGroupRequests(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-requests")
		if !ok {
			return
		}

		participants, err := instances.GetGroupRequests(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-requests").Err(err).Msg("get group requests failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		response := GroupRequestsResponse{Participants: make([]representation.GroupParticipantResponse, 0, len(participants))}
		for _, p := range participants {
			response.Participants = append(response.Participants, representation.GroupParticipantResponse{
				JID:          p.JID,
				IsAdmin:      p.IsAdmin,
				IsSuperAdmin: p.IsSuperAdmin,
			})
		}
		core.JSON(w, r, http.StatusOK, response)
	}
}

// HandleUpdateGroupRequests approves or declines a batch of pending join
// requests and answers 200. A bad action or an empty batch answers 422 before
// the session is touched; acting without group permission answers 403.
//
// @Summary Approve or decline group join requests
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param Idempotency-Key header string false "Idempotency key, 24h replay per instance"
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Param request body UpdateGroupRequestsRequest true "Requests payload"
// @Success 200 {object} core.Envelope{data=GroupUpdatedResponse} "Applied, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected, or key already in flight; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Unknown action or invalid participants"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/requests [post]
func HandleUpdateGroupRequests(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-requests-update")
		if !ok {
			return
		}

		var request UpdateGroupRequestsRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		action := strings.TrimSpace(request.Action)
		if !ValidGroupRequestAction(action) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "action must be approve or decline")
			return
		}
		participants, ok := CleanJIDs(request.Participants)
		if !ok || len(participants) == 0 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "participants must be non-empty addresses")
			return
		}

		if err := instances.UpdateGroupRequests(r.Context(), id, groupJID, action, participants); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-requests-update").Err(err).Msg("update group requests failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupUpdatedResponse{Updated: true})
	}
}

// HandleUpdateGroupSettings applies the group modes present in the body and
// answers 200 with the command result (matrix: {updated:true}). A body
// without fields, or a join_approval/member_add_mode outside its allowlist,
// answers 422 before the session is touched.
//
// @Summary Update group settings
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Param request body UpdateGroupSettingsRequest true "Settings payload"
// @Success 200 {object} core.Envelope{data=GroupUpdatedResponse} "Updated, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "No fields or value outside the allowlist"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/settings [patch]
func HandleUpdateGroupSettings(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-settings")
		if !ok {
			return
		}

		var request UpdateGroupSettingsRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if request.Announce == nil && request.Locked == nil && request.JoinApproval == nil && request.MemberAddMode == nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "at least one setting is required")
			return
		}
		var joinApproval, memberAddMode *string
		if request.JoinApproval != nil {
			trimmed := strings.TrimSpace(*request.JoinApproval)
			if trimmed != "on" && trimmed != "off" {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "join_approval must be on or off")
				return
			}
			joinApproval = &trimmed
		}
		if request.MemberAddMode != nil {
			trimmed := strings.TrimSpace(*request.MemberAddMode)
			if trimmed != "all_members" && trimmed != "admin_only" {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "member_add_mode must be all_members or admin_only")
				return
			}
			memberAddMode = &trimmed
		}

		if _, err := instances.UpdateGroupSettings(r.Context(), id, groupJID, request.Announce, request.Locked, joinApproval, memberAddMode); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-settings").Err(err).Msg("update group settings failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupUpdatedResponse{Updated: true})
	}
}
