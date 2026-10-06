package httpapi

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// groupRequestsResponse answers the pending join requests of a group.
type groupRequestsResponse struct {
	Participants []groupParticipantResponse `json:"participants"`
}

// updateGroupRequestsRequest is the POST
// /instances/{id}/groups/{group_id}/requests payload.
type updateGroupRequestsRequest struct {
	Action       string   `json:"action"`
	Participants []string `json:"participants"`
}

// updateGroupSettingsRequest is the PATCH
// /instances/{id}/groups/{group_id}/settings payload: nil fields keep their
// upstream value.
type updateGroupSettingsRequest struct {
	Announce      *bool   `json:"announce"`
	Locked        *bool   `json:"locked"`
	JoinApproval  *string `json:"join_approval"`
	MemberAddMode *string `json:"member_add_mode"`
}

// validGroupRequestAction reports whether action is one of the join-request
// operations the session accepts.
func validGroupRequestAction(action string) bool {
	switch action {
	case "approve", "decline":
		return true
	}
	return false
}

// handleGroupRequests answers the pending join requests of a group. An unknown
// group answers 404 and a disconnected session 409.
//
// @Summary List group join requests
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} envelope{data=groupRequestsResponse} "Pending requests, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/requests [get]
func handleGroupRequests(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-requests")
		if !ok {
			return
		}

		participants, err := instances.GetGroupRequests(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-requests").Err(err).Msg("get group requests failed")
			writeInstanceError(w, r, err)
			return
		}
		response := groupRequestsResponse{Participants: make([]groupParticipantResponse, 0, len(participants))}
		for _, p := range participants {
			response.Participants = append(response.Participants, groupParticipantResponse{
				JID:          p.JID,
				IsAdmin:      p.IsAdmin,
				IsSuperAdmin: p.IsSuperAdmin,
			})
		}
		JSON(w, r, http.StatusOK, response)
	}
}

// handleUpdateGroupRequests approves or declines a batch of pending join
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
// @Param request body updateGroupRequestsRequest true "Requests payload"
// @Success 200 {object} envelope{data=groupUpdatedResponse} "Applied, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected, or key already in flight; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Unknown action or invalid participants"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/requests [post]
func handleUpdateGroupRequests(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-requests-update")
		if !ok {
			return
		}

		var request updateGroupRequestsRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		action := strings.TrimSpace(request.Action)
		if !validGroupRequestAction(action) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "action must be approve or decline")
			return
		}
		participants, ok := cleanJIDs(request.Participants)
		if !ok || len(participants) == 0 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "participants must be non-empty addresses")
			return
		}

		if err := instances.UpdateGroupRequests(r.Context(), id, groupJID, action, participants); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-requests-update").Err(err).Msg("update group requests failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupUpdatedResponse{Updated: true})
	}
}

// handleUpdateGroupSettings applies the group modes present in the body and
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
// @Param request body updateGroupSettingsRequest true "Settings payload"
// @Success 200 {object} envelope{data=groupUpdatedResponse} "Updated, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_ambiguous for a legacy name matching multiple instances"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "No fields or value outside the allowlist"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/settings [patch]
func handleUpdateGroupSettings(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-settings")
		if !ok {
			return
		}

		var request updateGroupSettingsRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if request.Announce == nil && request.Locked == nil && request.JoinApproval == nil && request.MemberAddMode == nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "at least one setting is required")
			return
		}
		var joinApproval, memberAddMode *string
		if request.JoinApproval != nil {
			trimmed := strings.TrimSpace(*request.JoinApproval)
			if trimmed != "on" && trimmed != "off" {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "join_approval must be on or off")
				return
			}
			joinApproval = &trimmed
		}
		if request.MemberAddMode != nil {
			trimmed := strings.TrimSpace(*request.MemberAddMode)
			if trimmed != "all_members" && trimmed != "admin_only" {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "member_add_mode must be all_members or admin_only")
				return
			}
			memberAddMode = &trimmed
		}

		if _, err := instances.UpdateGroupSettings(r.Context(), id, groupJID, request.Announce, request.Locked, joinApproval, memberAddMode); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-settings").Err(err).Msg("update group settings failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupUpdatedResponse{Updated: true})
	}
}
