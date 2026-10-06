package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"wzap/internal/session"
)

// maxGroupNameLen caps the group subject: longer names are rejected by the
// server with 406, so the handler answers 422 before the session.
const maxGroupNameLen = 25

// maxGroupDescriptionLen caps the group topic accepted by the session.
const maxGroupDescriptionLen = 512

// CreateGroup creates a group with name and the initial participants. An
// empty name or an unparsable participant is ErrInvalidRecipient; an offline
// client is ErrNotConnected.
func (s *instanceSession) CreateGroup(ctx context.Context, name string, participantJIDs []string) (session.GroupInfo, error) {
	if utf8.RuneCountInString(strings.TrimSpace(name)) == 0 || utf8.RuneCountInString(name) > maxGroupNameLen {
		return session.GroupInfo{}, fmt.Errorf("%w: invalid group name", session.ErrInvalidRecipient)
	}
	participants := make([]types.JID, 0, len(participantJIDs))
	for _, raw := range participantJIDs {
		parsed, err := types.ParseJID(raw)
		if err != nil || parsed.IsEmpty() {
			return session.GroupInfo{}, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, raw)
		}
		participants = append(participants, parsed)
	}
	if !s.client.IsConnected() {
		return session.GroupInfo{}, fmt.Errorf("%w: create group", session.ErrNotConnected)
	}
	info, err := s.client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{Name: name, Participants: participants})
	if err != nil {
		return session.GroupInfo{}, classifyRemoteError(err)
	}
	return groupInfoFromTypes(info), nil
}

// GetGroup returns the live metadata of groupJID. An unknown group is
// session.ErrNotFound; a group the instance left is session.ErrForbidden.
func (s *instanceSession) GetGroup(ctx context.Context, groupJID string) (session.GroupInfo, error) {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return session.GroupInfo{}, err
	}
	if !s.client.IsConnected() {
		return session.GroupInfo{}, fmt.Errorf("%w: get group", session.ErrNotConnected)
	}
	info, err := s.client.GetGroupInfo(ctx, jid)
	if err != nil {
		return session.GroupInfo{}, classifyRemoteError(err)
	}
	return groupInfoFromTypes(info), nil
}

// SetGroupName replaces the subject of groupJID.
func (s *instanceSession) SetGroupName(ctx context.Context, groupJID, name string) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(strings.TrimSpace(name)) == 0 || utf8.RuneCountInString(name) > maxGroupNameLen {
		return fmt.Errorf("%w: invalid group name", session.ErrInvalidRecipient)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set group name", session.ErrNotConnected)
	}
	if err := s.client.SetGroupName(ctx, jid, name); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// SetGroupDescription replaces the topic of groupJID, clearing it when
// description is empty.
func (s *instanceSession) SetGroupDescription(ctx context.Context, groupJID, description string) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	if len(description) > maxGroupDescriptionLen {
		return fmt.Errorf("%w: group description too long", session.ErrInvalidRecipient)
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: set group description", session.ErrNotConnected)
	}
	if err := s.client.SetGroupTopic(ctx, jid, "", "", description); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// SetGroupPhoto replaces the picture of groupJID with the image bytes.
func (s *instanceSession) SetGroupPhoto(ctx context.Context, groupJID string, image []byte) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	if len(image) == 0 {
		return fmt.Errorf("%w: empty group photo", session.ErrInvalidRecipient)
	}
	if !s.IsConnected() {
		return fmt.Errorf("%w: set group photo", session.ErrNotConnected)
	}
	setPhoto := s.client.SetGroupPhoto
	if s.setGroupPhotoFn != nil {
		setPhoto = s.setGroupPhotoFn
	}
	if _, err := setPhoto(ctx, jid, image); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// UpdateGroupParticipants applies action (add, remove, promote or demote) to
