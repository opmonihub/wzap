// Shared shapes of the Go API contract. Success answers { data }, failures
// answer { error: { code, message } }; every response echoes X-Request-Id.

// Success envelope wrapping every 2xx JSON payload.
export interface ApiEnvelope<T> {
  data: T
}

// Error envelope returned for 4xx/5xx JSON responses.
export interface ApiErrorEnvelope {
  error: {
    code: string
    message: string
  }
}

// Account role assigned at creation; admin sees everything, user sees only
// the instances the account owns.
export type AccountRole = 'admin' | 'user'

// Identity answered by POST /auth/login and GET /auth/me.
export interface SessionUser {
  id: string
  email: string
  role: AccountRole
}

// Vision scope derived from the role: admin gets the operator-wide vision,
// user is restricted to the account's own instances.
export type VisionScope = 'global' | 'instance'

// Connection state reported by the API for an instance.
export type InstanceStatus = 'disconnected' | 'pairing' | 'connected' | 'error'

// Instance as answered by GET /instances and GET /instances/{id}. The shape
// mirrors instanceResponse in internal/httpapi/instances.go: it carries the
// owner and webhook configuration on every read but never the instance API
// key, which is returned in clear exactly once by the create and rotate
// answers below.
export interface Instance {
  id: string
  name: string
  external_ref: string
  owner_user_id: string | null
  webhook_url: string | null
  webhook_enabled: boolean
  webhook_events: string[]
  status: InstanceStatus
  whatsapp_jid: string
  last_error: string
  last_connected_at: string | null
  created_at: string
  updated_at: string
}

// One page of GET /instances with the opaque cursor of the next page, empty
// on the last page.
export interface InstanceListPage {
  items: Instance[]
  next_cursor: string
}

// GET /instances/stats answer: the scoped total plus the breakdown by
// connection status. Mirrors instanceStatsResponse in
// internal/httpapi/instances.go: the four buckets (connected, disconnected,
// pairing, error) always sum to total — unknown statuses fold into
// disconnected server-side. Admin and global sessions count everything, user
// sessions count exactly their own rows, instance keys get 403.
export interface InstanceStats {
  total: number
  by_status: Record<string, number>
}

// POST /instances answer: the instance plus its one-time plaintext key. No
// other read route returns this shape.
export interface CreatedInstance extends Instance {
  instance_api_key: string
}

// POST /instances/{id}/apikey/rotate answer: the instance id with its fresh
// one-time plaintext key.
export interface RotatedInstanceKey {
  id: string
  instance_api_key: string
}

// POST /instances/{id}/connect and GET /instances/{id}/qr answer: the
// resulting status plus, while pairing, the QR payload and its validity.
// Mirrors connectResponse in internal/httpapi/connection.go: qr_code and
// qr_expires_at are absent when the instance is already connected, and GET
// qr answers 409 instead when there is no QR to scan.
export interface ConnectResult {
  status: InstanceStatus
  qr_code?: string
  qr_expires_at?: string | null
}

// GET /instances/{id}/status answer: the connection state of an instance.
// Mirrors statusResponse in internal/httpapi/connection.go.
export interface ConnectionStatus {
  status: InstanceStatus
  whatsapp_jid: string
  last_error: string
  last_connected_at: string | null
}

// Payload sent to POST /instances. owner_user_id stays unset here: only the
// global scope and admin sessions may send it, and the console always creates
// for the signed-in account (user sessions own what they create, admin
// creations without owner fall back to the oldest admin server-side).
export interface CreateInstanceInput {
  name: string
  external_ref?: string
}

// Payload sent to PATCH /instances/{id}. An explicit empty external_ref
// clears the stored reference.
export interface UpdateInstanceInput {
  name: string
  external_ref: string
}

// Canonical webhook event types in the stored order. Mirrors canonicalEvents
// in internal/webhook/webhook.go: matching is case-sensitive and an explicit
// empty list stays empty (it delivers nothing).
export const WEBHOOK_EVENT_TYPES = ['message', 'receipt', 'connection', 'message.status'] as const

