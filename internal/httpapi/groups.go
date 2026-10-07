package httpapi

import (
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/instance"
)

const (
	// maxGroupNameLen mirrors the upstream subject limit: longer subjects are
	// rejected by the server, so the handler answers 422 before the session.
	maxGroupNameLen = 25
	// defaultGroupPhotoBytes caps the group picture upload when no media cap
	// is configured.
	defaultGroupPhotoBytes = 5 << 20
)

// groupParticipantResponse is one member of a group in the REST contract.
type groupParticipantResponse struct {
	JID          string `json:"jid"`
	IsAdmin      bool   `json:"is_admin"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

// groupResponse is the JSON representation of a group. InviteCode travels
// only on creation; UpdatedAt is the last metadata refresh (zero when no
// metadata store is wired).
type groupResponse struct {
	JID              string                     `json:"jid"`
	Name             string                     `json:"name"`
	Description      string                     `json:"description,omitempty"`
	Participants     []groupParticipantResponse `json:"participants"`
	ParticipantCount int                        `json:"participant_count"`
	InviteCode       string                     `json:"invite_code,omitempty"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

// createGroupRequest is the POST /instances/{id}/groups payload.
type createGroupRequest struct {
	Name         string   `json:"name"`
	Participants []string `json:"participants,omitempty"`
}

// updateGroupRequest is the PATCH /instances/{id}/groups/{group_id} payload:
// nil fields keep their upstream value while an explicit empty description
// clears the topic.
type updateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// updateParticipantsRequest is the POST
// /instances/{id}/groups/{group_id}/participants payload.
type updateParticipantsRequest struct {
	Action       string   `json:"action"`
	Participants []string `json:"participants"`
}

// groupInviteResponse is the answer to the invite consult and reset.
type groupInviteResponse struct {
	InviteCode string `json:"invite_code"`
}

// joinGroupRequest is the POST /instances/{id}/groups/join payload: the bare
// invite code or the full invite link.
type joinGroupRequest struct {
	InviteCode string `json:"invite_code"`
}

// groupJoinResponse is the answer to a join: the entered group JID.
type groupJoinResponse struct {
	JID string `json:"jid"`
}

// groupLeaveResponse is the answer to a leave.
type groupLeaveResponse struct {
	Left bool `json:"left"`
}

// groupUpdatedResponse is the answer to a picture or participants update.
type groupUpdatedResponse struct {
	Updated bool `json:"updated"`
}

// newGroupResponse maps a stored group to its JSON representation. A nil
// participant list is emitted as an empty array so the field keeps its array
// shape on every read.
func newGroupResponse(group instance.Group) groupResponse {
	participants := make([]groupParticipantResponse, 0, len(group.Participants))
	for _, p := range group.Participants {
		participants = append(participants, groupParticipantResponse{
			JID:          p.JID,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		})
	}
	return groupResponse{
		JID:              group.JID,
		Name:             group.Name,
		Description:      group.Description,
		Participants:     participants,
		ParticipantCount: group.ParticipantCount,
		UpdatedAt:        group.UpdatedAt,
	}
}

// groupNameValid reports whether name fits the upstream subject limit of 25
// characters.
func groupNameValid(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && utf8.RuneCountInString(trimmed) <= maxGroupNameLen
}

// validParticipantAction reports whether action is one of the group member
// operations the session accepts.
func validParticipantAction(action string) bool {
	switch action {
	case "add", "remove", "promote", "demote":
		return true
	}
	return false
}

// cleanJIDs trims and drops empty addresses, reporting false when any entry
// is blank.
func cleanJIDs(raw []string) ([]string, bool) {
	out := make([]string, 0, len(raw))
	for _, jid := range raw {
		trimmed := strings.TrimSpace(jid)
		if trimmed == "" {
			return nil, false
		}
		out = append(out, trimmed)
	}
	return out, true
}

// handleCreateGroup creates a group and answers 201 with its metadata plus
// the invite code. It loads the target first (404) and authorizes (403)
// before reading the body or touching the session; an offline session answers
// 409 and an invalid payload 422.
//
// When the post-create invite lookup fails the group already exists
// upstream, so the handler still answers 201 with the group payload and an
// empty invite_code (omitted JSON field); the client must reconcile the code
// via GET .../invite instead of retrying the create, which would duplicate
// the group.
//
// @Summary Create a group
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body createGroupRequest true "Group payload"
// @Success 201 {object} envelope{data=groupEnvelope} "Created, wrapped in the data envelope (invite_code empty when the post-create invite lookup fails; reconcile via GET .../invite, do not retry the create)"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid name or participants"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups [post]
func handleCreateGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "group-create").Msg("create group request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-create").Err(err).Msg("create group failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request createGroupRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		name := strings.TrimSpace(request.Name)
		if !groupNameValid(name) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name is required, max 25 characters")
			return
		}
		participants, ok := cleanJIDs(request.Participants)
		if !ok {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "participants must be non-empty addresses")
			return
		}

		group, err := instances.CreateGroup(r.Context(), id, instance.CreateGroupInput{Name: name, Participants: participants})
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-create").Err(err).Msg("create group failed")
			writeInstanceError(w, r, err)
			return
		}
		invite, err := instances.GetGroupInvite(r.Context(), id, group.JID)
		response := newGroupResponse(group)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-create").Err(err).Msg("create group invite failed")
			log.Debug().Str("instance_id", id.String()).Str("op", "group-create").Msg("create group result (partial, no invite)")
			JSON(w, r, http.StatusCreated, groupEnvelope{Group: response})
			return
		}
		response.InviteCode = invite
		log.Debug().Str("instance_id", id.String()).Str("op", "group-create").Msg("create group result")
		JSON(w, r, http.StatusCreated, groupEnvelope{Group: response})
	}
}

