package groups

import (
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
	"wzap/internal/instance"
)

const (
	// maxGroupNameLen mirrors the upstream subject limit: longer subjects are
	// rejected by the server, so the handler answers 422 before the session.
	MaxGroupNameLen = 25
	// defaultGroupPhotoBytes caps the group picture upload when no media cap
	// is configured.
	DefaultGroupPhotoBytes = 5 << 20
)

// CreateGroupRequest is the POST /instances/{id}/groups payload.
type CreateGroupRequest struct {
	Name         string   `json:"name"`
	Participants []string `json:"participants,omitempty"`
}

// UpdateGroupRequest is the PATCH /instances/{id}/groups/{group_id} payload:
// nil fields keep their upstream value while an explicit empty description
// clears the topic.
type UpdateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// UpdateParticipantsRequest is the POST
// /instances/{id}/groups/{group_id}/participants payload.
type UpdateParticipantsRequest struct {
	Action       string   `json:"action"`
	Participants []string `json:"participants"`
}

// GroupInviteResponse is the answer to the invite consult and reset.
type GroupInviteResponse struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

// JoinGroupRequest is the POST /instances/{id}/groups/join payload: the bare
// invite code or the full invite link.
type JoinGroupRequest struct {
	InviteCode string `json:"invite_code"`
}

// GroupJoinResponse is the answer to a join: the entered group JID.
type GroupJoinResponse struct {
	JID string `json:"jid" binding:"required"`
}

// GroupLeaveResponse is the answer to a leave.
type GroupLeaveResponse struct {
	Left bool `json:"left" binding:"required"`
}

// GroupUpdatedResponse is the answer to a picture or participants update.
type GroupUpdatedResponse struct {
	Updated bool `json:"updated" binding:"required"`
}

// GroupNameValid reports whether name fits the upstream subject limit of 25
// characters.
func GroupNameValid(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && utf8.RuneCountInString(trimmed) <= MaxGroupNameLen
}

// ValidParticipantAction reports whether action is one of the group member
// operations the session accepts.
func ValidParticipantAction(action string) bool {
	switch action {
	case "add", "remove", "promote", "demote":
		return true
	}
	return false
}

// CleanJIDs trims and drops empty addresses, reporting false when any entry
// is blank.
func CleanJIDs(raw []string) ([]string, bool) {
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

// HandleCreateGroup creates a group and answers 201 with its metadata plus
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
// @Param request body CreateGroupRequest true "Group payload"
// @Success 201 {object} core.Envelope{data=representation.GroupEnvelope} "Created, wrapped in the data envelope (invite_code empty when the post-create invite lookup fails; reconcile via GET .../invite, do not retry the create)"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid name or participants"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups [post]
func HandleCreateGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "group-create").Msg("create group request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-create").Err(err).Msg("create group failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request CreateGroupRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		name := strings.TrimSpace(request.Name)
		if !GroupNameValid(name) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name is required, max 25 characters")
			return
		}
		participants, ok := CleanJIDs(request.Participants)
		if !ok {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "participants must be non-empty addresses")
			return
		}

		group, err := instances.CreateGroup(r.Context(), id, instance.CreateGroupInput{Name: name, Participants: participants})
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-create").Err(err).Msg("create group failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		invite, err := instances.GetGroupInvite(r.Context(), id, group.JID)
		response := representation.NewGroupResponse(group)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-create").Err(err).Msg("create group invite failed")
			log.Debug().Str("instance_id", id.String()).Str("op", "group-create").Msg("create group result (partial, no invite)")
			core.JSON(w, r, http.StatusCreated, representation.GroupEnvelope{Group: response})
			return
		}
		response.InviteCode = invite
		log.Debug().Str("instance_id", id.String()).Str("op", "group-create").Msg("create group result")
		core.JSON(w, r, http.StatusCreated, representation.GroupEnvelope{Group: response})
	}
}

// HandleGetGroup answers the live metadata of a group. An unknown group
// answers 404.
//
// @Summary Get a group
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} core.Envelope{data=representation.GroupEnvelope} "Group, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id} [get]
func HandleGetGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-get")
		if !ok {
			return
		}

		group, err := instances.GetGroup(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-get").Err(err).Msg("get group failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, representation.GroupEnvelope{Group: representation.NewGroupResponse(group)})
	}
}

