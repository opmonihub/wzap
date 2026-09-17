import type {
  Newsletter,
  NewsletterFollowResult,
  NewsletterListPage,
  StatusDeleteResult,
  StatusListPage,
  StatusPublishResult
} from '~/types/api'

// Typed client for newsletters (4 routes) and own statuses (4 routes).
// Text/caption validation mirrors status.go: trimmed 1..700 characters;
// media kind is image|video only. Publish is fire-and-forget (no outbox
// retry); the 202 answer carries message_id with idempotency replay.
export function useInstanceChannels() {
  const { api } = useApi()

  async function followNewsletter(instanceId: string, channel: string): Promise<NewsletterFollowResult> {
    return await api<NewsletterFollowResult>(`/instances/${instanceId}/newsletters/follow`, {
      method: 'POST',
      body: { channel: channel.trim() }
    })
  }

  async function unfollowNewsletter(instanceId: string, channel: string): Promise<NewsletterFollowResult> {
    return await api<NewsletterFollowResult>(`/instances/${instanceId}/newsletters/unfollow`, {
      method: 'POST',
      body: { channel: channel.trim() }
    })
  }

  async function getNewsletter(instanceId: string, channel: string): Promise<Newsletter> {
    return await api<Newsletter>(`/instances/${instanceId}/newsletters/${encodeURIComponent(channel.trim())}`)
  }

  async function listNewsletters(instanceId: string, cursor?: string): Promise<NewsletterListPage> {
    const query = cursor ? { cursor } : {}
    return await api<NewsletterListPage>(`/instances/${instanceId}/newsletters`, { query })
  }

  async function publishTextStatus(instanceId: string, text: string): Promise<StatusPublishResult> {
    return await api<StatusPublishResult>(`/instances/${instanceId}/status/updates`, {
      method: 'POST',
      headers: { 'Idempotency-Key': newIdempotencyKey() },
      body: { type: 'text', text: text.trim() }
    })
  }

  async function publishMediaStatus(
    instanceId: string,
    kind: 'image' | 'video',
    file: File,
    caption?: string
  ): Promise<StatusPublishResult> {
    const form = new FormData()
    form.append('type', kind)
    if ((caption ?? '').trim() !== '') {
      form.append('caption', (caption ?? '').trim())
    }
    form.append('file', file, file.name)
    return await api<StatusPublishResult>(`/instances/${instanceId}/status/updates/media`, {
      method: 'POST',
      headers: { 'Idempotency-Key': newIdempotencyKey() },
      body: form
    })
  }

  async function listStatuses(instanceId: string): Promise<StatusListPage> {
    return await api<StatusListPage>(`/instances/${instanceId}/status/updates`)
  }

  async function deleteStatus(instanceId: string, statusId: string): Promise<StatusDeleteResult> {
    return await api<StatusDeleteResult>(`/instances/${instanceId}/status/updates/${encodeURIComponent(statusId)}`, { method: 'DELETE' })
  }

  return {
    followNewsletter,
    unfollowNewsletter,
    getNewsletter,
    listNewsletters,
    publishTextStatus,
    publishMediaStatus,
    listStatuses,
    deleteStatus
  }
}

function newIdempotencyKey(): string {
  try {
    return crypto.randomUUID()
  } catch {
    return `${Date.now()}-${Math.floor(Math.random() * 0x100000000).toString(16)}`
  }
}
