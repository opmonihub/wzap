package instance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"wzap/internal/session"
)

var (
	// ErrGroupNotFound reports that the requested group does not exist upstream.
	// The handler maps it to 404.
	ErrGroupNotFound = errors.New("group not found")
	// ErrNewsletterNotFound reports that the requested channel does not exist
	// upstream. The handler maps it to 404.
	ErrNewsletterNotFound = errors.New("newsletter not found")
	// ErrForbidden reports that the instance may not perform the operation on
	// the remote resource (for example a non-admin managing a group). The
	// handler maps it to 403.
	ErrForbidden = errors.New("forbidden")
)

// GroupParticipant is one member of a group: the primary address with its
// admin flags.
type GroupParticipant struct {
	JID          string
	IsAdmin      bool
	IsSuperAdmin bool
}

// Group is the metadata of a group: the live upstream view. UpdatedAt is the
// last metadata refresh (storage write-through); it is zero when no metadata
// store is wired.
type Group struct {
	JID              string
	Name             string
	Description      string
	Participants     []GroupParticipant
	ParticipantCount int
	UpdatedAt        time.Time
}

// CreateGroupInput is the payload accepted by CreateGroup: the subject and
// the initial member addresses.
type CreateGroupInput struct {
	Name         string
	Participants []string
}

// UpdateGroupInput is a partial group update: nil fields keep their upstream
// value. An explicit empty description clears the topic; an empty name is
// rejected before the session.
type UpdateGroupInput struct {
	Name        *string
	Description *string
}

// CreateGroup creates a group with name and the initial participants through
// the instance session. It is synchronous and direct: no outbox, no retry. A
// disconnected session is ErrNotConnected and a malformed target is
// ErrInvalidInput; anything else is returned unchanged for a 500.
func (s *Service) CreateGroup(ctx context.Context, id uuid.UUID, input CreateGroupInput) (Group, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return Group{}, mapError("create group", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return Group{}, fmt.Errorf("create group: create session: %w", err)
	}
	info, err := sess.CreateGroup(ctx, input.Name, input.Participants)
	if err != nil {
		return Group{}, mapGroupError("create group", err)
	}
	return groupFromSession(info), nil
}

// GetGroup returns the live metadata of groupJID through the instance
// session. An unknown group is ErrGroupNotFound. Error mapping follows
// CreateGroup.
func (s *Service) GetGroup(ctx context.Context, id uuid.UUID, groupJID string) (Group, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return Group{}, mapError("get group", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return Group{}, fmt.Errorf("get group: create session: %w", err)
	}
	info, err := sess.GetGroup(ctx, groupJID)
	if err != nil {
		return Group{}, mapGroupError("get group", err)
	}
	return groupFromSession(info), nil
}

// UpdateGroup applies the fields present in input to the upstream group and
// returns its metadata. Error mapping follows CreateGroup.
func (s *Service) UpdateGroup(ctx context.Context, id uuid.UUID, groupJID string, input UpdateGroupInput) (Group, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return Group{}, mapError("update group", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return Group{}, fmt.Errorf("update group: create session: %w", err)
	}
	if input.Name != nil {
		if err := sess.SetGroupName(ctx, groupJID, *input.Name); err != nil {
			return Group{}, mapGroupError("update group", err)
		}
	}
	if input.Description != nil {
		if err := sess.SetGroupDescription(ctx, groupJID, *input.Description); err != nil {
			return Group{}, mapGroupError("update group", err)
		}
	}
	info, err := sess.GetGroup(ctx, groupJID)
	if err != nil {
		return Group{}, mapGroupError("update group", err)
	}
	return groupFromSession(info), nil
}

// SetGroupPhoto replaces the picture of groupJID with the image bytes.
// Error mapping follows CreateGroup.
func (s *Service) SetGroupPhoto(ctx context.Context, id uuid.UUID, groupJID string, image []byte) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("set group photo", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("set group photo: create session: %w", err)
	}
	if err := sess.SetGroupPhoto(ctx, groupJID, image); err != nil {
		return mapGroupError("set group photo", err)
	}
	return nil
}

// UpdateGroupParticipants applies action (add, remove, promote or demote) to
// the participants of groupJID. Acting without group permission is
// ErrForbidden. Other error mapping follows CreateGroup.
func (s *Service) UpdateGroupParticipants(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("update group participants", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("update group participants: create session: %w", err)
	}
	if err := sess.UpdateGroupParticipants(ctx, groupJID, action, participants); err != nil {
		return mapGroupError("update group participants", err)
	}
	return nil
}

// GetGroupInvite returns the current invite code of groupJID without revoking
// it. Error mapping follows CreateGroup.
func (s *Service) GetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", mapError("get group invite", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return "", fmt.Errorf("get group invite: create session: %w", err)
	}
	code, err := sess.GetGroupInvite(ctx, groupJID)
	if err != nil {
		return "", mapGroupError("get group invite", err)
	}
	return code, nil
}

// ResetGroupInvite revokes the current invite code of groupJID and returns the
// fresh one. Error mapping follows CreateGroup.
func (s *Service) ResetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", mapError("reset group invite", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return "", fmt.Errorf("reset group invite: create session: %w", err)
	}
	code, err := sess.ResetGroupInvite(ctx, groupJID)
	if err != nil {
		return "", mapGroupError("reset group invite", err)
	}
	return code, nil
}

// JoinGroup enters the group behind inviteCode and returns the group JID. An
// unknown code is ErrGroupNotFound. Error mapping follows CreateGroup.
func (s *Service) JoinGroup(ctx context.Context, id uuid.UUID, inviteCode string) (string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", mapError("join group", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return "", fmt.Errorf("join group: create session: %w", err)
	}
	groupJID, err := sess.JoinGroup(ctx, inviteCode)
	if err != nil {
		return "", mapGroupError("join group", err)
	}
	return groupJID, nil
}

// LeaveGroup removes the instance from groupJID. Error mapping follows
// CreateGroup.
func (s *Service) LeaveGroup(ctx context.Context, id uuid.UUID, groupJID string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("leave group", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("leave group: create session: %w", err)
	}
	if err := sess.LeaveGroup(ctx, groupJID); err != nil {
		return mapGroupError("leave group", err)
	}
	return nil
}

// groupFromSession translates the session group metadata into the service
// shape.
func groupFromSession(info session.GroupInfo) Group {
	participants := make([]GroupParticipant, 0, len(info.Participants))
	for _, p := range info.Participants {
		participants = append(participants, GroupParticipant{
			JID:          p.JID,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		})
	}
	return Group{
		JID:              info.JID,
		Name:             info.Name,
		Description:      info.Description,
		Participants:     participants,
		ParticipantCount: info.ParticipantCount,
	}
}

// mapGroupError translates a group session call failure into the service
// sentinel the HTTP layer maps to a status code, preserving the operation
// context for the logs. Unknown failures pass through unchanged for a 500
// without leaking their cause (the handler never echoes them).
func mapGroupError(op string, err error) error {
	switch {
	case errors.Is(err, session.ErrNotFound):
		return fmt.Errorf("%s: %w", op, ErrGroupNotFound)
	case errors.Is(err, session.ErrForbidden):
		return fmt.Errorf("%s: %w", op, ErrForbidden)
	default:
		return mapSessionError(op, err)
	}
}
