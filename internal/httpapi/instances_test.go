package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/httpapi/core"
	"wzap/internal/httpapi/representation"
	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/session"
)

// fakeInstanceService is an in-memory InstanceService: the function fields
// configure each outcome and the recorded fields expose the calls the handlers
// made.
type fakeInstanceService struct {
	createFn                 func(ctx context.Context, input instance.CreateInput) (*model.Instance, string, error)
	oldestAdminFn            func(ctx context.Context) (uuid.UUID, error)
	getByNameFn              func(ctx context.Context, name string) (*model.Instance, error)
	getNames                 []string
	getFn                    func(ctx context.Context, id uuid.UUID) (*model.Instance, error)
	listFn                   func(ctx context.Context) ([]model.Instance, error)
	updateFn                 func(ctx context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error)
	deleteFn                 func(ctx context.Context, id uuid.UUID) error
	disconnectFn             func(ctx context.Context, id uuid.UUID) error
	connectFn                func(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)
	qrFn                     func(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)
	revokeFn                 func(ctx context.Context, id uuid.UUID, chatJID, messageID string) error
	markReadFn               func(ctx context.Context, id uuid.UUID, chatJID, senderJID, messageID string) error
	sendPresenceFn           func(ctx context.Context, id uuid.UUID, chatJID, state string) error
	pairPhoneFn              func(ctx context.Context, id uuid.UUID, phone string) (instance.PairPhoneResult, error)
	createGroupFn            func(ctx context.Context, id uuid.UUID, input instance.CreateGroupInput) (instance.Group, error)
	getGroupFn               func(ctx context.Context, id uuid.UUID, groupJID string) (instance.Group, error)
	updateGroupFn            func(ctx context.Context, id uuid.UUID, groupJID string, input instance.UpdateGroupInput) (instance.Group, error)
	setGroupPhotoFn          func(ctx context.Context, id uuid.UUID, groupJID string, image []byte) error
	updateParticipantsFn     func(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error
	getGroupInviteFn         func(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	resetGroupInviteFn       func(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	joinGroupFn              func(ctx context.Context, id uuid.UUID, inviteCode string) (string, error)
	leaveGroupFn             func(ctx context.Context, id uuid.UUID, groupJID string) error
	followNewsletterFn       func(ctx context.Context, id uuid.UUID, channelJID string) error
	unfollowNewsletterFn     func(ctx context.Context, id uuid.UUID, channelJID string) error
	getNewsletterFn          func(ctx context.Context, id uuid.UUID, channelJID string) (instance.Newsletter, error)
	listNewslettersFn        func(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Newsletter, string, error)
	publishStatusFn          func(ctx context.Context, id uuid.UUID, input session.StatusInput) (string, error)
	listStatusesFn           func(ctx context.Context, id uuid.UUID) ([]session.StatusInfo, error)
	deleteStatusFn           func(ctx context.Context, id uuid.UUID, statusID string) error
	rejectCallFn             func(ctx context.Context, id uuid.UUID, fromJID, callID string) error
	getProfileFn             func(ctx context.Context, id uuid.UUID) (session.Profile, error)
	setProfileNameFn         func(ctx context.Context, id uuid.UUID, name string) error
	setProfileStatusFn       func(ctx context.Context, id uuid.UUID, text string) error
	setProfilePhotoFn        func(ctx context.Context, id uuid.UUID, image []byte) error
	getPrivacyFn             func(ctx context.Context, id uuid.UUID) (session.Privacy, error)
	setPrivacyFn             func(ctx context.Context, id uuid.UUID, input session.Privacy) (session.Privacy, error)
	editMessageFn            func(ctx context.Context, id uuid.UUID, chatJID, messageID, text string) (string, error)
	getJoinedGroupsFn        func(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Group, string, error)
	getGroupInvitePreviewFn  func(ctx context.Context, id uuid.UUID, inviteCode string) (instance.Group, error)
	checkContactsFn          func(ctx context.Context, id uuid.UUID, phones []string) ([]session.ContactCheckResult, error)
	getContactDevicesFn      func(ctx context.Context, id uuid.UUID, jid string) ([]string, error)
	getContactPhotoFn        func(ctx context.Context, id uuid.UUID, jid string) (session.ProfilePictureInfo, error)
	getContactBusinessFn     func(ctx context.Context, id uuid.UUID, jid string) (session.BusinessProfile, error)
	getBlocklistFn           func(ctx context.Context, id uuid.UUID) ([]string, error)
	getStatusPrivacyFn       func(ctx context.Context, id uuid.UUID) (session.StatusPrivacy, error)
	getDisappearingTimerFn   func(ctx context.Context, id uuid.UUID, chatJID string) (time.Duration, bool, error)
	getNewsletterMessagesFn  func(ctx context.Context, id uuid.UUID, channel, cursor string, limit int) ([]session.NewsletterMessage, string, error)
	getNewsletterUpdatesFn   func(ctx context.Context, id uuid.UUID, channel string) ([]session.NewsletterMessage, error)
	getGroupRequestsFn       func(ctx context.Context, id uuid.UUID, groupJID string) ([]instance.GroupParticipant, error)
	updateGroupRequestsFn    func(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error
	updateGroupSettingsFn    func(ctx context.Context, id uuid.UUID, groupJID string, announce, locked *bool, joinApproval, memberAddMode *string) (instance.Group, error)
	updateBlocklistFn        func(ctx context.Context, id uuid.UUID, jid, action string) error
	setDisappearingTimerFn   func(ctx context.Context, id uuid.UUID, chatJID string, duration time.Duration) error
	setDefaultDisappearingFn func(ctx context.Context, id uuid.UUID, duration time.Duration) error
	subscribePresenceFn      func(ctx context.Context, id uuid.UUID, jid string) error
	getContactQRLinkFn       func(ctx context.Context, id uuid.UUID, revoke bool) (string, error)
	createNewsletterFn       func(ctx context.Context, id uuid.UUID, title, description string) (instance.Newsletter, error)
	muteNewsletterFn         func(ctx context.Context, id uuid.UUID, channel string, muted bool) error
	markNewsletterViewedFn   func(ctx context.Context, id uuid.UUID, channel string, serverIDs []string) error
	reactNewsletterFn        func(ctx context.Context, id uuid.UUID, channel, serverID, reaction string) error

	createInputs                []instance.CreateInput
	updateInputs                []instance.UpdateInput
	getIDs                      []uuid.UUID
	deleteIDs                   []uuid.UUID
	disconnectIDs               []uuid.UUID
	connectIDs                  []uuid.UUID
	qrIDs                       []uuid.UUID
	revokeCalls                 []revokeCall
	markReadCalls               []markReadCall
	presenceCalls               []presenceCall
	pairPhoneCalls              []pairPhoneCall
	createGroupCalls            []createGroupCall
	getGroupCalls               []groupTargetCall
	updateGroupCalls            []updateGroupCall
	setGroupPhotoCalls          []setGroupPhotoCall
	updateParticipantsCalls     []updateParticipantsCall
	getGroupInviteCalls         []groupTargetCall
	resetGroupInviteCalls       []groupTargetCall
	joinGroupCalls              []joinGroupCall
	leaveGroupCalls             []groupTargetCall
	followNewsletterCalls       []newsletterTargetCall
	unfollowNewsletterCalls     []newsletterTargetCall
	getNewsletterCalls          []newsletterTargetCall
	listNewsletterCalls         []listNewsletterCall
	publishStatusCalls          []publishStatusCall
	listStatusCalls             []uuid.UUID
	deleteStatusCalls           []deleteStatusCall
	rejectCallCalls             []rejectCallRecord
	setProfileNameCalls         []profileNameCall
	setProfileStatusCalls       []profileStatusCall
	setProfilePhotoCalls        []profilePhotoCall
	setPrivacyCalls             []privacyCall
	editMessageCalls            []editMessageCall
	getJoinedGroupsCalls        []listGroupsCall
	getGroupInvitePreviewCalls  []invitePreviewCall
	checkContactsCalls          []checkContactsCall
	getContactDevicesCalls      []contactTargetCall
	getContactPhotoCalls        []contactTargetCall
	getContactBusinessCalls     []contactTargetCall
	getBlocklistCalls           []uuid.UUID
	getStatusPrivacyCalls       []uuid.UUID
	getDisappearingTimerCalls   []disappearingCall
	getNewsletterMessagesCalls  []newsletterMessagesCall
	getNewsletterUpdatesCalls   []newsletterTargetCall
	getGroupRequestsCalls       []groupTargetCall
	updateGroupRequestsCalls    []updateParticipantsCall
	updateGroupSettingsCalls    []updateGroupSettingsCall
	updateBlocklistCalls        []blocklistCall
	setDisappearingTimerCalls   []disappearingSetCall
	setDefaultDisappearingCalls []uuid.UUID
	subscribePresenceCalls      []contactTargetCall
	getContactQRLinkCalls       []contactLinkCall
	createNewsletterCalls       []createNewsletterCall
	muteNewsletterCalls         []muteNewsletterCall
	markNewsletterViewedCalls   []markNewsletterViewedCall
	reactNewsletterCalls        []reactNewsletterCall
	listCalls                   int
}

// revokeCall records one RevokeMessage call received by the fake.
type revokeCall struct {
	InstanceID uuid.UUID
	ChatJID    string
	MessageID  string
}

// markReadCall records one MarkRead call received by the fake.
type markReadCall struct {
	InstanceID uuid.UUID
	ChatJID    string
	SenderJID  string
	MessageID  string
}

// presenceCall records one SendPresence call received by the fake.
type presenceCall struct {
	InstanceID uuid.UUID
	ChatJID    string
	State      string
}

// pairPhoneCall records one PairPhone call received by the fake.
type pairPhoneCall struct {
	InstanceID uuid.UUID
	Phone      string
}

// createGroupCall records one CreateGroup call received by the fake.
type createGroupCall struct {
	InstanceID uuid.UUID
	Input      instance.CreateGroupInput
}

// groupTargetCall records one group call addressing a group JID.
type groupTargetCall struct {
	InstanceID uuid.UUID
	GroupJID   string
}

// updateGroupCall records one UpdateGroup call received by the fake.
type updateGroupCall struct {
	InstanceID uuid.UUID
	GroupJID   string
	Input      instance.UpdateGroupInput
}

// setGroupPhotoCall records one SetGroupPhoto call received by the fake.
type setGroupPhotoCall struct {
	InstanceID uuid.UUID
	GroupJID   string
	Image      []byte
}

// updateParticipantsCall records one UpdateGroupParticipants call received by
// the fake.
type updateParticipantsCall struct {
	InstanceID   uuid.UUID
	GroupJID     string
	Action       string
	Participants []string
}

// joinGroupCall records one JoinGroup call received by the fake.
type joinGroupCall struct {
	InstanceID uuid.UUID
	InviteCode string
}

// newsletterTargetCall records one newsletter call addressing a channel.
type newsletterTargetCall struct {
	InstanceID uuid.UUID
	ChannelJID string
}

// listNewsletterCall records one ListNewsletters call received by the fake.
type listNewsletterCall struct {
	InstanceID uuid.UUID
	Limit      int
	Cursor     string
}

// publishStatusCall records one PublishStatus call received by the fake.
type publishStatusCall struct {
	InstanceID uuid.UUID
	Input      session.StatusInput
}

// deleteStatusCall records one DeleteStatus call received by the fake.
type deleteStatusCall struct {
	InstanceID uuid.UUID
	StatusID   string
}

// rejectCallRecord records one RejectCall call received by the fake.
type rejectCallRecord struct {
	InstanceID uuid.UUID
	FromJID    string
	CallID     string
}

// profileNameCall records one SetProfileName call received by the fake.
type profileNameCall struct {
	InstanceID uuid.UUID
	Name       string
}

// profileStatusCall records one SetProfileStatusText call received by the
// fake.
type profileStatusCall struct {
	InstanceID uuid.UUID
	Text       string
}

// profilePhotoCall records one SetProfilePhoto call received by the fake.
type profilePhotoCall struct {
	InstanceID uuid.UUID
	Image      []byte
}

// privacyCall records one SetPrivacy call received by the fake.
type privacyCall struct {
	InstanceID uuid.UUID
	Input      session.Privacy
}

// editMessageCall records one EditMessage call received by the fake.
type editMessageCall struct {
	InstanceID uuid.UUID
	ChatJID    string
	MessageID  string
	Text       string
}

// listGroupsCall records one GetJoinedGroups call received by the fake.
type listGroupsCall struct {
	InstanceID uuid.UUID
	Limit      int
	Cursor     string
}

// invitePreviewCall records one GetGroupInvitePreview call received by the
// fake.
type invitePreviewCall struct {
	InstanceID uuid.UUID
	InviteCode string
}

// checkContactsCall records one CheckContacts call received by the fake.
type checkContactsCall struct {
	InstanceID uuid.UUID
	Phones     []string
}

// contactTargetCall records one contact-directory call addressing a JID.
type contactTargetCall struct {
	InstanceID uuid.UUID
	JID        string
}

// disappearingCall records one GetDisappearingTimer call received by the fake.
type disappearingCall struct {
	InstanceID uuid.UUID
	ChatJID    string
}

// newsletterMessagesCall records one GetNewsletterMessages call received by
// the fake.
type newsletterMessagesCall struct {
	InstanceID uuid.UUID
	Channel    string
	Cursor     string
	Limit      int
}

// updateGroupSettingsCall records one UpdateGroupSettings call received by
// the fake.
type updateGroupSettingsCall struct {
	InstanceID    uuid.UUID
	GroupJID      string
	Announce      *bool
	Locked        *bool
	JoinApproval  *string
	MemberAddMode *string
}

// blocklistCall records one UpdateBlocklist call received by the fake.
type blocklistCall struct {
	InstanceID uuid.UUID
	JID        string
	Action     string
}

// disappearingSetCall records one SetDisappearingTimer call received by the
// fake.
type disappearingSetCall struct {
	InstanceID uuid.UUID
	ChatJID    string
	Duration   time.Duration
}

// contactLinkCall records one GetContactQRLink call received by the fake.
type contactLinkCall struct {
	InstanceID uuid.UUID
	Revoke     bool
}

// createNewsletterCall records one CreateNewsletter call received by the fake.
type createNewsletterCall struct {
	InstanceID  uuid.UUID
	Title       string
	Description string
}

// muteNewsletterCall records one MuteNewsletter call received by the fake.
type muteNewsletterCall struct {
	InstanceID uuid.UUID
	ChannelJID string
	Muted      bool
}

// markNewsletterViewedCall records one MarkNewsletterViewed call received by
// the fake.
type markNewsletterViewedCall struct {
	InstanceID uuid.UUID
	ChannelJID string
	ServerIDs  []string
}

// reactNewsletterCall records one ReactNewsletter call received by the fake.
type reactNewsletterCall struct {
	InstanceID uuid.UUID
	ChannelJID string
	ServerID   string
	Reaction   string
}

// Create records the input and returns the configured instance with its
// one-time key, defaulting to a fresh disconnected instance with an empty key.
func (f *fakeInstanceService) Create(ctx context.Context, input instance.CreateInput) (*model.Instance, string, error) {
	f.createInputs = append(f.createInputs, input)
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return &model.Instance{ID: uuid.New(), Name: input.Name, ExternalRef: input.ExternalRef, OwnerUserID: input.OwnerUserID, Connection: model.InstanceConnection{Status: "disconnected"}}, "", nil
}

// OldestAdmin returns the configured oldest admin, defaulting to a fresh id.
func (f *fakeInstanceService) OldestAdmin(ctx context.Context) (uuid.UUID, error) {
	if f.oldestAdminFn != nil {
		return f.oldestAdminFn(ctx)
	}
	return uuid.New(), nil
}

// Get records the id and returns the configured instance, defaulting to a
// stored disconnected instance with the requested id so global-scope tests
// exercise the operation behind the ownership gate.
func (f *fakeInstanceService) Get(ctx context.Context, id uuid.UUID) (*model.Instance, error) {
	f.getIDs = append(f.getIDs, id)
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return &model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: "disconnected"}}, nil
}
func (f *fakeInstanceService) GetByName(ctx context.Context, name string) (*model.Instance, error) {
	f.getNames = append(f.getNames, name)
	if f.getByNameFn != nil {
		return f.getByNameFn(ctx, name)
	}
	return nil, instance.ErrNotFound
}

// List returns the configured collection, defaulting to an empty one.
func (f *fakeInstanceService) List(ctx context.Context) ([]model.Instance, error) {
	f.listCalls++
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return nil, nil
}

// Update records the input and returns the configured instance.
func (f *fakeInstanceService) Update(ctx context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error) {
	f.updateInputs = append(f.updateInputs, input)
	if f.updateFn != nil {
		return f.updateFn(ctx, id, input)
	}
	return &model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: "disconnected"}}, nil
}

