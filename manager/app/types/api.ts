// Shared shapes of the Go API contract after the storage/JSON remodel.
// Success answers { data }, failures answer { error: { code, message } };
// every response echoes X-Request-Id. Persisted resources travel under
// data.<entity> on single reads/writes and under data.items[].<entity> on
// collections; commands keep their flat result objects (response-matrix
// rules §1–§7).

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

// Identity answered by POST /auth/login and GET /auth/me under data.me.
export interface SessionUser {
  id: string
  email: string
  role: AccountRole
}

// Wrapper of the session identity routes.
export interface MeEnvelope {
  me: SessionUser
}

// Vision scope derived from the role: admin gets the operator-wide vision,
// user is restricted to the account's own instances.
export type VisionScope = 'global' | 'instance'

// Connection state reported by the API for an instance.
export type InstanceStatus = 'disconnected' | 'pairing' | 'connected' | 'error'

// Structured failure of a connection or a send: code from the closed
// catalog (legacy_error for migrated free text), the human-readable message
// and occurred_at (null for legacy errors whose time is unknown).
export interface LastError {
  code: string
  message: string
  occurred_at: string | null
}

// Public connection block of an instance and of the connection-scoped
// routes. QR fields only appear while a pairing is in progress.
export interface ConnectionInfo {
  status: InstanceStatus
  last_error: LastError | null
  last_connected_at: string | null
  qr_code?: string
  qr_expires_at?: string | null
}

// Pairing subset answered by POST connect and GET qr: status plus the QR
// pair only while pairing is in progress.
export interface PairingConnection {
  status: InstanceStatus
  qr_code?: string
  qr_expires_at?: string | null
}

// Public webhook block of an instance: enabled flag, optional URL and the
// subscribed event types (always an array).
export interface WebhookConfig {
  enabled: boolean
  url: string | null
  events: string[]
}

// Instance as answered by GET /instances and GET /instances/{id} under
// data.instance, and nested under data.items[].instance in the collection.
// Mirrors instanceResponse in internal/httpapi/dto.go: identity plus the
// nested connection and webhook blocks and the timestamps. Internal fields
// (device_jid, external_ref, owner_user_id, key hashes) never leave the
// service; the instance API key is returned in clear exactly once by the
// create and rotate answers below.
export interface Instance {
  id: string
  name: string
  connection: ConnectionInfo
  webhook: WebhookConfig
  created_at: string
  updated_at: string
}

// Wrapper of a single instance answer.
export interface InstanceEnvelope {
  instance: Instance
}

// Complete collection of instances authorized for GET /instances: every
// element nests the instance under its own key.
export interface InstanceList {
  items: InstanceEnvelope[]
}

// GET /instances/stats answer under data.stats: the scoped total plus the
// breakdown by connection status. The four buckets (connected,
// disconnected, pairing, error) always sum to total — unknown statuses fold
// into disconnected server-side. Admin and global sessions count
// everything, user sessions count exactly their own rows, instance keys get
// 403.
export interface InstanceStats {
  total: number
  by_status: Record<string, number>
}

export interface StatsEnvelope {
  stats: InstanceStats
}

// POST /instances answer: the instance plus its one-time plaintext key. No
// other read route returns this shape.
export interface CreatedInstance {
  instance: Instance
  instance_api_key: string
}

// POST /instances/{id}/apikey/rotate answer: the instance id with its fresh
// one-time plaintext key.
export interface RotatedInstanceKey {
  id: string
  instance_api_key: string
}

// POST /instances/{id}/connect and GET /instances/{id}/qr answer under
// data.connection: the pairing subset — status plus, while pairing, the QR
// payload and its validity. GET qr answers 409 instead when there is no QR
// to scan.
export interface ConnectResult {
  connection: PairingConnection
}

// GET /instances/{id}/status answer: the full connection block of the
// instance (whatsapp_jid was removed from the public read).
export interface ConnectionStatus {
  connection: ConnectionInfo
}

// Webhook block of the create/PATCH instance payloads, mirroring the public
// instance.webhook sub-object. Omitted fields keep their stored value: a
// missing url stays unset, url:"" clears it and events:[] clears the
// subscription.
export interface WebhookInput {
  url?: string
  enabled?: boolean
  events?: string[]
}

// Payload sent to POST /instances. owner_user_id stays unset here: only the
// global scope and admin sessions may send it, and the console always creates
// for the signed-in account (user sessions own what they create, admin
// creations without owner fall back to the oldest admin server-side). An
// omitted webhook keeps every default; an omitted events field defaults to
// every type.
export interface CreateInstanceInput {
  name: string
  external_ref?: string
  webhook?: WebhookInput
}

// Payload sent to PATCH /instances/{id}. Omitted fields keep the stored
// value; an explicit empty external_ref clears the reference, an explicit
// empty webhook.url unsets the webhook and an explicit empty events list
// clears the subscription.
export interface UpdateInstanceInput {
  name?: string
  external_ref?: string
  webhook?: WebhookInput
}

// Canonical webhook event types in the stored order. Mirrors canonicalEvents
// in internal/webhook/webhook.go: matching is case-sensitive and an explicit
// empty list stays empty (it delivers nothing). The four first types are the
// defaults; the rich/group/call types are opt-in and MUST stay in the catalog
// so the webhook form renders and preserves subscriptions already configured
// outside the defaults instead of dropping them on save.
export const WEBHOOK_EVENT_TYPES = [
  'message',
  'receipt',
  'connection',
  'message.status',
  'poll.vote',
  'message.reaction',
  'interactive.response',
  'group.participants',
  'group.info',
  'call.offer'
] as const

// One subscribed webhook event type.
export type WebhookEventType = typeof WEBHOOK_EVENT_TYPES[number]

