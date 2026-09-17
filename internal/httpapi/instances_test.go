package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/instance"
	"wzap/internal/model"
	"wzap/internal/session"
)

// fakeInstanceService is an in-memory InstanceService: the function fields
// configure each outcome and the recorded fields expose the calls the handlers
// made.
type fakeInstanceService struct {
	createFn             func(ctx context.Context, input instance.CreateInput) (*model.Instance, string, error)
	oldestAdminFn        func(ctx context.Context) (uuid.UUID, error)
	getFn                func(ctx context.Context, id uuid.UUID) (*model.Instance, error)
	listFn               func(ctx context.Context, limit int, cursor string) ([]model.Instance, string, error)
	updateFn             func(ctx context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error)
	deleteFn             func(ctx context.Context, id uuid.UUID) error
	disconnectFn         func(ctx context.Context, id uuid.UUID) error
	connectFn            func(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)
	qrFn                 func(ctx context.Context, id uuid.UUID) (instance.ConnectResult, error)
	revokeFn             func(ctx context.Context, id uuid.UUID, chatJID, messageID string) error
	markReadFn           func(ctx context.Context, id uuid.UUID, chatJID, senderJID, messageID string) error
	sendPresenceFn       func(ctx context.Context, id uuid.UUID, chatJID, state string) error
	pairPhoneFn          func(ctx context.Context, id uuid.UUID, phone string) (instance.PairPhoneResult, error)
	createGroupFn        func(ctx context.Context, id uuid.UUID, input instance.CreateGroupInput) (instance.Group, error)
	getGroupFn           func(ctx context.Context, id uuid.UUID, groupJID string) (instance.Group, error)
	updateGroupFn        func(ctx context.Context, id uuid.UUID, groupJID string, input instance.UpdateGroupInput) (instance.Group, error)
	setGroupPhotoFn      func(ctx context.Context, id uuid.UUID, groupJID string, image []byte) error
	updateParticipantsFn func(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error
	getGroupInviteFn     func(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	resetGroupInviteFn   func(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	joinGroupFn          func(ctx context.Context, id uuid.UUID, inviteCode string) (string, error)
	leaveGroupFn         func(ctx context.Context, id uuid.UUID, groupJID string) error
	followNewsletterFn   func(ctx context.Context, id uuid.UUID, channelJID string) error
	unfollowNewsletterFn func(ctx context.Context, id uuid.UUID, channelJID string) error
	getNewsletterFn      func(ctx context.Context, id uuid.UUID, channelJID string) (instance.Newsletter, error)
	listNewslettersFn    func(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Newsletter, string, error)
	publishStatusFn      func(ctx context.Context, id uuid.UUID, input session.StatusInput) (string, error)
	listStatusesFn       func(ctx context.Context, id uuid.UUID) ([]session.StatusInfo, error)
	deleteStatusFn       func(ctx context.Context, id uuid.UUID, statusID string) error

	createInputs            []instance.CreateInput
	updateInputs            []instance.UpdateInput
	getIDs                  []uuid.UUID
	deleteIDs               []uuid.UUID
	disconnectIDs           []uuid.UUID
	connectIDs              []uuid.UUID
	qrIDs                   []uuid.UUID
	revokeCalls             []revokeCall
	markReadCalls           []markReadCall
	presenceCalls           []presenceCall
	pairPhoneCalls          []pairPhoneCall
	createGroupCalls        []createGroupCall
	getGroupCalls           []groupTargetCall
	updateGroupCalls        []updateGroupCall
	setGroupPhotoCalls      []setGroupPhotoCall
	updateParticipantsCalls []updateParticipantsCall
	getGroupInviteCalls     []groupTargetCall
	resetGroupInviteCalls   []groupTargetCall
	joinGroupCalls          []joinGroupCall
	leaveGroupCalls         []groupTargetCall
	followNewsletterCalls   []newsletterTargetCall
	unfollowNewsletterCalls []newsletterTargetCall
	getNewsletterCalls      []newsletterTargetCall
	listNewsletterCalls     []listNewsletterCall
	publishStatusCalls      []publishStatusCall
	listStatusCalls         []uuid.UUID
	deleteStatusCalls       []deleteStatusCall
	listLimit               int
	listCursor              string
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

// Create records the input and returns the configured instance with its
// one-time key, defaulting to a fresh disconnected instance with an empty key.
func (f *fakeInstanceService) Create(ctx context.Context, input instance.CreateInput) (*model.Instance, string, error) {
	f.createInputs = append(f.createInputs, input)
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return &model.Instance{ID: uuid.New(), Name: input.Name, ExternalRef: input.ExternalRef, Status: "disconnected", OwnerUserID: input.OwnerUserID}, "", nil
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
	return &model.Instance{ID: id, Name: "loja", Status: "disconnected"}, nil
}

// List records the pagination and returns the configured page, defaulting to an
// empty one.
func (f *fakeInstanceService) List(ctx context.Context, limit int, cursor string) ([]model.Instance, string, error) {
	f.listLimit = limit
	f.listCursor = cursor
	if f.listFn != nil {
		return f.listFn(ctx, limit, cursor)
	}
	return nil, "", nil
}

// Update records the input and returns the configured instance.
func (f *fakeInstanceService) Update(ctx context.Context, id uuid.UUID, input instance.UpdateInput) (*model.Instance, error) {
	f.updateInputs = append(f.updateInputs, input)
	if f.updateFn != nil {
		return f.updateFn(ctx, id, input)
	}
	return &model.Instance{ID: id, Name: "loja", Status: "disconnected"}, nil
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

// instancesServer builds the server under test with svc as the instance service.
func instancesServer(t *testing.T, svc InstanceService) *http.Server {
	t.Helper()
	if svc == nil {
		svc = &fakeInstanceService{}
	}
	return New(config.Config{HTTPAddr: "127.0.0.1:0", APIKey: testToken}, zerolog.Nop(),
		Deps{
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
	created := &model.Instance{ID: uuid.New(), Name: "loja", ExternalRef: "crm-1", Status: "disconnected", OwnerUserID: &oldest}
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
		Data createInstanceResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.ID != created.ID.String() {
		t.Errorf("data.id = %q, want %q", payload.Data.ID, created.ID)
	}
	if payload.Data.Status != "disconnected" {
		t.Errorf("data.status = %q, want %q", payload.Data.Status, "disconnected")
	}
	if payload.Data.Name != "loja" || payload.Data.ExternalRef != "crm-1" {
		t.Errorf("data = %+v, want name loja and external ref crm-1", payload.Data)
	}
	if payload.Data.OwnerUserID == nil || *payload.Data.OwnerUserID != oldest {
		t.Errorf("data.owner_user_id = %v, want the oldest admin %s", payload.Data.OwnerUserID, oldest)
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
	oversized := `{"name":"` + strings.Repeat("a", maxJSONBodyBytes+1) + `"}`

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
	first := model.Instance{
		ID: uuid.New(), Name: "a", ExternalRef: "ref-a", Status: "disconnected",
		WhatsAppJID: "5511@wa", CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	second := model.Instance{
		ID: uuid.New(), Name: "b", ExternalRef: "ref-b", Status: "connected",
		WhatsAppJID: "5522@wa", CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	svc := &fakeInstanceService{listFn: func(context.Context, int, string) ([]model.Instance, string, error) {
		return []model.Instance{first, second}, "cursor-1", nil
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data instanceListResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if len(payload.Data.Items) != 2 {
		t.Fatalf("data.items length = %d, want 2", len(payload.Data.Items))
	}
	if payload.Data.Items[0].ID != first.ID.String() {
		t.Errorf("data.items[0].id = %q, want %q", payload.Data.Items[0].ID, first.ID)
	}
	if payload.Data.Items[1].WhatsAppJID != second.WhatsAppJID {
		t.Errorf("data.items[1].whatsapp_jid = %q, want %q", payload.Data.Items[1].WhatsAppJID, second.WhatsAppJID)
	}
	if payload.Data.NextCursor != "cursor-1" {
		t.Errorf("data.next_cursor = %q, want %q", payload.Data.NextCursor, "cursor-1")
	}
	if svc.listLimit != defaultInstancesLimit {
		t.Errorf("List limit = %d, want the default %d", svc.listLimit, defaultInstancesLimit)
	}
	if svc.listCursor != "" {
		t.Errorf("List cursor = %q, want empty", svc.listCursor)
	}
}

func TestInstancesListPassesPagination(t *testing.T) {
	svc := &fakeInstanceService{}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances?limit=7&cursor=abc", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		Data instanceListResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Items == nil {
		t.Error("data.items = null, want an empty array")
	}
	if svc.listLimit != 7 || svc.listCursor != "abc" {
		t.Errorf("List(%d, %q), want (7, abc)", svc.listLimit, svc.listCursor)
	}
}

func TestInstancesListLimit(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "default", query: "", want: defaultInstancesLimit},
		{name: "explicit", query: "?limit=7", want: 7},
		{name: "capped", query: "?limit=1000", want: maxInstancesLimit},
		{name: "malformed falls back to default", query: "?limit=abc", want: defaultInstancesLimit},
		{name: "non positive falls back to default", query: "?limit=0", want: defaultInstancesLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeInstanceService{}

			rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances"+tt.query, "")

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if svc.listLimit != tt.want {
				t.Errorf("List limit = %d, want %d", svc.listLimit, tt.want)
			}
		})
	}
}

func TestInstancesListRejectsInvalidCursor(t *testing.T) {
	svc := &fakeInstanceService{listFn: func(context.Context, int, string) ([]model.Instance, string, error) {
		return nil, "", instance.ErrInvalidCursor
	}}

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances?cursor=not-a-uuid", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != "invalid_request" {
		t.Errorf("error code = %q, want %q", code, "invalid_request")
	}
}

func TestInstancesGet(t *testing.T) {
	want := &model.Instance{ID: uuid.New(), Name: "loja", ExternalRef: "crm-1", Status: "connected"}
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
		Data instanceResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.ID != want.ID.String() || payload.Data.Status != "connected" {
		t.Errorf("data = %+v, want instance %s connected", payload.Data, want.ID)
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

	rec := serveJSON(t, instancesServer(t, svc), http.MethodGet, "/instances/not-a-uuid", "")

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
	updated := &model.Instance{ID: id, Name: "novo", ExternalRef: "ref-1", Status: "connected"}
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
		Data instanceResponse `json:"data"`
	}
	decodeJSON(t, rec.Body.Bytes(), &payload)
	if payload.Data.Name != "novo" || payload.Data.ExternalRef != "ref-1" {
		t.Errorf("data = %+v, want name novo and external ref ref-1", payload.Data)
	}
}

func TestInstancesUpdateClearsExternalRef(t *testing.T) {
	id := uuid.New()
	svc := &fakeInstanceService{updateFn: func(_ context.Context, _ uuid.UUID, input instance.UpdateInput) (*model.Instance, error) {
		if input.ExternalRef == nil || *input.ExternalRef != "" {
			t.Errorf("Update external ref = %v, want a pointer to an empty string", input.ExternalRef)
		}
		return &model.Instance{ID: id, Name: "loja", Status: "disconnected"}, nil
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