// Delete records the id and returns the configured error.
func (f *fakeInstanceService) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleteIDs = append(f.deleteIDs, id)
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

// Disconnect records the id and returns the configured error.
func (f *fakeInstanceService) Disconnect(ctx context.Context, id uuid.UUID) error {
	f.disconnectIDs = append(f.disconnectIDs, id)
	if f.disconnectFn != nil {
		return f.disconnectFn(ctx, id)
	}
	return nil
}

// Connect records the id and returns the configured result, defaulting to a
// fresh pairing result.
func (f *fakeInstanceService) Connect(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error) {
	f.connectIDs = append(f.connectIDs, id)
	if f.connectFn != nil {
		return f.connectFn(ctx, id)
	}
	expiresAt := time.Now().Add(time.Minute)
	return instance.ConnectResult{Status: "pairing", QRCode: "qr-code", QRExpiresAt: &expiresAt}, nil
}

// QR records the id and returns the configured result, defaulting to the same
// pairing result as Connect.
func (f *fakeInstanceService) QR(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error) {
	f.qrIDs = append(f.qrIDs, id)
	if f.qrFn != nil {
		return f.qrFn(ctx, id)
	}
	expiresAt := time.Now().Add(time.Minute)
	return instance.ConnectResult{Status: "pairing", QRCode: "qr-code", QRExpiresAt: &expiresAt}, nil
}

