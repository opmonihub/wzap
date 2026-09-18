// Package session defines the WhatsApp session contracts consumed by the wzap
// services. The whatsmeow implementation lives in the whatsmeow subpackage so
// the library types stay confined to it.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"wzap/internal/model"
)

// Status is the lifecycle state of an instance session.
type Status string

const (
	// StatusDisconnected means the session has no active connection.
	StatusDisconnected Status = "disconnected"
	// StatusPairing means the session is waiting for a QR code to be scanned.
	StatusPairing Status = "pairing"
	// StatusConnected means the session is authenticated and online.
	StatusConnected Status = "connected"
	// StatusError means the session stopped on a failure that needs attention.
	StatusError Status = "error"
)

// Session errors classified by the outbox workers and the services.
var (
	// ErrTransient marks a failure that may succeed on a retry.
	ErrTransient = errors.New("session transient error")
	// ErrNotConnected marks an operation attempted on a disconnected session.
	ErrNotConnected = errors.New("session not connected")
	// ErrInvalidRecipient marks a malformed recipient JID.
	ErrInvalidRecipient = errors.New("session invalid recipient")
	// ErrNotFound marks an operation on an unknown remote resource (group or
	// channel). The service maps it to the resource 404.
	ErrNotFound = errors.New("session resource not found")
	// ErrForbidden marks an operation the instance may not perform on the
	// remote resource (for example a non-admin managing a group). The service
	// maps it to 403 without leaking the upstream cause.
	ErrForbidden = errors.New("session forbidden")
	// ErrNoDevice marks an instance whose persisted device is gone, for
	// example after an external logout or a device removal. The pairing cannot
	// be resumed and the instance must be paired again.
	ErrNoDevice = errors.New("session device not found")
	// ErrUnsupported marks an operation the upstream protocol does not
	// support on this session (for example setting the profile name or
	// photo, which the companion library exposes no setter for). The
	// service maps it to the resource 501.
	ErrUnsupported = errors.New("session operation not supported")
	// ErrStatusNotFound marks a status id the session does not know: it was
	// never published here or it already expired upstream. The service maps
	// it to the resource 404.
	ErrStatusNotFound = errors.New("session status not found")
)

// OutboundMessage is the normalized message handed to a session for delivery.
// Payload carries the type-specific JSON body: text {"text"}, location
// {"latitude","longitude","name","address"}, contact {"display_name","vcard"}
// and media {"caption","filename","mime_type","ptt"}, with MediaPath pointing
// at the file on disk for media types.
type OutboundMessage struct {
	Type         string
	RecipientJID string
	Payload      []byte
	MediaPath    string
}

// InboundMessage is a received message translated away from the library types.
type InboundMessage struct {
	InstanceID uuid.UUID
	MessageID  string
	ChatJID    string
	SenderJID  string
	IsGroup    bool
	Type       string
	Text       string
	Timestamp  time.Time

	MediaAvailable bool
	MediaMime      string
	MediaFilename  string
	// MediaLength is the size in bytes announced by the source, or zero when
	// it is unknown. It lets the consumer reject oversized media before
	// downloading it.
	MediaLength int64
	// MediaDownload fetches the media bytes on demand. It is nil when the
	// message carries no downloadable media.
	MediaDownload func(ctx context.Context) ([]byte, error)
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed or the
	// source carried nothing worth keeping.
	Raw json.RawMessage
}