// One subscribed webhook event type.
export type WebhookEventType = typeof WEBHOOK_EVENT_TYPES[number]

// Payload sent to PATCH /instances/{id} for webhook-only edits. The name and
// external_ref fields stay omitted so the stored values are kept; an explicit
// empty webhook_url unsets the webhook and an explicit empty webhook_events
// list clears the subscription. A 422 ApiError carries the server validation
// message (unknown type, bad URL, non-loopback http).
export interface UpdateWebhookInput {
  webhook_url: string
  webhook_enabled: boolean
  webhook_events: string[]
}

// Payload sent to POST /users (admin scope only). An omitted instance_quota
// applies the server default; 0 means unlimited.
export interface CreateUserInput {
  email: string
  password: string
  role: AccountRole
  instance_quota?: number
}

// Account as answered by GET /users (admin scope only). Used to resolve the
// owner column of the instance list.
export interface AccountUser {
  id: string
  email: string
  role: AccountRole
  instance_quota: number
}

// Delivery state reported by the API for an outbound message: queued is the
// initial state answered with 202, sending means a worker claimed it, sent
// and failed are terminal. Mirrors the message package statuses.
export type MessageStatus = 'queued' | 'sending' | 'sent' | 'failed'

// Outbound message as answered by GET /instances/{id}/messages/{message_id}
// and by the list below. Mirrors messageResponse in
// internal/httpapi/messages.go: recipient carries the resolved WhatsApp JID,
// whatsapp_message_id is empty until the message reaches sent.
export interface OutboundMessage {
  id: string
  instance_id: string
  type: string
  recipient: string
  status: MessageStatus
  whatsapp_message_id: string
  last_error: string
  attempts: number
  delivered_at: string | null
  read_at: string | null
  created_at: string
  updated_at: string
}

// One page of GET /instances/{id}/messages with the opaque cursor of the next
// page, empty on the last page.
export interface MessageListPage {
  items: OutboundMessage[]
  next_cursor: string
}

// 202 answer of the four send endpoints. The message starts queued and moves
// to sent or failed; the detail screen polls the message until it settles.
export interface AcceptedMessage {
  message_id: string
  status: string
}

// POST /instances/{id}/numbers/check answer: exists is false with an empty
// jid when the number is malformed or absent from WhatsApp. The check never
// enqueues anything.
export interface NumberCheckResult {
  exists: boolean
  jid: string
  normalized: string
}

// Media kind declared on a multipart upload. It must match the uploaded
// content type or the server answers 422; the console derives it from the
// file MIME with the same mapping as media.Kind.
export type MediaKind = 'image' | 'video' | 'audio' | 'document'

// Payload for POST /instances/{id}/messages/media (multipart/form-data).
export interface SendMediaInput {
  to: string
  type: MediaKind
  caption?: string
  filename?: string
  ptt?: boolean
  file: File
}

// Group member as answered by group reads.
export interface GroupParticipant {
  jid: string
  is_admin: boolean
  is_super_admin: boolean
}

// Group as answered by create/get/update (invite_code only on create).
export interface Group {
  jid: string
  name: string
  description?: string
  participants: GroupParticipant[]
  participant_count: number
  invite_code?: string
  updated_at: string
}

export interface CreateGroupInput {
  name: string
  participants?: string[]
}

export interface UpdateGroupInput {
  name?: string
  description?: string
}

export interface GroupInvite {
  invite_code: string
}

export interface GroupParticipantsInput {
  action: 'add' | 'remove' | 'promote' | 'demote'
  participants: string[]
}

export interface JoinGroupInput {
  invite_code: string
}

// 200 answers of the group invite/participants/join/leave routes. Join
// answers the entered group JID; leave and picture/participant updates
// answer a boolean flag. Mirrors groupInviteResponse, groupJoinResponse,
// groupLeaveResponse and groupUpdatedResponse in internal/httpapi/groups.go.
export interface GroupJoinResult {
  jid: string
}