// RevokeMessage records the call and returns the configured error.
func (f *fakeInstanceService) RevokeMessage(ctx context.Context, id uuid.UUID, chatJID, messageID string) error {
	f.revokeCalls = append(f.revokeCalls, revokeCall{InstanceID: id, ChatJID: chatJID, MessageID: messageID})
	if f.revokeFn != nil {
		return f.revokeFn(ctx, id, chatJID, messageID)
	}
	return nil
}

// MarkRead records the call and returns the configured error.
func (f *fakeInstanceService) MarkRead(ctx context.Context, id uuid.UUID, chatJID, senderJID, messageID string) error {
	f.markReadCalls = append(f.markReadCalls, markReadCall{InstanceID: id, ChatJID: chatJID, SenderJID: senderJID, MessageID: messageID})
	if f.markReadFn != nil {
		return f.markReadFn(ctx, id, chatJID, senderJID, messageID)
	}
	return nil
}

// SendPresence records the call and returns the configured error.
func (f *fakeInstanceService) SendPresence(ctx context.Context, id uuid.UUID, chatJID, state string) error {
	f.presenceCalls = append(f.presenceCalls, presenceCall{InstanceID: id, ChatJID: chatJID, State: state})
	if f.sendPresenceFn != nil {
		return f.sendPresenceFn(ctx, id, chatJID, state)
	}
	return nil
}