// HandleUpdateGroup applies the subject/topic patch and answers the command
// result (matrix: {updated:true}). At least one field must be present.
//
// @Summary Update a group
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Param request body UpdateGroupRequest true "Group patch"
// @Success 200 {object} core.Envelope{data=GroupUpdatedResponse} "Updated, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Empty patch or invalid values"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id} [patch]
func HandleUpdateGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-update")
		if !ok {
			return
		}

		var request UpdateGroupRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if request.Name == nil && request.Description == nil {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name or description is required")
			return
		}
		input := instance.UpdateGroupInput{Description: request.Description}
		if request.Name != nil {
			name := strings.TrimSpace(*request.Name)
			if !GroupNameValid(name) {
				core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "name is required, max 25 characters")
				return
			}
			input.Name = &name
		}

		if _, err := instances.UpdateGroup(r.Context(), id, groupJID, input); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-update").Err(err).Msg("update group failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupUpdatedResponse{Updated: true})
	}
}

// HandleSetGroupPhoto replaces the group picture with the raw image bytes of
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
// @Success 200 {object} core.Envelope{data=GroupUpdatedResponse} "Updated, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Image exceeds the cap"
// @Failure 422 {object} core.ErrorEnvelope "Missing or non image body"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/photo [put]
func HandleSetGroupPhoto(instances InstanceService, log zerolog.Logger, maxBytes int64) http.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultGroupPhotoBytes
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-photo")
		if !ok {
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "content type must be an image")
			return
		}
		image, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
		if err != nil {
			core.Error(w, r, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
		if len(image) == 0 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "image body is required")
			return
		}
		if int64(len(image)) > maxBytes {
			core.Error(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "image exceeds the size cap")
			return
		}

		if err := instances.SetGroupPhoto(r.Context(), id, groupJID, image); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-photo").Err(err).Msg("set group photo failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupUpdatedResponse{Updated: true})
	}
}

// HandleUpdateGroupParticipants applies the member/admin operation and answers
// 200. Acting without group permission answers 403 and changes nothing.
//
// @Summary Manage group participants
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Param request body UpdateParticipantsRequest true "Participants payload"
// @Success 200 {object} core.Envelope{data=GroupUpdatedResponse} "Applied, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Unknown action or invalid participants"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/participants [post]
func HandleUpdateGroupParticipants(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-participants")
		if !ok {
			return
		}

		var request UpdateParticipantsRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		action := strings.TrimSpace(request.Action)
		if !ValidParticipantAction(action) {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "action must be add, remove, promote or demote")
			return
		}
		participants, ok := CleanJIDs(request.Participants)
		if !ok || len(participants) == 0 {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "participants must be non-empty addresses")
			return
		}

		if err := instances.UpdateGroupParticipants(r.Context(), id, groupJID, action, participants); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-participants").Err(err).Msg("update group participants failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupUpdatedResponse{Updated: true})
	}
}

// HandleGetGroupInvite answers the current invite code without revoking it.
//
// @Summary Get a group invite code
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} core.Envelope{data=GroupInviteResponse} "Invite, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/invite [get]
func HandleGetGroupInvite(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-invite")
		if !ok {
			return
		}

		code, err := instances.GetGroupInvite(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-invite").Err(err).Msg("get group invite failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupInviteResponse{InviteCode: code})
	}
}

// HandleResetGroupInvite revokes the current invite code and answers the fresh
// one.
//
// @Summary Reset a group invite code
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} core.Envelope{data=GroupInviteResponse} "Fresh invite, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner, or no group permission"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/invite/reset [post]
func HandleResetGroupInvite(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-invite-reset")
		if !ok {
			return
		}

		code, err := instances.ResetGroupInvite(r.Context(), id, groupJID)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-invite-reset").Err(err).Msg("reset group invite failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupInviteResponse{InviteCode: code})
	}
}