// handleGetGroup answers the live metadata of a group. An unknown group
// answers 404.
//
// @Summary Get a group
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} envelope{data=groupEnvelope} "Group, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id} [get]
func handleGetGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-get")
		if !ok {
			return
		}

		group, err := instances.GetGroup(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-get").Err(err).Msg("get group failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupEnvelope{Group: newGroupResponse(group)})
	}
}

// handleUpdateGroup applies the subject/topic patch and answers the command
// result (matrix: {updated:true}). At least one field must be present.
//
// @Summary Update a group
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Param request body updateGroupRequest true "Group patch"
// @Success 200 {object} envelope{data=groupUpdatedResponse} "Updated, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Empty patch or invalid values"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id} [patch]
func handleUpdateGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-update")
		if !ok {
			return
		}

		var request updateGroupRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if request.Name == nil && request.Description == nil {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name or description is required")
			return
		}
		input := instance.UpdateGroupInput{Description: request.Description}
		if request.Name != nil {
			name := strings.TrimSpace(*request.Name)
			if !groupNameValid(name) {
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name is required, max 25 characters")
				return
			}
			input.Name = &name
		}

		if _, err := instances.UpdateGroup(r.Context(), id, groupJID, input); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-update").Err(err).Msg("update group failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupUpdatedResponse{Updated: true})
	}
}

// handleSetGroupPhoto replaces the group picture with the raw image bytes of
// the request body (jpeg, png or webp).
//
// @Summary Update a group picture
// @Tags groups
// @Accept image/*
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Param image body string true "Raw image bytes with an image/* Content-Type (for example JPEG, PNG or WebP)"
// @Success 200 {object} envelope{data=groupUpdatedResponse} "Updated, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Image exceeds the cap"
// @Failure 422 {object} errorEnvelope "Missing or non image body"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/photo [put]
func handleSetGroupPhoto(instances InstanceService, log zerolog.Logger, maxBytes int64) http.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = defaultGroupPhotoBytes
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-photo")
		if !ok {
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "content type must be an image")
			return
		}
		image, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
		if err != nil {
			Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
		if len(image) == 0 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "image body is required")
			return
		}
		if int64(len(image)) > maxBytes {
			Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "image exceeds the size cap")
			return
		}

		if err := instances.SetGroupPhoto(r.Context(), id, groupJID, image); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-photo").Err(err).Msg("set group photo failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupUpdatedResponse{Updated: true})
	}
}

// handleUpdateGroupParticipants applies the member/admin operation and answers
// 200. Acting without group permission answers 403 and changes nothing.
//
// @Summary Manage group participants
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Param request body updateParticipantsRequest true "Participants payload"
// @Success 200 {object} envelope{data=groupUpdatedResponse} "Applied, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Unknown action or invalid participants"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/participants [post]
func handleUpdateGroupParticipants(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-participants")
		if !ok {
			return
		}

		var request updateParticipantsRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		action := strings.TrimSpace(request.Action)
		if !validParticipantAction(action) {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "action must be add, remove, promote or demote")
			return
		}
		participants, ok := cleanJIDs(request.Participants)
		if !ok || len(participants) == 0 {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "participants must be non-empty addresses")
			return
		}

		if err := instances.UpdateGroupParticipants(r.Context(), id, groupJID, action, participants); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-participants").Err(err).Msg("update group participants failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupUpdatedResponse{Updated: true})
	}
}