// PairPhone records the call and returns the configured result.
func (f *fakeInstanceService) PairPhone(ctx context.Context, id uuid.UUID, phone string) (instance.PairPhoneResult, error) {
	f.pairPhoneCalls = append(f.pairPhoneCalls, pairPhoneCall{InstanceID: id, Phone: phone})
	if f.pairPhoneFn != nil {
		return f.pairPhoneFn(ctx, id, phone)
	}
	return instance.PairPhoneResult{Code: "12345678", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

// CreateGroup records the call and returns the configured group.
func (f *fakeInstanceService) CreateGroup(ctx context.Context, id uuid.UUID, input instance.CreateGroupInput) (instance.Group, error) {
	f.createGroupCalls = append(f.createGroupCalls, createGroupCall{InstanceID: id, Input: input})
	if f.createGroupFn != nil {
		return f.createGroupFn(ctx, id, input)
	}
	return instance.Group{JID: "120363000000000000@g.us", Name: input.Name}, nil
}

// GetGroup records the call and returns the configured group.
func (f *fakeInstanceService) GetGroup(ctx context.Context, id uuid.UUID, groupJID string) (instance.Group, error) {
	f.getGroupCalls = append(f.getGroupCalls, groupTargetCall{InstanceID: id, GroupJID: groupJID})
	if f.getGroupFn != nil {
		return f.getGroupFn(ctx, id, groupJID)
	}
	return instance.Group{JID: groupJID}, nil
}

// UpdateGroup records the call and returns the configured group.
func (f *fakeInstanceService) UpdateGroup(ctx context.Context, id uuid.UUID, groupJID string, input instance.UpdateGroupInput) (instance.Group, error) {
	f.updateGroupCalls = append(f.updateGroupCalls, updateGroupCall{InstanceID: id, GroupJID: groupJID, Input: input})
	if f.updateGroupFn != nil {
		return f.updateGroupFn(ctx, id, groupJID, input)
	}
	return instance.Group{JID: groupJID}, nil
}

// SetGroupPhoto records the call and returns the configured error.
func (f *fakeInstanceService) SetGroupPhoto(ctx context.Context, id uuid.UUID, groupJID string, image []byte) error {
	f.setGroupPhotoCalls = append(f.setGroupPhotoCalls, setGroupPhotoCall{InstanceID: id, GroupJID: groupJID, Image: image})
	if f.setGroupPhotoFn != nil {
		return f.setGroupPhotoFn(ctx, id, groupJID, image)
	}
	return nil
}

// UpdateGroupParticipants records the call and returns the configured error.
func (f *fakeInstanceService) UpdateGroupParticipants(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error {
	f.updateParticipantsCalls = append(f.updateParticipantsCalls, updateParticipantsCall{InstanceID: id, GroupJID: groupJID, Action: action, Participants: participants})
	if f.updateParticipantsFn != nil {
		return f.updateParticipantsFn(ctx, id, groupJID, action, participants)
	}
	return nil
}

// GetGroupInvite records the call and returns the configured code.
func (f *fakeInstanceService) GetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error) {
	f.getGroupInviteCalls = append(f.getGroupInviteCalls, groupTargetCall{InstanceID: id, GroupJID: groupJID})
	if f.getGroupInviteFn != nil {
		return f.getGroupInviteFn(ctx, id, groupJID)
	}
	return "invite-code-1", nil
}

// ResetGroupInvite records the call and returns the configured code.
func (f *fakeInstanceService) ResetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error) {
	f.resetGroupInviteCalls = append(f.resetGroupInviteCalls, groupTargetCall{InstanceID: id, GroupJID: groupJID})
	if f.resetGroupInviteFn != nil {
		return f.resetGroupInviteFn(ctx, id, groupJID)
	}
	return "invite-code-2", nil
}

// JoinGroup records the call and returns the configured group JID.
func (f *fakeInstanceService) JoinGroup(ctx context.Context, id uuid.UUID, inviteCode string) (string, error) {
	f.joinGroupCalls = append(f.joinGroupCalls, joinGroupCall{InstanceID: id, InviteCode: inviteCode})
	if f.joinGroupFn != nil {
		return f.joinGroupFn(ctx, id, inviteCode)
	}
	return "120363000000000000@g.us", nil
}

// LeaveGroup records the call and returns the configured error.
func (f *fakeInstanceService) LeaveGroup(ctx context.Context, id uuid.UUID, groupJID string) error {
	f.leaveGroupCalls = append(f.leaveGroupCalls, groupTargetCall{InstanceID: id, GroupJID: groupJID})
	if f.leaveGroupFn != nil {
		return f.leaveGroupFn(ctx, id, groupJID)
	}
	return nil
}

// FollowNewsletter records the call and returns the configured error.
func (f *fakeInstanceService) FollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error {
	f.followNewsletterCalls = append(f.followNewsletterCalls, newsletterTargetCall{InstanceID: id, ChannelJID: channelJID})
	if f.followNewsletterFn != nil {
		return f.followNewsletterFn(ctx, id, channelJID)
	}
	return nil
}

// UnfollowNewsletter records the call and returns the configured error.
func (f *fakeInstanceService) UnfollowNewsletter(ctx context.Context, id uuid.UUID, channelJID string) error {
	f.unfollowNewsletterCalls = append(f.unfollowNewsletterCalls, newsletterTargetCall{InstanceID: id, ChannelJID: channelJID})
	if f.unfollowNewsletterFn != nil {
		return f.unfollowNewsletterFn(ctx, id, channelJID)
	}
	return nil
}

// GetNewsletter records the call and returns the configured channel.
func (f *fakeInstanceService) GetNewsletter(ctx context.Context, id uuid.UUID, channelJID string) (instance.Newsletter, error) {
	f.getNewsletterCalls = append(f.getNewsletterCalls, newsletterTargetCall{InstanceID: id, ChannelJID: channelJID})
	if f.getNewsletterFn != nil {
		return f.getNewsletterFn(ctx, id, channelJID)
	}
	return instance.Newsletter{ChannelJID: channelJID}, nil
}

// ListNewsletters records the call and returns the configured page.
func (f *fakeInstanceService) ListNewsletters(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Newsletter, string, error) {
	f.listNewsletterCalls = append(f.listNewsletterCalls, listNewsletterCall{InstanceID: id, Limit: limit, Cursor: cursor})
	if f.listNewslettersFn != nil {
		return f.listNewslettersFn(ctx, id, limit, cursor)
	}
	return nil, "", nil
}

// PublishStatus records the call and returns the configured upstream id.
func (f *fakeInstanceService) PublishStatus(ctx context.Context, id uuid.UUID, input session.StatusInput) (string, error) {
	f.publishStatusCalls = append(f.publishStatusCalls, publishStatusCall{InstanceID: id, Input: input})
	if f.publishStatusFn != nil {
		return f.publishStatusFn(ctx, id, input)
	}
	return "wamid.status", nil
}

// ListStatuses records the call and returns the configured statuses.
func (f *fakeInstanceService) ListStatuses(ctx context.Context, id uuid.UUID) ([]session.StatusInfo, error) {
	f.listStatusCalls = append(f.listStatusCalls, id)
	if f.listStatusesFn != nil {
		return f.listStatusesFn(ctx, id)
	}
	return nil, nil
}

// DeleteStatus records the call and returns the configured error.
func (f *fakeInstanceService) DeleteStatus(ctx context.Context, id uuid.UUID, statusID string) error {
	f.deleteStatusCalls = append(f.deleteStatusCalls, deleteStatusCall{InstanceID: id, StatusID: statusID})
	if f.deleteStatusFn != nil {
		return f.deleteStatusFn(ctx, id, statusID)
	}
	return nil
}

// RejectCall records the call and returns the configured error.
func (f *fakeInstanceService) RejectCall(ctx context.Context, id uuid.UUID, fromJID, callID string) error {
	f.rejectCallCalls = append(f.rejectCallCalls, rejectCallRecord{InstanceID: id, FromJID: fromJID, CallID: callID})
	if f.rejectCallFn != nil {
		return f.rejectCallFn(ctx, id, fromJID, callID)
	}
	return nil
}

// GetProfile records the call and returns the configured profile.
func (f *fakeInstanceService) GetProfile(ctx context.Context, id uuid.UUID) (session.Profile, error) {
	if f.getProfileFn != nil {
		return f.getProfileFn(ctx, id)
	}
	return session.Profile{}, nil
}

// SetProfileName records the call and returns the configured error.
func (f *fakeInstanceService) SetProfileName(ctx context.Context, id uuid.UUID, name string) error {
	f.setProfileNameCalls = append(f.setProfileNameCalls, profileNameCall{InstanceID: id, Name: name})
	if f.setProfileNameFn != nil {
		return f.setProfileNameFn(ctx, id, name)
	}
	return nil
}

// SetProfileStatusText records the call and returns the configured error.
func (f *fakeInstanceService) SetProfileStatusText(ctx context.Context, id uuid.UUID, text string) error {
	f.setProfileStatusCalls = append(f.setProfileStatusCalls, profileStatusCall{InstanceID: id, Text: text})
	if f.setProfileStatusFn != nil {
		return f.setProfileStatusFn(ctx, id, text)
	}
	return nil
}

// SetProfilePhoto records the call and returns the configured error.
func (f *fakeInstanceService) SetProfilePhoto(ctx context.Context, id uuid.UUID, image []byte) error {
	f.setProfilePhotoCalls = append(f.setProfilePhotoCalls, profilePhotoCall{InstanceID: id, Image: image})
	if f.setProfilePhotoFn != nil {
		return f.setProfilePhotoFn(ctx, id, image)
	}
	return nil
}

// GetPrivacy records the call and returns the configured settings.
func (f *fakeInstanceService) GetPrivacy(ctx context.Context, id uuid.UUID) (session.Privacy, error) {
	if f.getPrivacyFn != nil {
		return f.getPrivacyFn(ctx, id)
	}
	return session.Privacy{}, nil
}

// SetPrivacy records the call and returns the configured settings.
func (f *fakeInstanceService) SetPrivacy(ctx context.Context, id uuid.UUID, input session.Privacy) (session.Privacy, error) {
	f.setPrivacyCalls = append(f.setPrivacyCalls, privacyCall{InstanceID: id, Input: input})
	if f.setPrivacyFn != nil {
		return f.setPrivacyFn(ctx, id, input)
	}
	return session.Privacy{}, nil
}

// EditMessage records the call and returns the configured upstream id.
func (f *fakeInstanceService) EditMessage(ctx context.Context, id uuid.UUID, chatJID, messageID, text string) (string, error) {
	f.editMessageCalls = append(f.editMessageCalls, editMessageCall{InstanceID: id, ChatJID: chatJID, MessageID: messageID, Text: text})
	if f.editMessageFn != nil {
		return f.editMessageFn(ctx, id, chatJID, messageID, text)
	}
	return "WAID-NEW-1", nil
}

// GetJoinedGroups records the call and returns the configured page.
func (f *fakeInstanceService) GetJoinedGroups(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Group, string, error) {
	f.getJoinedGroupsCalls = append(f.getJoinedGroupsCalls, listGroupsCall{InstanceID: id, Limit: limit, Cursor: cursor})
	if f.getJoinedGroupsFn != nil {
		return f.getJoinedGroupsFn(ctx, id, limit, cursor)
	}
	return nil, "", nil
}

// GetGroupInvitePreview records the call and returns the configured preview.
func (f *fakeInstanceService) GetGroupInvitePreview(ctx context.Context, id uuid.UUID, inviteCode string) (instance.Group, error) {
	f.getGroupInvitePreviewCalls = append(f.getGroupInvitePreviewCalls, invitePreviewCall{InstanceID: id, InviteCode: inviteCode})
	if f.getGroupInvitePreviewFn != nil {
		return f.getGroupInvitePreviewFn(ctx, id, inviteCode)
	}
	return instance.Group{}, nil
}

// CheckContacts records the call and returns the configured results.
func (f *fakeInstanceService) CheckContacts(ctx context.Context, id uuid.UUID, phones []string) ([]session.ContactCheckResult, error) {
	f.checkContactsCalls = append(f.checkContactsCalls, checkContactsCall{InstanceID: id, Phones: phones})
	if f.checkContactsFn != nil {
		return f.checkContactsFn(ctx, id, phones)
	}
	return nil, nil
}

// GetContactDevices records the call and returns the configured devices.
func (f *fakeInstanceService) GetContactDevices(ctx context.Context, id uuid.UUID, jid string) ([]string, error) {
	f.getContactDevicesCalls = append(f.getContactDevicesCalls, contactTargetCall{InstanceID: id, JID: jid})
	if f.getContactDevicesFn != nil {
		return f.getContactDevicesFn(ctx, id, jid)
	}
	return nil, nil
}

// GetContactPhoto records the call and returns the configured picture.
func (f *fakeInstanceService) GetContactPhoto(ctx context.Context, id uuid.UUID, jid string) (session.ProfilePictureInfo, error) {
	f.getContactPhotoCalls = append(f.getContactPhotoCalls, contactTargetCall{InstanceID: id, JID: jid})
	if f.getContactPhotoFn != nil {
		return f.getContactPhotoFn(ctx, id, jid)
	}
	return session.ProfilePictureInfo{}, nil
}

// GetContactBusiness records the call and returns the configured profile.
func (f *fakeInstanceService) GetContactBusiness(ctx context.Context, id uuid.UUID, jid string) (session.BusinessProfile, error) {
	f.getContactBusinessCalls = append(f.getContactBusinessCalls, contactTargetCall{InstanceID: id, JID: jid})
	if f.getContactBusinessFn != nil {
		return f.getContactBusinessFn(ctx, id, jid)
	}
	return session.BusinessProfile{}, nil
}

// GetBlocklist records the call and returns the configured JIDs.
func (f *fakeInstanceService) GetBlocklist(ctx context.Context, id uuid.UUID) ([]string, error) {
	f.getBlocklistCalls = append(f.getBlocklistCalls, id)
	if f.getBlocklistFn != nil {
		return f.getBlocklistFn(ctx, id)
	}
	return nil, nil
}

// GetStatusPrivacy records the call and returns the configured audience.
func (f *fakeInstanceService) GetStatusPrivacy(ctx context.Context, id uuid.UUID) (session.StatusPrivacy, error) {
	f.getStatusPrivacyCalls = append(f.getStatusPrivacyCalls, id)
	if f.getStatusPrivacyFn != nil {
		return f.getStatusPrivacyFn(ctx, id)
	}
	return session.StatusPrivacy{}, nil
}

// GetDisappearingTimer records the call and returns the configured timer.
func (f *fakeInstanceService) GetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string) (time.Duration, bool, error) {
	f.getDisappearingTimerCalls = append(f.getDisappearingTimerCalls, disappearingCall{InstanceID: id, ChatJID: chatJID})
	if f.getDisappearingTimerFn != nil {
		return f.getDisappearingTimerFn(ctx, id, chatJID)
	}
	return 0, false, nil
}

