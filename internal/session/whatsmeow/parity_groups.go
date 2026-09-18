package whatsmeow

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

// GetJoinedGroups returns the live metadata of every group the instance is
// a member of. An offline client is ErrNotConnected.
func (s *instanceSession) GetJoinedGroups(ctx context.Context) ([]session.GroupInfo, error) {
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: list joined groups", session.ErrNotConnected)
	}
	infos, err := s.client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	out := make([]session.GroupInfo, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		out = append(out, groupInfoFromTypes(info))
	}
	return out, nil
}

// GetGroupInfoFromLink resolves inviteCode (the bare code or the full
// invite link) to the group metadata without joining. An empty code is
// ErrInvalidRecipient; an unknown code is ErrNotFound.
func (s *instanceSession) GetGroupInfoFromLink(ctx context.Context, inviteCode string) (session.GroupInfo, error) {
	code := inviteCodeFromLink(inviteCode)
	if code == "" {
		return session.GroupInfo{}, fmt.Errorf("%w: empty invite code", session.ErrInvalidRecipient)
	}
	if !s.client.IsConnected() {
		return session.GroupInfo{}, fmt.Errorf("%w: group info from link", session.ErrNotConnected)
	}
	info, err := s.client.GetGroupInfoFromLink(ctx, code)
	if err != nil {
		return session.GroupInfo{}, classifyRemoteError(err)
	}
	return groupInfoFromTypes(info), nil
}

// GetGroupRequestParticipants lists the pending join requests of groupJID.
// An unknown group is ErrNotFound.
func (s *instanceSession) GetGroupRequestParticipants(ctx context.Context, groupJID string) ([]session.GroupParticipant, error) {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return nil, err
	}
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: list group join requests", session.ErrNotConnected)
	}
	requests, err := s.client.GetGroupRequestParticipants(ctx, jid)
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	out := make([]session.GroupParticipant, 0, len(requests))
	for _, req := range requests {
		out = append(out, session.GroupParticipant{JID: req.JID.String()})
	}
	return out, nil
}

// UpdateGroupRequestParticipants approves (approve) or rejects (decline)
// participantJIDs of groupJID. An unknown action is ErrInvalidRecipient;
// acting without group permission is ErrForbidden.
func (s *instanceSession) UpdateGroupRequestParticipants(ctx context.Context, groupJID, action string, participantJIDs []string) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	var change whatsmeow.ParticipantRequestChange
	switch action {
	case "approve":
		change = whatsmeow.ParticipantChangeApprove
	case "decline":
		change = whatsmeow.ParticipantChangeReject
	default:
		return fmt.Errorf("%w: unknown request action %q", session.ErrInvalidRecipient, action)
	}
	if len(participantJIDs) == 0 {
		return fmt.Errorf("%w: no participants", session.ErrInvalidRecipient)
	}
	participants := make([]types.JID, 0, len(participantJIDs))
	for _, raw := range participantJIDs {
		parsed, err := types.ParseJID(raw)
		if err != nil || parsed.IsEmpty() {
			return fmt.Errorf("%w: %s", session.ErrInvalidRecipient, raw)
		}
		participants = append(participants, parsed)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: update group join requests", session.ErrNotConnected)
	}
	if _, err := s.client.UpdateGroupRequestParticipants(ctx, jid, participants, change); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// SetGroupAnnounce toggles the announce-only mode of groupJID. Acting
// without group permission is ErrForbidden.
func (s *instanceSession) SetGroupAnnounce(ctx context.Context, groupJID string, announce bool) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set group announce", session.ErrNotConnected)
	}
	if err := s.client.SetGroupAnnounce(ctx, jid, announce); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// SetGroupLocked toggles the locked (info-edit restricted) mode of
// groupJID. Acting without group permission is ErrForbidden.
func (s *instanceSession) SetGroupLocked(ctx context.Context, groupJID string, locked bool) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set group locked", session.ErrNotConnected)
	}
	if err := s.client.SetGroupLocked(ctx, jid, locked); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// SetGroupJoinApprovalMode sets the join-approval mode of groupJID: on
// enables approval, off disables it. An unknown mode is
// ErrInvalidRecipient.
func (s *instanceSession) SetGroupJoinApprovalMode(ctx context.Context, groupJID, mode string) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	var enabled bool
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on":
		enabled = true
	case "off":
		enabled = false
	default:
		return fmt.Errorf("%w: unknown join approval mode %q", session.ErrInvalidRecipient, mode)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set group join approval", session.ErrNotConnected)
	}
	if err := s.client.SetGroupJoinApprovalMode(ctx, jid, enabled); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// SetGroupMemberAddMode sets the member-add mode of groupJID: all_members
// lets every member add, admin_only restricts it to admins. An unknown
// mode is ErrInvalidRecipient.
func (s *instanceSession) SetGroupMemberAddMode(ctx context.Context, groupJID, mode string) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	var upstream types.GroupMemberAddMode
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "all_members":
		upstream = types.GroupMemberAddModeAllMember
	case "admin_only":
		upstream = types.GroupMemberAddModeAdmin
	default:
		return fmt.Errorf("%w: unknown member add mode %q", session.ErrInvalidRecipient, mode)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set group member add mode", session.ErrNotConnected)
	}
	if err := s.client.SetGroupMemberAddMode(ctx, jid, upstream); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}