// Receipt is a delivery/read acknowledgement for previously sent messages.
type Receipt struct {
	InstanceID uuid.UUID
	MessageIDs []string
	ChatJID    string
	SenderJID  string
	Status     string
	Timestamp  time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// MessageEdit is an edit of a previously received message, translated away
// from the library types. MessageID is the original message, Text the
// replacement content and Timestamp the edit moment.
type MessageEdit struct {
	InstanceID uuid.UUID
	MessageID  string
	ChatJID    string
	SenderJID  string
	IsGroup    bool
	Text       string
	Timestamp  time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// MessageDelete is a delete-for-everyone of a previously received message,
// translated away from the library types. MessageID is the original message
// and Timestamp the moment the revocation was observed.
type MessageDelete struct {
	InstanceID uuid.UUID
	MessageID  string
	ChatJID    string
	SenderJID  string
	IsGroup    bool
	Timestamp  time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// PollVote is a vote on a poll message, translated away from the library
// types. PollMessageID is the poll being voted on. SelectedOptionIDs are the
// hex of the option hashes carried on the wire; SelectedOptionNames stays
// empty on inbound votes because the wire carries only hashes, never names.
type PollVote struct {
	InstanceID          uuid.UUID
	PollMessageID       string
	ChatJID             string
	SenderJID           string
	IsGroup             bool
	SelectedOptionIDs   []string
	SelectedOptionNames []string
	Timestamp           time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// Reaction is a reaction to a previous message, translated away from the
// library types. MessageID is the reacted message and Emoji the reaction
// content; an empty emoji removes the reaction on the same target.
type Reaction struct {
	InstanceID uuid.UUID
	MessageID  string
	ChatJID    string
	SenderJID  string
	IsGroup    bool
	Emoji      string
	Timestamp  time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// InteractiveResponse is a unified answer to an interactive message
// (buttons, list or native flow), translated away from the library types.
// MessageID is the response stanza id. Source is one of buttons, list or
// native_flow; SelectedID carries the button id, the list row id or the flow
// name, and Title the display text.
type InteractiveResponse struct {
	InstanceID uuid.UUID
	MessageID  string
	ChatJID    string
	SenderJID  string
	IsGroup    bool
	Source     string
	SelectedID string
	Title      string
	Timestamp  time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// GroupParticipant is one member of a group, translated away from the library
// types. JID is the primary address; IsAdmin covers admins and IsSuperAdmin
// the group creator.
type GroupParticipant struct {
	JID          string
	IsAdmin      bool
	IsSuperAdmin bool
}

// GroupInfo is the metadata of a group, translated away from the library
// types. Description is the group topic; DescriptionID its version, empty
// when the upstream carries none.
type GroupInfo struct {
	JID              string
	Name             string
	Description      string
	DescriptionID    string
	Participants     []GroupParticipant
	ParticipantCount int
	CreatedAt        time.Time
}

// NewsletterInfo is the metadata of a channel, translated away from the
// library types. FollowerCount is best-effort: zero when the upstream omits
// it.
type NewsletterInfo struct {
	ChannelJID    string
	Title         string
	Description   string
	FollowerCount int
}

// Group event kinds carried by GroupEvent.Kind: membership changes travel as
// participants, subject/topic/picture changes as info.
const (
	// GroupEventParticipants marks a membership change: members joined, left,
	// were added or were removed.
	GroupEventParticipants = "participants"
	// GroupEventInfo marks a metadata change: subject, topic or picture.
	GroupEventInfo = "info"
)

// GroupEvent is a group change observed on the wire, translated away from the
// library types. ActorJID is who made the change (empty when the upstream
// carries none); Affected lists the members that joined, left or were
// moved; Name and Description snapshot the new subject/topic on info events.
type GroupEvent struct {
	InstanceID  uuid.UUID
	GroupJID    string
	Kind        string
	ActorJID    string
	Affected    []string
	Name        string
	Description string
	Timestamp   time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// Status kinds published by PublishStatus: plain text or an image/video with
// an optional caption.
const (
	// StatusKindText is a plain text status.
	StatusKindText = "text"
	// StatusKindImage is an image status with an optional caption.
	StatusKindImage = "image"
	// StatusKindVideo is a video status with an optional caption.
	StatusKindVideo = "video"
)

// StatusInput is the content accepted by PublishStatus. A text status carries
// Text only; an image or video status carries the media bytes with their mime
// type and an optional caption. The handler validates the kind and the size
// before the session is touched.
type StatusInput struct {
	Text      string
	MediaMime string
	MediaData []byte
	Caption   string
}

// StatusInfo is one own status published through the session: the upstream
// id, its kind and content snapshot, and when it was published here.
type StatusInfo struct {
	ID        string
	Kind      string
	Text      string
	Caption   string
	CreatedAt time.Time
}

// Call states carried by CallEvent.State: the unified call.offer event
// distinguishes the offer, the accept, the reject and the end through this
// field (Ruling D2).
const (
	// CallStateOffer marks an incoming call offer (1:1 or group notice).
	CallStateOffer = "offer"
	// CallStateAccept marks a call accepted on another device.
	CallStateAccept = "accept"
	// CallStateReject marks a call rejected on another device.
	CallStateReject = "reject"
	// CallStateEnd marks a terminated call.
	CallStateEnd = "end"
)

// CallEvent is a call change observed on the wire, translated away from the
// library types. CallID is the upstream call id; FromJID the caller (the
// creator on group notices); State one of the CallState* values; IsVideo
// best-effort (true only when the wire names video, as on group notices).
// The service never initiates calls: it only observes them and rejects the
// active one through RejectCall when the upstream allows it.
type CallEvent struct {
	InstanceID uuid.UUID
	CallID     string
	FromJID    string
	State      string
	IsVideo    bool
	Timestamp  time.Time
	// Raw is the best-effort JSON of the raw upstream event, captured by the
	// adapter for webhook delivery. It is nil when the capture failed.
	Raw json.RawMessage
}

// Profile is the own profile of the instance: the display name, the recado
// (status text) and the photo URL (empty when the instance carries no photo
// or the upstream reports none). The name is best-effort: the pinned library
// exposes no profile fetch, so the adapter reports the device push name.
type Profile struct {
	Name       string
	StatusText string
	PhotoURL   string
}

// Privacy is the own privacy of the instance, one value per field. Every
// field travels as the upstream literal: last_seen, profile_photo, status
// and groups_add take all, contacts, contact_blacklist or none; read_receipts
// takes all or none (the pinned library models it as a two-value switch, so
// the API exposes the allowlist instead of a boolean — Ruling D3).
type Privacy struct {
	LastSeen     string
	ProfilePhoto string
	Status       string
	ReadReceipts string
	GroupsAdd    string
}

// ContactCheckResult is the on-WhatsApp lookup of one phone number: JID and
// IsOnWhatsApp report the registration, LastSeen is nil when unknown.
type ContactCheckResult struct {
	Phone        string
	JID          string
	IsOnWhatsApp bool
	LastSeen     *time.Time
}

// ProfilePictureInfo is the picture URL of a contact with its version token,
// empty when the contact carries no picture.
type ProfilePictureInfo struct {
	URL     string
	Version string
}

// BusinessProfile is the business profile of a contact: the display name, the
// description and the verified name (empty when not verified).
type BusinessProfile struct {
	Name         string
	Description  string
	VerifiedName string
}

// NewsletterMessage is one channel message: ServerID is the upstream id,
// Content the text snapshot and Timestamp the publish moment.
type NewsletterMessage struct {
	ServerID  string
	Content   string
	Timestamp time.Time
}

// StatusPrivacy is the own status privacy: Mode travels as the upstream
// literal (contacts, contact_blacklist or none) with JIDs the allow/deny
// list.
type StatusPrivacy struct {
	Mode string
	JIDs []string
}

// Disappearing timer presets carried by the chat-settings methods: Off
// disables the timer, the others set the matching duration.
// DisappearingOff disables disappearing messages.
const DisappearingOff time.Duration = 0

// Disappearing24h expires messages after 24 hours.
const Disappearing24h time.Duration = 24 * time.Hour

// Disappearing7d expires messages after 7 days.
const Disappearing7d time.Duration = 7 * 24 * time.Hour

// Disappearing90d expires messages after 90 days.
const Disappearing90d time.Duration = 90 * 24 * time.Hour

// EventSink consumes session events. Implementations must be safe for
// concurrent use and should not block the session for long.
type EventSink interface {
	OnMessage(ctx context.Context, msg InboundMessage)
	OnMessageEdit(ctx context.Context, edit MessageEdit)
	OnMessageDelete(ctx context.Context, del MessageDelete)
	OnPollVote(ctx context.Context, vote PollVote)
	OnReaction(ctx context.Context, reaction Reaction)
	OnInteractiveResponse(ctx context.Context, response InteractiveResponse)
	OnGroupEvent(ctx context.Context, event GroupEvent)
	OnCallEvent(ctx context.Context, event CallEvent)
	OnReceipt(ctx context.Context, receipt Receipt)
	OnConnection(ctx context.Context, instanceID uuid.UUID, status Status, jid string, reason string)
}

// Session is a single instance connection.
type Session interface {
	// Connect starts pairing and returns the first QR code and its expiry. It
	// returns an empty QR when the instance already has stored credentials and
	// only needs to be brought online.
	Connect(ctx context.Context) (qr string, expiresAt time.Time, err error)
	// QR returns the current pairing QR code and its expiry.
	QR(ctx context.Context) (string, time.Time, error)
	// Send delivers an outbound message and returns the WhatsApp message id.
	Send(ctx context.Context, msg OutboundMessage) (whatsappID string, err error)
	// IsOnWhatsApp checks whether a phone number is registered on WhatsApp,
	// returning its canonical JID when it is.
	IsOnWhatsApp(ctx context.Context, phone string) (jid string, ok bool, err error)
	// SendPresence reports chat presence ("composing"/"paused") or user
	// presence ("available"/"unavailable") for a chat.
	SendPresence(ctx context.Context, chatJID, state string) error
	// DeleteMessage revokes a sent message for everyone in the chat.
	DeleteMessage(ctx context.Context, chatJID, messageID string) error
	// MarkRead sends a read receipt for messageID in chatJID. senderJID is the
	// author of the message and is required in group chats; an empty sender
	// falls back to the chat JID for direct chats.
	MarkRead(ctx context.Context, chatJID, senderJID, messageID string) error
	// PairPhone requests the 8-character pairing code that links the phone
	// number without scanning a QR code. The session must already hold an open
	// pairing channel (Connect first); the code expires with the QR channel.
	PairPhone(ctx context.Context, number string) (code string, err error)
	// CreateGroup creates a group with name and the initial participants and
	// returns its metadata. participantJIDs are the member addresses; the
	// instance itself joins implicitly.
	CreateGroup(ctx context.Context, name string, participantJIDs []string) (GroupInfo, error)
	// GetGroup returns the live metadata of groupJID. An unknown group is
	// ErrNotFound; the metadata cache (storage) is never consulted here.
	GetGroup(ctx context.Context, groupJID string) (GroupInfo, error)
	// SetGroupName replaces the subject of groupJID.
	SetGroupName(ctx context.Context, groupJID, name string) error
	// SetGroupDescription replaces the topic of groupJID. An empty
	// description clears it.
	SetGroupDescription(ctx context.Context, groupJID, description string) error
	// SetGroupPhoto replaces the picture of groupJID with the image bytes
	// (jpeg, png or webp).
	SetGroupPhoto(ctx context.Context, groupJID string, image []byte) error
	// UpdateGroupParticipants applies action (add, remove, promote or demote)
	// to participantJIDs of groupJID. Acting without group permission is
	// ErrForbidden.
	UpdateGroupParticipants(ctx context.Context, groupJID, action string, participantJIDs []string) error
	// GetGroupInvite returns the current invite code of groupJID without
	// revoking it.
	GetGroupInvite(ctx context.Context, groupJID string) (code string, err error)
	// ResetGroupInvite revokes the current invite code of groupJID and returns
	// the fresh one.
	ResetGroupInvite(ctx context.Context, groupJID string) (code string, err error)
	// JoinGroup enters the group behind inviteCode (the bare code or the full
	// invite link) and returns the group JID.
	JoinGroup(ctx context.Context, inviteCode string) (groupJID string, err error)
	// LeaveGroup removes the instance from groupJID.
	LeaveGroup(ctx context.Context, groupJID string) error
	// FollowNewsletter subscribes the instance to channelJID. An unknown
	// channel is ErrNotFound.
	FollowNewsletter(ctx context.Context, channelJID string) error
	// UnfollowNewsletter ends the subscription of the instance to channelJID.
	UnfollowNewsletter(ctx context.Context, channelJID string) error
	// GetNewsletter returns the live metadata of channelJID, refreshed on
	// demand: the stored metadata is a cache, never the source of truth.
	GetNewsletter(ctx context.Context, channelJID string) (NewsletterInfo, error)
	// ListNewsletters returns the live metadata of every channel the instance
	// follows.
	ListNewsletters(ctx context.Context) ([]NewsletterInfo, error)
	// PublishStatus publishes an own status (story) of the input kind and
	// returns its upstream id. A text status carries Text; an image or video
	// status carries the media bytes with their mime type and an optional
	// caption. Publishing answers 202 at the REST boundary: the id travels
	// as message_id with the same idempotency semantics as the message
	// sends.
	PublishStatus(ctx context.Context, input StatusInput) (statusID string, err error)
	// ListStatuses returns the own statuses published through this session
	// that are still known here. Entries expire upstream after ~24h; the
	// registry drops them on list.
	ListStatuses(ctx context.Context) ([]StatusInfo, error)
	// DeleteStatus removes the own statusID published through this session.
	// An empty or unknown id is ErrStatusNotFound.
	DeleteStatus(ctx context.Context, statusID string) error
	// RejectCall rejects the active call callID from fromJID. The service
	// never initiates calls; when the upstream cannot reject, the adapter
	// returns ErrUnsupported for the documented 501.
	RejectCall(ctx context.Context, fromJID, callID string) error
	// GetProfile returns the own profile of the instance (name, recado and
	// photo URL).
	GetProfile(ctx context.Context) (Profile, error)
	// SetProfileName replaces the own display name. The pinned library
	// exposes no setter, so the adapter returns ErrUnsupported for the
	// documented 501.
	SetProfileName(ctx context.Context, name string) error
	// SetProfileStatusText replaces the own recado (status text). An empty
	// text clears it.
	SetProfileStatusText(ctx context.Context, text string) error
	// SetProfilePhoto replaces the own photo with the image bytes (jpeg,
	// png or webp). The pinned library exposes no setter, so the adapter
	// returns ErrUnsupported for the documented 501.
	SetProfilePhoto(ctx context.Context, image []byte) error
	// GetPrivacy returns the own privacy settings, one value per field.
	GetPrivacy(ctx context.Context) (Privacy, error)
	// SetPrivacy applies the non-empty fields of input to the upstream
	// privacy settings and returns the resulting settings.
	SetPrivacy(ctx context.Context, input Privacy) (Privacy, error)
	// EditMessage replaces the text of messageID in chatJID. An unknown
	// message is ErrNotFound; editing without permission is ErrForbidden.
	EditMessage(ctx context.Context, chatJID, messageID, text string) (newMessageID string, err error)
	// GetJoinedGroups returns the live metadata of every group the instance
	// is a member of.
	GetJoinedGroups(ctx context.Context) ([]GroupInfo, error)
	// GetGroupInfoFromLink resolves inviteCode to the group metadata without
	// joining. An unknown code is ErrNotFound.
	GetGroupInfoFromLink(ctx context.Context, inviteCode string) (GroupInfo, error)
	// GetGroupRequestParticipants lists the pending join requests of
	// groupJID. An unknown group is ErrNotFound.
	GetGroupRequestParticipants(ctx context.Context, groupJID string) ([]GroupParticipant, error)
	// UpdateGroupRequestParticipants approves or rejects participantJIDs of
	// groupJID. An unknown action is ErrInvalidRecipient; acting without
	// group permission is ErrForbidden.
	UpdateGroupRequestParticipants(ctx context.Context, groupJID, action string, participantJIDs []string) error
	// SetGroupAnnounce toggles the announce-only mode of groupJID. Acting
	// without group permission is ErrForbidden.
	SetGroupAnnounce(ctx context.Context, groupJID string, announce bool) error
	// SetGroupLocked toggles the locked (info-edit restricted) mode of
	// groupJID. Acting without group permission is ErrForbidden.
	SetGroupLocked(ctx context.Context, groupJID string, locked bool) error
	// SetGroupJoinApprovalMode sets the join-approval mode of groupJID. An
	// unknown mode is ErrInvalidRecipient.
	SetGroupJoinApprovalMode(ctx context.Context, groupJID, mode string) error
	// SetGroupMemberAddMode sets the member-add mode of groupJID. An unknown
	// mode is ErrInvalidRecipient.
	SetGroupMemberAddMode(ctx context.Context, groupJID, mode string) error
	// CheckContacts looks up phones on WhatsApp, one result per input in
	// order. Malformed numbers report IsOnWhatsApp false, never an error.
	CheckContacts(ctx context.Context, phones []string) ([]ContactCheckResult, error)
	// GetContactDevices lists the companion device JIDs of jid. A malformed
	// jid is ErrInvalidRecipient.
	GetContactDevices(ctx context.Context, jid string) ([]string, error)
	// GetProfilePictureInfo returns the picture URL of jid. A contact
	// without picture returns an empty URL without failing.
	GetProfilePictureInfo(ctx context.Context, jid string) (ProfilePictureInfo, error)
	// GetBusinessProfile returns the business profile of jid. A contact
	// without business profile is ErrNotFound.
	GetBusinessProfile(ctx context.Context, jid string) (BusinessProfile, error)
	// GetContactQRLink returns the own contact QR link, revoking it first
	// when revoke is true.
	GetContactQRLink(ctx context.Context, revoke bool) (link string, err error)
	// GetBlocklist returns the JIDs the instance has blocked.
	GetBlocklist(ctx context.Context) ([]string, error)
	// UpdateBlocklist applies action (block or unblock) to jid. An unknown
	// action is ErrInvalidRecipient.
	UpdateBlocklist(ctx context.Context, jid, action string) error
	// CreateNewsletter creates a channel with title and description and
	// returns its metadata. A duplicate title is ErrForbidden.
	CreateNewsletter(ctx context.Context, title, description string) (NewsletterInfo, error)
	// NewsletterToggleMute mutes or unmutes channelJID. An unknown channel
	// is ErrNotFound.
	NewsletterToggleMute(ctx context.Context, channelJID string, muted bool) error
	// NewsletterMarkViewed marks serverIDs of channelJID as viewed. An
	// unknown channel is ErrNotFound.
	NewsletterMarkViewed(ctx context.Context, channelJID string, serverIDs []string) error
	// NewsletterSendReaction sends reaction to serverID of channelJID. An
	// unknown message is ErrNotFound.
	NewsletterSendReaction(ctx context.Context, channelJID, serverID, reaction string) error
	// GetNewsletterMessages pages the messages of channelJID from cursor
	// with limit entries. An unknown channel is ErrNotFound.
	GetNewsletterMessages(ctx context.Context, channelJID, cursor string, limit int) ([]NewsletterMessage, string, error)
	// GetNewsletterMessageUpdates returns the pending message updates of
	// channelJID. An unknown channel is ErrNotFound.
	GetNewsletterMessageUpdates(ctx context.Context, channelJID string) ([]NewsletterMessage, error)
	// SetDisappearingTimer sets the disappearing timer of chatJID. An
	// unsupported duration is ErrInvalidRecipient.
	SetDisappearingTimer(ctx context.Context, chatJID string, duration time.Duration) error
	// SetDefaultDisappearingTimer sets the default disappearing timer for
	// new chats. An unsupported duration is ErrInvalidRecipient.
	SetDefaultDisappearingTimer(ctx context.Context, duration time.Duration) error
	// GetDisappearingTimer returns the disappearing timer of chatJID, with
	// found false when the chat carries no timer.
	GetDisappearingTimer(ctx context.Context, chatJID string) (duration time.Duration, found bool, err error)
	// GetStatusPrivacy returns the own status privacy settings.
	GetStatusPrivacy(ctx context.Context) (StatusPrivacy, error)
	// SubscribePresence subscribes to the presence of jid. A malformed jid
	// is ErrInvalidRecipient.
	SubscribePresence(ctx context.Context, jid string) error
	// HistorySyncSnapshot returns the accumulated history-sync feed of the
	// instance (progress, conversation batches, contacts). The Import plan
	// consumes it after pairing; each session accumulates only its own feed.
	HistorySyncSnapshot() HistorySyncSnapshot
	// ResetHistorySync clears the accumulated history-sync feed after a
	// successful import, so the next sync starts from zero.
	ResetHistorySync()
	// Disconnect asks WhatsApp to log the companion device out, then closes
	// the connection. A session that was never online or whose device is
	// already gone disconnects locally without failing.
	Disconnect(ctx context.Context) error
	// Status returns the current lifecycle state.
	Status() Status
	// JID returns the public WhatsApp JID, empty while pairing.
	JID() string
	// IsConnected reports whether the underlying websocket is currently
	// live. It is distinct from Status (and from the DB-persisted status):
	// the stored status can say "connected" while the socket is already
	// dead, and vice-versa while a reconnect is in flight.
	IsConnected() bool
}

// Manager owns the session of every instance.
type Manager interface {
	// RestoreAll reconnects the sessions persisted in the database, with
	// limited concurrency.
	RestoreAll(ctx context.Context) error
	// Get returns the session of instanceID when one exists.
	Get(instanceID uuid.UUID) (Session, bool)
	// Create returns the session of instance, building a new device when the
	// instance was never paired. Repeating it for a known instance returns the
	// existing session.
	Create(instance *model.Instance) (Session, error)
	// Remove tears the session down and deletes its stored credentials.
	Remove(ctx context.Context, instanceID uuid.UUID) error
}