// GetNewsletterMessages records the call and returns the configured page.
func (f *fakeInstanceService) GetNewsletterMessages(ctx context.Context, id uuid.UUID, channel, cursor string, limit int) ([]session.NewsletterMessage, string, error) {
	f.getNewsletterMessagesCalls = append(f.getNewsletterMessagesCalls, newsletterMessagesCall{InstanceID: id, Channel: channel, Cursor: cursor, Limit: limit})
	if f.getNewsletterMessagesFn != nil {
		return f.getNewsletterMessagesFn(ctx, id, channel, cursor, limit)
	}
	return nil, "", nil
}

// GetNewsletterUpdates records the call and returns the configured updates.
func (f *fakeInstanceService) GetNewsletterUpdates(ctx context.Context, id uuid.UUID, channel string) ([]session.NewsletterMessage, error) {
	f.getNewsletterUpdatesCalls = append(f.getNewsletterUpdatesCalls, newsletterTargetCall{InstanceID: id, ChannelJID: channel})
	if f.getNewsletterUpdatesFn != nil {
		return f.getNewsletterUpdatesFn(ctx, id, channel)
	}
	return nil, nil
}

// GetGroupRequests records the call and returns the configured pending join
// requests.
func (f *fakeInstanceService) GetGroupRequests(ctx context.Context, id uuid.UUID, groupJID string) ([]instance.GroupParticipant, error) {
	f.getGroupRequestsCalls = append(f.getGroupRequestsCalls, groupTargetCall{InstanceID: id, GroupJID: groupJID})
	if f.getGroupRequestsFn != nil {
		return f.getGroupRequestsFn(ctx, id, groupJID)
	}
	return nil, nil
}

