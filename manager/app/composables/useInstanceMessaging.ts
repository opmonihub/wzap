import type {
  AcceptedMessage,
  MarkReadInput,
  PresenceInput,
  RevokeInput,
  SendContactInput,
  SendLocationInput,
  SendRichInput
} from '~/types/api'

// Typed client for location/contact/rich sends plus revoke, mark-read,
// presence and media download. Mirrors useMessages.ts: session cookie travels
// automatically, failures throw ApiError, sends mint a fresh Idempotency-Key
// per click (reused keys across payloads answer 422).
export function useInstanceMessaging() {
  const { api, raw } = useApi()

  async function sendLocation(instanceId: string, input: SendLocationInput): Promise<AcceptedMessage> {
    return await api<AcceptedMessage>(`/instances/${instanceId}/messages/location`, {
      method: 'POST',
      headers: { 'Idempotency-Key': newIdempotencyKey() },
      body: { to: input.to, latitude: input.latitude, longitude: input.longitude }
    })
  }

  async function sendContact(instanceId: string, input: SendContactInput): Promise<AcceptedMessage> {
    return await api<AcceptedMessage>(`/instances/${instanceId}/messages/contact`, {
      method: 'POST',
      headers: { 'Idempotency-Key': newIdempotencyKey() },
      body: { to: input.to, display_name: input.display_name, vcard: input.vcard }
    })
  }

  async function sendRich(instanceId: string, input: SendRichInput): Promise<AcceptedMessage> {
    return await api<AcceptedMessage>(`/instances/${instanceId}/messages`, {
      method: 'POST',
      headers: { 'Idempotency-Key': newIdempotencyKey() },
      body: input
    })
  }

  async function revokeMessage(instanceId: string, input: RevokeInput): Promise<{ revoked: boolean }> {
    return await api<{ revoked: boolean }>(`/instances/${instanceId}/messages/revoke`, {
      method: 'POST',
      body: { chat: input.chat, message_id: input.message_id }
    })
  }

  async function markRead(instanceId: string, input: MarkReadInput): Promise<{ marked_read: boolean }> {
    return await api<{ marked_read: boolean }>(`/instances/${instanceId}/chats/mark-read`, {
      method: 'POST',
      body: { chat: input.chat, sender: input.sender ?? '', message_id: input.message_id }
    })
  }

  async function sendPresence(instanceId: string, input: PresenceInput): Promise<{ sent: boolean }> {
    return await api<{ sent: boolean }>(`/instances/${instanceId}/presence`, {
      method: 'POST',
      body: { chat: input.chat, state: input.state }
    })
  }

  // GET /media/{id} answers binary; fetch raw and hand the caller a blob URL.
  async function downloadMedia(mediaId: string): Promise<string> {
    const blob = await raw<Blob>(`/media/${mediaId}`, { responseType: 'blob' })
    return URL.createObjectURL(blob)
  }

  return { sendLocation, sendContact, sendRich, revokeMessage, markRead, sendPresence, downloadMedia }
}

function newIdempotencyKey(): string {
  try {
    return crypto.randomUUID()
  } catch {
    return `${Date.now()}-${Math.floor(Math.random() * 0x100000000).toString(16)}`
  }
}