// HandleJoinGroup enters the group behind the invite code and answers its JID.
// An unknown code answers 404.
//
// @Summary Join a group by invite
// @Tags groups
// @Accept json
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param request body JoinGroupRequest true "Join payload"
// @Success 200 {object} core.Envelope{data=GroupJoinResponse} "Joined group, wrapped in the data envelope"
// @Failure 400 {object} core.ErrorEnvelope "Malformed body"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance not found, or unknown invite"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 413 {object} core.ErrorEnvelope "Body exceeds the 1 MiB limit"
// @Failure 422 {object} core.ErrorEnvelope "Invalid invite code"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/join [post]
func HandleJoinGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := core.InstanceID(w, r)
		if !ok {
			return
		}
		if core.DenyForeignInstanceKey(w, r, id) {
			return
		}
		log.Debug().Str("instance_id", id.String()).Str("op", "group-join").Msg("join group request")

		stored, err := core.LoadInstance(r, instances, id)
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-join").Err(err).Msg("join group failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		if err := core.AuthorizeInstance(r, stored); err != nil {
			core.WriteForbidden(w, r)
			return
		}

		var request JoinGroupRequest
		if err := core.DecodeJSONBody(w, r, &request); err != nil {
			core.WriteJSONBodyError(w, r, err)
			return
		}
		if strings.TrimSpace(request.InviteCode) == "" {
			core.Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "invite_code is required")
			return
		}

		groupJID, err := instances.JoinGroup(r.Context(), id, strings.TrimSpace(request.InviteCode))
		if err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-join").Err(err).Msg("join group failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupJoinResponse{JID: groupJID})
	}
}

// HandleLeaveGroup removes the instance from the group.
//
// @Summary Leave a group
// @Tags groups
// @Produce json
// @Security apikey
// @Param id path string true "Instance UUID or name (exact, case-sensitive)"
// @Param group_id path string true "Group JID"
// @Success 200 {object} core.Envelope{data=GroupLeaveResponse} "Left, wrapped in the data envelope"
// @Failure 401 {object} core.ErrorEnvelope "Missing or invalid credential"
// @Failure 403 {object} core.ErrorEnvelope "Not the owner"
// @Failure 404 {object} core.ErrorEnvelope "Instance or group not found"
// @Failure 409 {object} core.ErrorEnvelope "Instance not connected; instance_name_taken: name already in use"
// @Failure 500 {object} core.ErrorEnvelope "Internal error"
// @Header all {string} X-Request-Id "Correlation id, generated when absent"
// @Router /instances/{id}/groups/{group_id}/leave [post]
func HandleLeaveGroup(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, groupJID, ok := GroupTarget(w, r, instances, log, "group-leave")
		if !ok {
			return
		}

		if err := instances.LeaveGroup(r.Context(), id, groupJID); err != nil {
			log.Warn().Str("instance_id", id.String()).Str("op", "group-leave").Err(err).Msg("leave group failed")
			core.WriteInstanceError(w, r, err)
			return
		}
		core.JSON(w, r, http.StatusOK, GroupLeaveResponse{Left: true})
	}
}

// GroupTarget loads the instance (404) and authorizes (403) for the group
// handlers sharing the {group_id} path, returning the group JID. A blank
// group id answers 404: it names no group.
func GroupTarget(w http.ResponseWriter, r *http.Request, instances InstanceService, log zerolog.Logger, op string) (uuid.UUID, string, bool) {
	id, ok := core.InstanceID(w, r)
	if !ok {
		return uuid.Nil, "", false
	}
	if core.DenyForeignInstanceKey(w, r, id) {
		return uuid.Nil, "", false
	}
	log.Debug().Str("instance_id", id.String()).Str("op", op).Msg("group request")

	stored, err := core.LoadInstance(r, instances, id)
	if err != nil {
		log.Warn().Str("instance_id", id.String()).Str("op", op).Err(err).Msg("group request failed")
		core.WriteInstanceError(w, r, err)
		return uuid.Nil, "", false
	}
	if err := core.AuthorizeInstance(r, stored); err != nil {
		core.WriteForbidden(w, r)
		return uuid.Nil, "", false
	}

	groupJID := strings.TrimSpace(core.PathParam(r, "group_id"))
	if groupJID == "" {
		core.Error(w, r, http.StatusNotFound, "not_found", "group not found")
		return uuid.Nil, "", false
	}
	return id, groupJID, true
}