// the participants of groupJID. An unknown action is ErrInvalidRecipient;
// acting without group permission is ErrForbidden.
func (s *instanceSession) UpdateGroupParticipants(ctx context.Context, groupJID, action string, participantJIDs []string) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	var change whatsmeow.ParticipantChange
	switch action {
	case "add":
		change = whatsmeow.ParticipantChangeAdd
	case "remove":
		change = whatsmeow.ParticipantChangeRemove
	case "promote":
		change = whatsmeow.ParticipantChangePromote
	case "demote":
		change = whatsmeow.ParticipantChangeDemote
	default:
		return fmt.Errorf("%w: unknown participant action %q", session.ErrInvalidRecipient, action)
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
		return fmt.Errorf("%w: update group participants", session.ErrNotConnected)
	}
	if _, err := s.client.UpdateGroupParticipants(ctx, jid, participants, change); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// GetGroupInvite returns the current invite code of groupJID without revoking
// it. The library answers the full link; only the trailing code travels.
func (s *instanceSession) GetGroupInvite(ctx context.Context, groupJID string) (string, error) {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return "", err
	}
	if !s.client.IsConnected() {
		return "", fmt.Errorf("%w: get group invite", session.ErrNotConnected)
	}
	link, err := s.client.GetGroupInviteLink(ctx, jid, false)
	if err != nil {
		return "", classifyRemoteError(err)
	}
	return strings.TrimPrefix(link, whatsmeow.InviteLinkPrefix), nil
}

// ResetGroupInvite revokes the current invite code of groupJID and returns the
// fresh one.
func (s *instanceSession) ResetGroupInvite(ctx context.Context, groupJID string) (string, error) {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return "", err
	}
	if !s.client.IsConnected() {
		return "", fmt.Errorf("%w: reset group invite", session.ErrNotConnected)
	}
	link, err := s.client.GetGroupInviteLink(ctx, jid, true)
	if err != nil {
		return "", classifyRemoteError(err)
	}
	return strings.TrimPrefix(link, whatsmeow.InviteLinkPrefix), nil
}

// JoinGroup enters the group behind inviteCode (the bare code or the full
// invite link) and returns the group JID.
func (s *instanceSession) JoinGroup(ctx context.Context, inviteCode string) (string, error) {
	code := inviteCodeFromLink(inviteCode)
	if code == "" {
		return "", fmt.Errorf("%w: empty invite code", session.ErrInvalidRecipient)
	}
	if !s.client.IsConnected() {
		return "", fmt.Errorf("%w: join group", session.ErrNotConnected)
	}
	jid, err := s.client.JoinGroupWithLink(ctx, code)
	if err != nil {
		return "", classifyRemoteError(err)
	}
	return jid.String(), nil
}

// LeaveGroup removes the instance from groupJID.
func (s *instanceSession) LeaveGroup(ctx context.Context, groupJID string) error {
	jid, err := parseGroupJID(groupJID)
	if err != nil {
		return err
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: leave group", session.ErrNotConnected)
	}
	if err := s.client.LeaveGroup(ctx, jid); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// FollowNewsletter subscribes the instance to channelJID. An unknown channel
// is session.ErrNotFound.
func (s *instanceSession) FollowNewsletter(ctx context.Context, channelJID string) error {
	jid, err := parseChannelJID(channelJID)
	if err != nil {
		return err
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: follow newsletter", session.ErrNotConnected)
	}
	if err := s.client.FollowNewsletter(ctx, jid); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// UnfollowNewsletter ends the subscription of the instance to channelJID.
func (s *instanceSession) UnfollowNewsletter(ctx context.Context, channelJID string) error {
	jid, err := parseChannelJID(channelJID)
	if err != nil {
		return err
	}
	if !s.client.IsConnected() {
		return fmt.Errorf("%w: unfollow newsletter", session.ErrNotConnected)
	}
	if err := s.client.UnfollowNewsletter(ctx, jid); err != nil {
		return classifyRemoteError(err)
	}
	return nil
}

// GetNewsletter returns the live metadata of channelJID.
func (s *instanceSession) GetNewsletter(ctx context.Context, channelJID string) (session.NewsletterInfo, error) {
	jid, err := parseChannelJID(channelJID)
	if err != nil {
		return session.NewsletterInfo{}, err
	}
	if !s.client.IsConnected() {
		return session.NewsletterInfo{}, fmt.Errorf("%w: get newsletter", session.ErrNotConnected)
	}
	meta, err := s.client.GetNewsletterInfo(ctx, jid)
	if err != nil {
		return session.NewsletterInfo{}, classifyRemoteError(err)
	}
	return newsletterFromMeta(meta), nil
}

// ListNewsletters returns the live metadata of every channel the instance
// follows.
func (s *instanceSession) ListNewsletters(ctx context.Context) ([]session.NewsletterInfo, error) {
	if !s.client.IsConnected() {
		return nil, fmt.Errorf("%w: list newsletters", session.ErrNotConnected)
	}
	metas, err := s.client.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, classifyRemoteError(err)
	}
	out := make([]session.NewsletterInfo, 0, len(metas))
	for _, meta := range metas {
		if meta == nil {
			continue
		}
		out = append(out, newsletterFromMeta(meta))
	}
	return out, nil
}

// parseGroupJID parses a group address. ParseJID in the pinned library only
// splits user/server, so an empty result is rejected explicitly like the
// message targets are.
func parseGroupJID(raw string) (types.JID, error) {
	jid, err := types.ParseJID(raw)
	if err != nil || jid.IsEmpty() {
		return types.JID{}, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, raw)
	}
	return jid, nil
}