// UpdateGroupRequests records the call and returns the configured error.
func (f *fakeInstanceService) UpdateGroupRequests(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error {
	f.updateGroupRequestsCalls = append(f.updateGroupRequestsCalls, updateParticipantsCall{InstanceID: id, GroupJID: groupJID, Action: action, Participants: participants})
	if f.updateGroupRequestsFn != nil {
		return f.updateGroupRequestsFn(ctx, id, groupJID, action, participants)
	}
	return nil
}

// UpdateGroupSettings records the call and returns the configured group.
func (f *fakeInstanceService) UpdateGroupSettings(ctx context.Context, id uuid.UUID, groupJID string, announce, locked *bool, joinApproval, memberAddMode *string) (instance.Group, error) {
	f.updateGroupSettingsCalls = append(f.updateGroupSettingsCalls, updateGroupSettingsCall{InstanceID: id, GroupJID: groupJID, Announce: announce, Locked: locked, JoinApproval: joinApproval, MemberAddMode: memberAddMode})
	if f.updateGroupSettingsFn != nil {
		return f.updateGroupSettingsFn(ctx, id, groupJID, announce, locked, joinApproval, memberAddMode)
	}
	return instance.Group{JID: groupJID}, nil
}

// UpdateBlocklist records the call and returns the configured error.
func (f *fakeInstanceService) UpdateBlocklist(ctx context.Context, id uuid.UUID, jid, action string) error {
	f.updateBlocklistCalls = append(f.updateBlocklistCalls, blocklistCall{InstanceID: id, JID: jid, Action: action})
	if f.updateBlocklistFn != nil {
		return f.updateBlocklistFn(ctx, id, jid, action)
	}
	return nil
}

// SetDisappearingTimer records the call and returns the configured error.
func (f *fakeInstanceService) SetDisappearingTimer(ctx context.Context, id uuid.UUID, chatJID string, duration time.Duration) error {
	f.setDisappearingTimerCalls = append(f.setDisappearingTimerCalls, disappearingSetCall{InstanceID: id, ChatJID: chatJID, Duration: duration})
	if f.setDisappearingTimerFn != nil {
		return f.setDisappearingTimerFn(ctx, id, chatJID, duration)
	}
	return nil
}

// SetDefaultDisappearingTimer records the call and returns the configured
// error.
func (f *fakeInstanceService) SetDefaultDisappearingTimer(ctx context.Context, id uuid.UUID, duration time.Duration) error {
	f.setDefaultDisappearingCalls = append(f.setDefaultDisappearingCalls, id)
	if f.setDefaultDisappearingFn != nil {
		return f.setDefaultDisappearingFn(ctx, id, duration)
	}
	return nil
}

// SubscribePresence records the call and returns the configured error.
func (f *fakeInstanceService) SubscribePresence(ctx context.Context, id uuid.UUID, jid string) error {
	f.subscribePresenceCalls = append(f.subscribePresenceCalls, contactTargetCall{InstanceID: id, JID: jid})
	if f.subscribePresenceFn != nil {
		return f.subscribePresenceFn(ctx, id, jid)
	}
	return nil
}

// GetContactQRLink records the call and returns the configured link.
func (f *fakeInstanceService) GetContactQRLink(ctx context.Context, id uuid.UUID, revoke bool) (string, error) {
	f.getContactQRLinkCalls = append(f.getContactQRLinkCalls, contactLinkCall{InstanceID: id, Revoke: revoke})
	if f.getContactQRLinkFn != nil {
		return f.getContactQRLinkFn(ctx, id, revoke)
	}
	return "", nil
}

// CreateNewsletter records the call and returns the configured channel.
func (f *fakeInstanceService) CreateNewsletter(ctx context.Context, id uuid.UUID, title, description string) (instance.Newsletter, error) {
	f.createNewsletterCalls = append(f.createNewsletterCalls, createNewsletterCall{InstanceID: id, Title: title, Description: description})
	if f.createNewsletterFn != nil {
		return f.createNewsletterFn(ctx, id, title, description)
	}
	return instance.Newsletter{Title: title, Description: description}, nil
}

// MuteNewsletter records the call and returns the configured error.
func (f *fakeInstanceService) MuteNewsletter(ctx context.Context, id uuid.UUID, channel string, muted bool) error {
	f.muteNewsletterCalls = append(f.muteNewsletterCalls, muteNewsletterCall{InstanceID: id, ChannelJID: channel, Muted: muted})
	if f.muteNewsletterFn != nil {
		return f.muteNewsletterFn(ctx, id, channel, muted)
	}
	return nil
}

// MarkNewsletterViewed records the call and returns the configured error.
func (f *fakeInstanceService) MarkNewsletterViewed(ctx context.Context, id uuid.UUID, channel string, serverIDs []string) error {
	f.markNewsletterViewedCalls = append(f.markNewsletterViewedCalls, markNewsletterViewedCall{InstanceID: id, ChannelJID: channel, ServerIDs: serverIDs})
	if f.markNewsletterViewedFn != nil {
		return f.markNewsletterViewedFn(ctx, id, channel, serverIDs)
	}
	return nil
}

// ReactNewsletter records the call and returns the configured error.
func (f *fakeInstanceService) ReactNewsletter(ctx context.Context, id uuid.UUID, channel, serverID, reaction string) error {
	f.reactNewsletterCalls = append(f.reactNewsletterCalls, reactNewsletterCall{InstanceID: id, ChannelJID: channel, ServerID: serverID, Reaction: reaction})
	if f.reactNewsletterFn != nil {
		return f.reactNewsletterFn(ctx, id, channel, serverID, reaction)
	}
	return nil
}

// instancesServer builds the server under test with svc as the instance service.
func instancesServer(t *testing.T, svc httpapi.InstanceService) *http.Server {
	t.Helper()
	if svc == nil {
		svc = &fakeInstanceService{}
	}
	return httpapi.New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken}, zerolog.Nop(),
		httpapi.Deps{
			ReadyChecker: checkFunc(func(context.Context) error { return nil }),
			Instances:    svc,
		})
}