// handleGetGroupInvite answers the current invite code without revoking it.
//
// @Summary Get a group invite code
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} envelope{data=groupInviteResponse} "Invite, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/invite [get]
func handleGetGroupInvite(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-invite")
		if !ok {
			return
		}

		code, err := instances.GetGroupInvite(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-invite").Err(err).Msg("get group invite failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupInviteResponse{InviteCode: code})
	}
}

// handleResetGroupInvite revokes the current invite code and answers the fresh
// one.
//
// @Summary Reset a group invite code
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} envelope{data=groupInviteResponse} "Fresh invite, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/invite/reset [post]
func handleResetGroupInvite(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-invite-reset")
		if !ok {
			return
		}

		code, err := instances.ResetGroupInvite(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-invite-reset").Err(err).Msg("reset group invite failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupInviteResponse{InviteCode: code})
	}
}

// handleJoinGroup enters the group behind the invite code and answers its JID.
// An unknown code answers 404.
//
// @Summary Join a group by invite
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body joinGroupRequest true "Join payload"
// @Success 200 {object} envelope{data=groupJoinResponse} "Joined group, wrapped in the data envelope"
// @Failure 400 {object} errorEnvelope "Malformed body"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance not found, or unknown invite"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} errorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} errorEnvelope "Invalid invite code"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/join [post]
func handleJoinGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := instanceID(w, r)
		if !ok {
			return
		}
		if denyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "group-join").Msg("join group request")

		stored, err := instances.Get(r.Context(), id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-join").Err(err).Msg("join group failed")
			writeInstanceError(w, r, err)
			return
		}
		if err := authorizeInstance(r, stored); err != nil {
			writeForbidden(w, r)
			return
		}

		var request joinGroupRequest
		if err := decodeJSONBody(w, r, &request); err != nil {
			writeJSONBodyError(w, r, err)
			return
		}
		if strings.TrimSpace(request.InviteCode) == "" {
			Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invite_code is required")
			return
		}

		groupJID, err := instances.JoinGroup(r.Context(), id, strings.TrimSpace(request.InviteCode))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-join").Err(err).Msg("join group failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupJoinResponse{JID: groupJID})
	}
}

// handleLeaveGroup removes the instance from the group.
//
// @Summary Leave a group
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} envelope{data=groupLeaveResponse} "Left, wrapped in the data envelope"
// @Failure 401 {object} errorEnvelope "Missing or invalid credential"
// @Failure 403 {object} errorEnvelope "Not the owner"
// @Failure 404 {object} errorEnvelope "Instance or group not found"
// @Failure 409 {object} errorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} errorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/leave [post]
func handleLeaveGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := groupTarget(w, r, instances, log, "group-leave")
		if !ok {
			return
		}

		if err := instances.LeaveGroup(r.Context(), id, groupJID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-leave").Err(err).Msg("leave group failed")
			writeInstanceError(w, r, err)
			return
		}
		JSON(w, r, http.StatusOK, groupLeaveResponse{Left: true})
	}
}

// groupTarget loads the instance (404) and authorizes (403) for the group
// handlers sharing the {group_id} path, returning the group JID. A blank
// group id answers 404: it names no group.
func groupTarget(w http.ResponseWriter, r *http.Request, instances InstanceService, log zerolog.Logger, op string) (uuid.UUID, string, bool) {
	id, ok := instanceID(w, r)
	if !ok {
		return uuid.Nil, "", false
	}
	if denyForeignInstanceKey(w, r, id) {
		return uuid.Nil, "", false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("group request")

	stored, err := instances.Get(r.Context(), id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("group request failed")
		writeInstanceError(w, r, err)
		return uuid.Nil, "", false
	}
	if err := authorizeInstance(r, stored); err != nil {
		writeForbidden(w, r)
		return uuid.Nil, "", false
	}

	groupJID := strings.TrimSpace(r.PathValue("group_id"))
	if groupJID == "" {
		Error(w, r, http.StatusNotFound, "not_found", "group not found")
		return uuid.Nil, "", false
	}
	return id, groupJID, true
}