// parseChannelJID parses a newsletter channel address with the same lenient
// rule as the group targets.
func parseChannelJID(raw string) (types.JID, error) {
	jid, err := types.ParseJID(raw)
	if err != nil || jid.IsEmpty() {
		return types.JID{}, fmt.Errorf("%w: %s", session.ErrInvalidRecipient, raw)
	}
	return jid, nil
}

// inviteCodeFromLink accepts the bare invite code or the full invite link and
// returns the code.
func inviteCodeFromLink(raw string) string {
	code := strings.TrimSpace(raw)
	if i := strings.LastIndex(code, "/"); i >= 0 {
		code = code[i+1:]
	}
	return code
}

// invalidGroupPhotoError reports the upstream rejection when group photo bytes
// are not a decodable image. The match is exact on the error text only.
func invalidGroupPhotoError(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if e.Error() == "not a valid image" {
			return true
		}
	}
	return false
}

// classifyRemoteError translates an upstream group/channel failure into the
// session sentinel the service maps to a status code, without leaking the
// upstream cause. Unknown failures fall through to the shared connectivity
// classifier.
func classifyRemoteError(err error) error {
	if invalidGroupPhotoError(err) {
		return fmt.Errorf("%w: %v", session.ErrInvalidRecipient, err)
	}
	switch {
	case errors.Is(err, whatsmeow.ErrGroupNotFound), errors.Is(err, whatsmeow.ErrIQNotFound):
		return fmt.Errorf("%w: %v", session.ErrNotFound, err)
	case errors.Is(err, whatsmeow.ErrNotInGroup),
		errors.Is(err, whatsmeow.ErrIQForbidden),
		errors.Is(err, whatsmeow.ErrIQNotAuthorized),
		errors.Is(err, whatsmeow.ErrGroupInviteLinkUnauthorized):
		return fmt.Errorf("%w: %v", session.ErrForbidden, err)
	default:
		return classifySessionError(err)
	}
}

// groupInfoFromTypes translates the library group metadata away from the
// library types.
func groupInfoFromTypes(info *types.GroupInfo) session.GroupInfo {
	if info == nil {
		return session.GroupInfo{}
	}
	out := session.GroupInfo{
		JID:              info.JID.String(),
		Name:             info.Name,
		Description:      info.Topic,
		DescriptionID:    info.TopicID,
		ParticipantCount: info.ParticipantCount,
		CreatedAt:        info.GroupCreated,
	}
	for _, p := range info.Participants {
		out.Participants = append(out.Participants, session.GroupParticipant{
			JID:          p.JID.String(),
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		})
	}
	if out.ParticipantCount == 0 {
		out.ParticipantCount = len(out.Participants)
	}
	return out
}

// newsletterFromMeta translates the library channel metadata away from the
// library types.
func newsletterFromMeta(meta *types.NewsletterMetadata) session.NewsletterInfo {
	if meta == nil {
		return session.NewsletterInfo{}
	}
	return session.NewsletterInfo{
		ChannelJID:    meta.ID.String(),
		Title:         meta.ThreadMeta.Name.Text,
		Description:   meta.ThreadMeta.Description.Text,
		FollowerCount: meta.ThreadMeta.SubscriberCount,
	}
}