export interface GroupLeaveResult {
  left: boolean
}

export interface GroupUpdatedResult {
  updated: boolean
}

// Newsletter channel as answered by follow/unfollow/get/list.
export interface Newsletter {
  channel: string
  title: string
  description?: string
  follower_count: number
  updated_at: string
}

export interface NewsletterListPage {
  items: Newsletter[]
  next_cursor: string
}

export interface NewsletterFollowInput {
  channel: string
}

// 200 answer of the newsletter follow/unfollow routes: followed is true
// after a follow, false after an unfollow. Mirrors
// newsletterFollowResponse in internal/httpapi/newsletters.go.
export interface NewsletterFollowResult {
  followed: boolean
}

// Own status entry of GET status/updates.
export interface OwnStatus {
  id: string
  type: string
  text?: string
  caption?: string
  created_at: string
}

export interface StatusPublishResult {
  message_id: string
  status: string
}

// Payload for POST /instances/{id}/status/updates (text statuses only;
// image and video ride the multipart media route below). Mirrors
// publishStatusRequest in internal/httpapi/status.go: text carries 1..700
// characters.
export interface PublishStatusInput {
  type: 'text'
  text: string
}

// Payload for POST /instances/{id}/status/updates/media
// (multipart/form-data). Mirrors the media publish handler in
// internal/httpapi/status.go: kind is image|video and must match the file
// content type, caption caps at 700 characters.
export interface PublishStatusMediaInput {
  type: 'image' | 'video'
  caption?: string
  file: File
}

// 200 answer of GET /instances/{id}/status/updates. Items is always an
// array, empty when nothing was published since boot. Mirrors
// statusListResponse in internal/httpapi/status.go.
export interface StatusListPage {
  items: OwnStatus[]
}

// 200 answer of DELETE /instances/{id}/status/updates/{status_id}.
// Mirrors statusDeleteResponse in internal/httpapi/status.go.
export interface StatusDeleteResult {
  deleted: boolean
}

// Payload for PATCH /instances/{id}/profile: nil-equivalent (omitted)
// fields keep their upstream value, an explicit empty status_text clears
// the recado while an empty name is rejected (1..100). Mirrors
// updateProfileRequest in internal/httpapi/profile.go.
export interface UpdateProfileInput {
  name?: string
  status_text?: string
}

// 200 answer of PUT /instances/{id}/profile/photo (octet-stream upload).
// Mirrors profilePhotoResponse in internal/httpapi/profile.go.
export interface ProfilePhotoResult {
  updated: boolean
}

// Privacy field allowlists. Last seen, profile photo, status and groups
// add take the four-valued literals; read receipts take all|none. Mirrors
// validPrivacyValue/validReadReceiptsValue in internal/httpapi/profile.go.
export type PrivacyFieldValue = 'all' | 'contacts' | 'contact_blacklist' | 'none'

export type ReadReceiptsValue = 'all' | 'none'

// Payload for PUT /instances/{id}/privacy: at least one field must be
// present. Mirrors updatePrivacyRequest in internal/httpapi/profile.go.
export interface UpdatePrivacyInput {
  last_seen?: PrivacyFieldValue
  profile_photo?: PrivacyFieldValue
  status?: PrivacyFieldValue
  read_receipts?: ReadReceiptsValue
  groups_add?: PrivacyFieldValue
}

// Payload for POST /instances/{id}/pair-phone. Requires an open pairing
// channel from a prior Connect; without one the server answers 409.
// Mirrors pairPhoneRequest in internal/httpapi/pair_phone.go.
export interface PairPhoneInput {
  phone: string
}

// Own profile and privacy.
export interface Profile {
  name: string
  status_text: string
  photo_url: string
}

export interface Privacy {
  last_seen: string
  profile_photo: string
  status: string
  read_receipts: string
  groups_add: string
}

