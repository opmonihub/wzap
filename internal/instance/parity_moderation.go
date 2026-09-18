package instance

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"wzap/internal/session"
)

// GetGroupRequests lists the pending join requests of groupJID. An unknown
// group is ErrGroupNotFound. Error mapping follows CreateGroup.
func (s *Service) GetGroupRequests(ctx context.Context, id uuid.UUID, groupJID string) ([]GroupParticipant, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapError("get group requests", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("get group requests: create session: %w", err)
	}
	members, err := sess.GetGroupRequestParticipants(ctx, groupJID)
	if err != nil {
		return nil, mapGroupError("get group requests", err)
	}
	participants := make([]GroupParticipant, 0, len(members))
	for _, p := range members {
		participants = append(participants, groupParticipantFromSession(p))
	}
	return participants, nil
}

// UpdateGroupRequests approves or rejects participantJIDs of groupJID. An
// action outside approve|decline or an empty participant batch is
// ErrInvalidInput before the session is touched; acting without group
// permission is ErrForbidden. Other error mapping follows CreateGroup.
func (s *Service) UpdateGroupRequests(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error {
	if action != "approve" && action != "decline" {
		return fmt.Errorf("update group requests: %w", ErrInvalidInput)
	}
	if len(participants) == 0 {
		return fmt.Errorf("update group requests: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("update group requests", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("update group requests: create session: %w", err)
	}
	if err := sess.UpdateGroupRequestParticipants(ctx, groupJID, action, participants); err != nil {
		return mapGroupError("update group requests", err)
	}
	return nil
}

// UpdateGroupSettings applies the modes present in the arguments to the
// upstream group and returns its metadata. At least one field must be
// present; joinApproval accepts on|off and memberAddMode accepts
// all_members|admin_only, anything else is ErrInvalidInput before the session
// is touched. Each non-nil field goes through its session setter
// sequentially, then the metadata is re-read like UpdateGroup. Error mapping
// follows CreateGroup.
func (s *Service) UpdateGroupSettings(ctx context.Context, id uuid.UUID, groupJID string, announce, locked *bool, joinApproval, memberAddMode *string) (Group, error) {
	if announce == nil && locked == nil && joinApproval == nil && memberAddMode == nil {
		return Group{}, fmt.Errorf("update group settings: %w", ErrInvalidInput)
	}
	if joinApproval != nil && *joinApproval != "on" && *joinApproval != "off" {
		return Group{}, fmt.Errorf("update group settings: %w", ErrInvalidInput)
	}
	if memberAddMode != nil && *memberAddMode != "all_members" && *memberAddMode != "admin_only" {
		return Group{}, fmt.Errorf("update group settings: %w", ErrInvalidInput)
	}
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return Group{}, mapError("update group settings", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return Group{}, fmt.Errorf("update group settings: create session: %w", err)
	}
	if announce != nil {
		if err := sess.SetGroupAnnounce(ctx, groupJID, *announce); err != nil {
			return Group{}, mapGroupError("update group settings", err)
		}
	}
	if locked != nil {
		if err := sess.SetGroupLocked(ctx, groupJID, *locked); err != nil {
			return Group{}, mapGroupError("update group settings", err)
		}
	}
	if joinApproval != nil {
		if err := sess.SetGroupJoinApprovalMode(ctx, groupJID, *joinApproval); err != nil {
			return Group{}, mapGroupError("update group settings", err)
		}
	}
	if memberAddMode != nil {
		if err := sess.SetGroupMemberAddMode(ctx, groupJID, *memberAddMode); err != nil {
			return Group{}, mapGroupError("update group settings", err)
		}
	}
	info, err := sess.GetGroup(ctx, groupJID)
	if err != nil {
		return Group{}, mapGroupError("update group settings", err)
	}
	return s.refreshGroupCache(ctx, id, groupFromSession(info)), nil
}

// groupParticipantFromSession translates one session group member into the
// service shape.
func groupParticipantFromSession(p session.GroupParticipant) GroupParticipant {
	return GroupParticipant{
		JID:          p.JID,
		IsAdmin:      p.IsAdmin,
		IsSuperAdmin: p.IsSuperAdmin,
	}
}