// Payload sent to PATCH /instances/{id} for webhook-only edits. The name and
// external_ref fields stay omitted so the stored values are kept; an explicit
// empty webhook.url unsets the webhook and an explicit empty webhook.events
// list clears the subscription. A 422 ApiError carries the server validation
// message (unknown type, bad URL, non-loopback http).
export interface UpdateWebhookInput {
  url: string
  enabled: boolean
  events: string[]
}

// Payload sent to POST /users (admin scope only). An omitted instance_limit
// applies the server default; 0 means unlimited.
export interface CreateUserInput {
  email: string
  password: string
  role: AccountRole
  instance_limit?: number
}

// Account as answered by GET /users (admin scope only), nested under
// data.items[].user in the collection and data.user on single reads.
// instance_limit is the per-user cap (0 = unlimited); instances_used is the
// backend-computed count of owned instances.
export interface AccountUser {
  id: string
  email: string
  role: AccountRole
  instance_limit: number
  instances_used: number
  created_at: string
  updated_at: string
}

export interface AccountUserEnvelope {
  user: AccountUser
}

export interface AccountUserList {
  items: AccountUserEnvelope[]
}

// Delivery state reported by the API for an outbound message: queued is the
// initial state answered with 202, sending means a worker claimed it, sent
// and failed are terminal. Mirrors the message package statuses.
export type MessageStatus = 'queued' | 'sending' | 'sent' | 'failed'

// Outbound message as answered by GET /instances/{id}/messages/{message_id}
// under data.message and by the list below under data.items[].message.
// Mirrors messageResponse in internal/httpapi/dto.go: recipient_jid carries
// the resolved WhatsApp JID, wa_id is null until the upstream confirms and
// last_error is the structured failure or null.
export interface OutboundMessage {
  id: string
  instance_id: string
  message_type: string
  recipient_jid: string
  send_status: MessageStatus
  wa_id: string | null
  media_id: string | null
  retry_count: number
  last_error: LastError | null
  next_attempt_at: string | null
  delivered_at: string | null
  read_at: string | null
  created_at: string
  updated_at: string
}

export interface OutboundMessageEnvelope {
  message: OutboundMessage
}

// One page of GET /instances/{id}/messages with the opaque cursor of the next
// page, empty on the last page.
export interface MessageListPage {
  items: OutboundMessageEnvelope[]
  next_cursor: string
}

// 202 answer of the send endpoints: the queue message as known at accept
// time, nested under data.message — only its real fields, never fabricated
// zero values (media_id is null except on media uploads). Mirrors
// acceptedMessageResponse in internal/httpapi/dto.go. The detail screen polls
// the message route for the full DTO until it settles.
export interface AcceptedMessageBody {
  id: string
  instance_id: string
  send_status: MessageStatus
  media_id: string | null
}

export interface AcceptedMessage {
  message: AcceptedMessageBody
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

// Group as answered by create/get (invite_code only on create), nested under
// data.group on single reads and data.items[].group in collections.
export interface Group {
  jid: string
  name: string
  description?: string
  participants: GroupParticipant[]
  participant_count: number
  invite_code?: string
  updated_at: string
}

export interface GroupEnvelope {
  group: Group
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

// Newsletter channel as answered by follow/unfollow/get/list, nested under
// data.channel on single reads and data.items[].channel in collections.
export interface Newsletter {
  channel: string
  title: string
  description?: string
  follower_count: number
  updated_at: string
}

export interface NewsletterEnvelope {
  channel: Newsletter
}

export interface NewsletterListPage {
  items: NewsletterEnvelope[]
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

// Own status entry of GET status/updates under data.items[].
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

// Own profile and privacy (flat data objects, derived upstream reads).
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

// Chatwoot connector as answered by GET/PUT /instances/{id}/chatwoot under
// data.chatwoot_config. The token is write-only and absent from reads — the
// field does not exist in the response.
export interface ChatwootConfig {
  instance_id: string
  is_enabled: boolean
  url: string
  account_id: string
  inbox_name: string
  is_sign_enabled: boolean
  sign_delimiter: string
  is_reopen_enabled: boolean
  is_pending_enabled: boolean
  is_merge_enabled: boolean
  is_import_contacts: boolean
  is_import_messages: boolean
  import_days: number
  is_auto_create: boolean
  organization: string
  logo: string
  ignored_jids: string[]
  webhook_url: string
}

export interface ChatwootConfigEnvelope {
  chatwoot_config: ChatwootConfig
}

// 202 answer of POST /instances/{id}/chatwoot/import (flat command result:
// the count of messages already imported).
export interface ChatwootImportResult {
  imported: number
}

// Payload for PUT /instances/{id}/chatwoot. Booleans are values (absent
// means false). Mirrors chatwootSetRequest in internal/httpapi/chatwoot.go.
// The token travels write-only: it is accepted on PUT and never appears in
// GET or PUT responses.
export interface ChatwootSetInput {
  is_enabled: boolean
  url: string
  account_id: string
  token: string
  inbox_name?: string
  is_sign_enabled?: boolean
  sign_delimiter?: string
  is_reopen_enabled?: boolean
  is_pending_enabled?: boolean
  is_merge_enabled?: boolean
  is_import_contacts?: boolean
  is_import_messages?: boolean
  import_days?: number
  is_auto_create?: boolean
  organization?: string
  logo?: string
  ignored_jids?: string[]
}

// Payload for POST /instances/{id}/chatwoot/command (status,
// init[:number], clearcache, disconnect). Mirrors chatwootCommandRequest
// in internal/httpapi/chatwoot.go.
export interface ChatwootCommandInput {
  command: string
  conversation_id: number
}

// 200 answer of the Chatwoot command route (flat command result).
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