export interface PairPhoneResult {
  pairing_code: string
  expires_at: string
}

// Chatwoot connector (token write-only in UI: GET display masks it).
export interface ChatwootConfig {
  instance_id: string
  enabled: boolean
  url: string
  account_id: string
  token: string
  name_inbox: string
  sign_msg: boolean
  sign_delimiter: string
  reopen_conversation: boolean
  conversation_pending: boolean
  merge_brazil_contacts: boolean
  import_contacts: boolean
  import_messages: boolean
  days_limit: number
  auto_create: boolean
  organization: string
  logo: string
  ignore_jids: string[]
  webhook_url: string
}

export interface ChatwootImportResult {
  imported: number
}

// Payload for PUT /instances/{id}/chatwoot. Booleans are values (absent
// means false). Mirrors chatwootSetRequest in internal/httpapi/chatwoot.go.
// The token travels write-only: GET responses always mask it.
export interface ChatwootSetInput {
  enabled: boolean
  url: string
  account_id: string
  token: string
  name_inbox?: string
  sign_msg?: boolean
  sign_delimiter?: string
  reopen_conversation?: boolean
  conversation_pending?: boolean
  merge_brazil_contacts?: boolean
  import_contacts?: boolean
  import_messages?: boolean
  days_limit?: number
  auto_create?: boolean
  organization?: string
  logo?: string
  ignore_jids?: string[]
}

// Payload for POST /instances/{id}/chatwoot/command (status,
// init[:number], clearcache, disconnect). Mirrors chatwootCommandRequest
// in internal/httpapi/chatwoot.go.
export interface ChatwootCommandInput {
  command: string
  conversation_id: number
}

// 200 answer of the Chatwoot command route.
export interface ChatwootCommandResult {
  ok: boolean
}

// Rich send inputs for POST /messages plus lifecycle inputs.
export interface SendLocationInput {
  to: string
  latitude: number
  longitude: number
}

export interface SendContactInput {
  to: string
  display_name: string
  vcard: string
}

export interface SendListRow {
  id: string
  title: string
  description?: string
}

export interface SendListSection {
  title: string
  rows: SendListRow[]
}

export interface SendButton {
  id: string
  title: string
}

export interface SendRichInput {
  type: 'poll' | 'reaction' | 'list' | 'buttons'
  to: string
  question?: string
  options?: string[]
  selectable_count?: number
  target?: string
  emoji?: string
  title?: string
  description?: string
  button_text?: string
  sections?: SendListSection[]
  footer?: string
  text?: string
  buttons?: SendButton[]
}

export interface PresenceInput {
  chat: string
  state: 'composing' | 'paused' | 'available' | 'unavailable'
}

// 200 answer of POST /instances/{id}/presence: one call publishes one
// signal, there is no continuous mode. Mirrors presenceResponse in
// internal/httpapi/presence.go.
export interface PresenceResult {
  sent: boolean
}

export interface RevokeInput {
  chat: string
  message_id: string
}

// 200 answer of POST /instances/{id}/messages/revoke. Revoked is always
// true here: the protocol revoke is fire-and-forget, failures surface as
// 409/422 instead. Mirrors revokeResponse in internal/httpapi/lifecycle.go.
export interface RevokeResult {
  revoked: boolean
  reason?: string
}

export interface MarkReadInput {
  chat: string
  sender?: string
  message_id: string
}

// 200 answer of POST /instances/{id}/chats/mark-read. Mirrors
// markReadResponse in internal/httpapi/lifecycle.go.
export interface MarkReadResult {
  marked_read: boolean
}

export interface RejectCallInput {
  call_id: string
  from: string
}

// 200 answer of POST /instances/{id}/calls/reject. An upstream that cannot
// reject answers 501 not_supported. Mirrors rejectCallResponse in
// internal/httpapi/calls.go.
export interface RejectCallResult {
  rejected: boolean
}