// serveJSON sends an authenticated request with an optional body through the
// server handler.
func serveJSON(t *testing.T, srv *http.Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("apikey", testToken)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}
func TestInstancesCreate(t *testing.T) {
	oldest := uuid.New()
	created := &model.Instance{ID: uuid.New(), Name: "loja", ExternalRef: "crm-1", OwnerUserID: &oldest, Connection: model.InstanceConnection{Status: "disconnected"}}
	svc := &fakeInstanceService{
		oldestAdminFn: func(context.Context) (uuid.UUID, error) { return oldest, nil },
		createFn: func(_ context.Context, input instance.CreateInput) (*model.Instance, string, error) {
			if input.Name != "loja" || input.ExternalRef != "crm-1" {
				t.Errorf("Create input = %+v, want name loja and external ref crm-1", input)
			}
			if input.OwnerUserID == nil || *input.OwnerUserID != oldest {
				t.Errorf("Create owner = %v, want the oldest admin %s", input.OwnerUserID, oldest)
			}
			return created, "one-time-key", nil
		},
	}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPost, "/instances",
		`{"name":"loja","external_ref":"crm-1"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	var payload struct {
		Data representation.CreateInstanceResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Instance.ID != created.ID.String() {
		t.Errorf("data.id = %q, want %q", payload.Data.Instance.ID, created.ID)
	}
	if payload.Data.Instance.Connection.Status != "disconnected" {
		t.Errorf("data.status = %q, want %q", payload.Data.Instance.Connection.Status, "disconnected")
	}
	if payload.Data.Instance.Name != "loja" {
		t.Errorf("data = %+v, want instance name loja", payload.Data)
	}

	if strings.Contains(rec.Body.String(), `"external_ref"`) || strings.Contains(rec.Body.String(), `"owner_user_id"`) {
		t.Errorf("body %q leaks internal fields external_ref/owner_user_id", rec.Body.String())
	}
	if payload.Data.InstanceAPIKey != "one-time-key" {
		t.Errorf("data.instance_api_key = %q, want the one-time key", payload.Data.InstanceAPIKey)
	}
}
func TestInstancesCreateDuplicateExternalRef(t *testing.T) {
	svc := &fakeInstanceService{createFn: func(context.Context, instance.CreateInput) (*model.Instance, string, error) {
		return nil, "", instance.ErrExternalRefTaken
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPost, "/instances",
		`{"name":"loja","external_ref":"crm-1"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "conflict" {
		t.Errorf("error code = %q, want %q", code, "conflict")
	}
}
func TestInstancesCreateRejectsInvalidBody(t *testing.T) {
	svc := &fakeInstanceService{}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPost, "/instances", `{"name":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "invalid_request" {
		t.Errorf("error code = %q, want %q", code, "invalid_request")
	}
	if len(svc.createInputs) != 0 {
		t.Errorf("Create calls = %v, want none on a malformed body", svc.createInputs)
	}
}
func TestInstancesCreateRejectsOversizedBody(t *testing.T) {
	svc := &fakeInstanceService{}
	oversized := `{"name":"` + strings.Repeat("a", core.MaxJSONBodyBytes+1) + `"}`

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPost, "/instances", oversized)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "request_too_large" {
		t.Errorf("error code = %q, want %q", code, "request_too_large")
	}
	if len(svc.createInputs) != 0 {
		t.Errorf("Create calls = %v, want none on an oversized body", svc.createInputs)
	}
}
func TestInstancesList(t *testing.T) {
	first := model.Instance{ID: uuid.New(), Name: "a", ExternalRef: "ref-a", CreatedAt: time.Now().UTC().Truncate(time.Second), Connection: model.InstanceConnection{Status: "disconnected", DeviceJID: "5511@wa"}}
	second := model.Instance{ID: uuid.New(), Name: "b", ExternalRef: "ref-b", CreatedAt: time.Now().UTC().Truncate(time.Second), Connection: model.InstanceConnection{Status: "connected", DeviceJID: "5522@wa"}}
	svc := &fakeInstanceService{listFn: func(context.Context) ([]model.Instance, error) {
		return []model.Instance{first, second}, nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data representation.InstanceListResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if len(payload.Data.Instances) != 2 {
		t.Fatalf("data.items length = %d, want 2", len(payload.Data.Instances))
	}
	if payload.Data.Instances[0].ID != first.ID.String() {
		t.Errorf("data.items[0].instance.id = %q, want %q", payload.Data.Instances[0].ID, first.ID)
	}
	if strings.Contains(rec.Body.String(), `"whatsapp_jid"`) || strings.Contains(rec.Body.String(), `"device_jid"`) {
		t.Errorf("body %q leaks internal jid fields", rec.Body.String())
	}
	if payload.Data.Instances[1].Connection.Status != "connected" {
		t.Errorf("data.items[1].instance.connection.status = %q, want connected", payload.Data.Instances[1].Connection.Status)
	}
	if svc.listCalls != 1 {
		t.Errorf("List calls = %d, want 1", svc.listCalls)
	}
}
func TestInstancesListServiceError(t *testing.T) {
	svc := &fakeInstanceService{listFn: func(context.Context) ([]model.Instance, error) {
		return nil, context.Canceled
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "internal_error" {
		t.Errorf("error code = %q, want %q", code, "internal_error")
	}
}
func TestInstancesGet(t *testing.T) {
	want := &model.Instance{ID: uuid.New(), Name: "loja", ExternalRef: "crm-1", Connection: model.InstanceConnection{Status: "connected"}}
	svc := &fakeInstanceService{getFn: func(_ context.Context, id uuid.UUID) (*model.Instance, error) {
		if id != want.ID {
			t.Errorf("Get id = %s, want %s", id, want.ID)
		}
		return want, nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/"+want.ID.String(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data representation.InstanceEnvelope `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Instance.ID != want.ID.String() || payload.Data.Instance.Connection.Status != "connected" {
		t.Errorf("data = %+v, want instance %s connected", payload.Data, want.ID)
	}
}

// TestInstancesGetPublicShape pins the remodeled fixture: the instance nests
// under data.instance, connection exposes the structured last_error and the webhook travels as
// the nested integration block — while external_ref, owner_user_id and
// device/whatsapp JIDs never leave the service.
func TestInstancesGetPublicShape(t *testing.T) {
	lastConnectedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	owner := uuid.New()
	want := &model.Instance{
		ID:          uuid.New(),
		Name:        "loja",
		ExternalRef: "crm-1",
		OwnerUserID: &owner,
		Connection: model.InstanceConnection{
			Status:          "error",
			DeviceJID:       "5511@s.whatsapp.net",
			LastConnectedAt: &lastConnectedAt,
			LastError:       &model.InstanceError{Code: "session_rejected", Message: "qr code expired"},
		},
		Webhook: model.InstanceWebhook{
			IsEnabled: true,
			Events:    []string{"message"},
		},
	}
	svc := &fakeInstanceService{getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
		return want, nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/"+want.ID.String(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data representation.InstanceEnvelope `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	got := payload.Data.Instance
	if got.Connection.LastError == nil || got.Connection.LastError.Code != "session_rejected" ||
		got.Connection.LastError.Message != "qr code expired" || got.Connection.LastError.OccurredAt != nil {
		t.Errorf("data.instance.connection.last_error = %+v, want session_rejected with null occurred_at", got.Connection.LastError)
	}
	if got.Connection.LastConnectedAt == nil || !got.Connection.LastConnectedAt.Equal(lastConnectedAt) {
		t.Errorf("data.instance.connection.last_connected_at = %v, want %v", got.Connection.LastConnectedAt, lastConnectedAt)
	}
	if !got.Integration.Webhook.Enabled || !reflect.DeepEqual(got.Integration.Webhook.Events, []string{"message"}) {
		t.Errorf("data.instance.integration.webhook = %+v, want enabled with [message]", got.Integration.Webhook)
	}
	for _, leaked := range []string{`"external_ref"`, `"owner_user_id"`, `"device_jid"`, `"whatsapp_jid"`, `"api_key_hash"`} {
		if strings.Contains(rec.Body.String(), leaked) {
			t.Errorf("body %q leaks internal field %s", rec.Body.String(), leaked)
		}
	}
}
func TestInstancesGetNotFound(t *testing.T) {
	svc := &fakeInstanceService{getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
		return nil, instance.ErrNotFound
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/"+uuid.NewString(), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
		t.Errorf("error code = %q, want %q", code, "not_found")
	}
}
func TestInstancesGetRejectsMalformedID(t *testing.T) {
	svc := &fakeInstanceService{}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/bad.name", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
		t.Errorf("error code = %q, want %q", code, "not_found")
	}
	if len(svc.getIDs) != 0 {
		t.Errorf("Get calls = %v, want none for a malformed id", svc.getIDs)
	}
}
func TestInstancesUpdate(t *testing.T) {
	id := uuid.New()
	updated := &model.Instance{ID: id, Name: "novo", ExternalRef: "ref-1", Connection: model.InstanceConnection{Status: "connected"}}
	svc := &fakeInstanceService{updateFn: func(_ context.Context, gotID uuid.UUID, input instance.UpdateInput) (*model.Instance, error) {
		if gotID != id {
			t.Errorf("Update id = %s, want %s", gotID, id)
		}
		if input.Name == nil || *input.Name != "novo" {
			t.Errorf("Update name = %v, want novo", input.Name)
		}
		if input.ExternalRef != nil {
			t.Errorf("Update external ref = %q, want nil (preserved)", *input.ExternalRef)
		}
		return updated, nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPatch, "/instances/"+id.String(), `{"name":"novo"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data representation.InstanceEnvelope `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Instance.Name != "novo" {
		t.Errorf("data = %+v, want instance name novo", payload.Data)
	}
	if strings.Contains(rec.Body.String(), `"external_ref"`) {
		t.Errorf("body %q leaks internal external_ref", rec.Body.String())
	}
}
func TestInstancesUpdateClearsExternalRef(t *testing.T) {
	id := uuid.New()
	svc := &fakeInstanceService{updateFn: func(_ context.Context, _ uuid.UUID, input instance.UpdateInput) (*model.Instance, error) {
		if input.ExternalRef == nil || *input.ExternalRef != "" {
			t.Errorf("Update external ref = %v, want a pointer to an empty string", input.ExternalRef)
		}
		return &model.Instance{ID: id, Name: "loja", Connection: model.InstanceConnection{Status: "disconnected"}}, nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPatch, "/instances/"+id.String(), `{"external_ref":""}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
func TestInstancesUpdateNotFound(t *testing.T) {
	svc := &fakeInstanceService{updateFn: func(context.Context, uuid.UUID, instance.UpdateInput) (*model.Instance, error) {
		return nil, instance.ErrNotFound
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodPatch, "/instances/"+uuid.NewString(), `{"name":"novo"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
		t.Errorf("error code = %q, want %q", code, "not_found")
	}
}
func TestInstancesDelete(t *testing.T) {
	id := uuid.New()
	svc := &fakeInstanceService{}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodDelete, "/instances/"+id.String(), "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
	if len(svc.deleteIDs) != 1 || svc.deleteIDs[0] != id {
		t.Errorf("Delete calls = %v, want [%s]", svc.deleteIDs, id)
	}
}
func TestInstancesDeleteNotFound(t *testing.T) {
	svc := &fakeInstanceService{deleteFn: func(context.Context, uuid.UUID) error {
		return instance.ErrNotFound
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodDelete, "/instances/"+uuid.NewString(), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "not_found" {
		t.Errorf("error code = %q, want %q", code, "not_found")
	}
}
func TestInstancesInternalError(t *testing.T) {
	svc := &fakeInstanceService{getFn: func(context.Context, uuid.UUID) (*model.Instance, error) {
		return nil, context.DeadlineExceeded
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/"+uuid.NewString(), "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "internal_error" {
		t.Errorf("error code = %q, want %q", code, "internal_error")
	}
	if strings.Contains(rec.Body.String(), "deadline") {
		t.Errorf("body leaks the internal error: %q", rec.Body.String())
	}
}
