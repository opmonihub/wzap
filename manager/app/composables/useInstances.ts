import type {
  AccountUserList,
  ConnectResult,
  ConnectionStatus,
  CreatedInstance,
  CreateInstanceInput,
  Instance,
  InstanceEnvelope,
  InstanceList,
  RotatedInstanceKey,
  UpdateInstanceInput,
  UpdateWebhookInput
} from '~/types/api'

// keySeenStorageKey remembers whether the operator acknowledged a freshly
// rotated key in this browser. It is not an API "has key" indicator.
function keySeenStorageKey(id: string): string {
  return `wzap.manager.keySeen.${id}`
}

export function hasSeenInstanceKey(id: string): boolean {
  try {
    return localStorage.getItem(keySeenStorageKey(id)) === '1'
  } catch {
    return false
  }
}

export function markInstanceKeySeen(id: string): void {
  try {
    localStorage.setItem(keySeenStorageKey(id), '1')
  } catch {
    // Private browsing or denied storage must not break the flow.
  }
}

export function forgetInstanceKeySeen(id: string): void {
  try {
    localStorage.removeItem(keySeenStorageKey(id))
  } catch {
    // Private browsing or denied storage must not break the flow.
  }
}

// Typed client for the instance and instance-key endpoints. Every call runs
// in the session scope (the wzap_session cookie travels automatically);
// failures throw ApiError with the envelope code (quota_exceeded, conflict,
// forbidden) so screens can render scoped messages.
export function useInstances() {
  const { api, raw } = useApi()

  async function listInstances(): Promise<InstanceList> {
    return await api<InstanceList>('/instances')
  }

  // Single reads/writes nest the instance under data.instance.
  async function getInstance(id: string): Promise<Instance> {
    return (await api<InstanceEnvelope>(`/instances/${id}`)).instance
  }

  // POST /instances answers 201 with data.instance plus the one-time
  // plaintext key as a sibling field.
  async function createInstance(input: CreateInstanceInput): Promise<CreatedInstance> {
    const body: Record<string, unknown> = { name: input.name }
    const externalRef = input.external_ref?.trim() ?? ''
    if (externalRef !== '') {
      body.external_ref = externalRef
    }
    if (input.webhook !== undefined) {
      body.webhook = input.webhook
    }
    return await api<CreatedInstance>('/instances', { method: 'POST', body })
  }

  async function updateInstance(id: string, input: UpdateInstanceInput): Promise<Instance> {
    return (await api<InstanceEnvelope>(`/instances/${id}`, {
      method: 'PATCH',
      body: input
    })).instance
  }

  // PATCH /instances/{id} with the webhook block only. Name and external_ref
  // stay omitted so the stored values are kept. Anyone operating the instance
  // (global, owning user, own instance key) may call it; a 422 ApiError
  // carries the server validation message verbatim for the UI to mirror.
  async function updateInstanceWebhook(id: string, input: UpdateWebhookInput): Promise<Instance> {
    return (await api<InstanceEnvelope>(`/instances/${id}`, {
      method: 'PATCH',
      body: {
        webhook: {
          url: input.url,
          enabled: input.enabled,
          events: input.events
        }
      }
    })).instance
  }

  // DELETE answers 204 with no envelope, so it goes through the raw client.
  async function deleteInstance(id: string): Promise<void> {
    await raw(`/instances/${id}`, { method: 'DELETE' })
  }

  // POST answers 204 with no envelope, so it goes through the raw client.
  async function disconnectInstance(id: string): Promise<void> {
    await raw(`/instances/${id}/disconnect`, { method: 'POST' })
  }

  // POST /instances/{id}/connect starts pairing and answers data.connection
  // with the QR payload and its validity, or the connected status with no QR
  // when the instance needs no pairing (stored credentials or an
  // already-open session).
  async function connectInstance(id: string): Promise<ConnectResult> {
    return await api<ConnectResult>(`/instances/${id}/connect`, { method: 'POST' })
  }

  // GET /instances/{id}/qr returns the current pairing QR under
  // data.connection, starting a new pairing when none is active so an
  // expired code is replaced. It throws a 409 ApiError when the instance is
  // already connected (nothing to scan).
  async function getPairingQR(id: string): Promise<ConnectResult> {
    return await api<ConnectResult>(`/instances/${id}/qr`)
  }

  // GET /instances/{id}/status reports the connection block without touching
  // the session; the pairing card polls it while a QR is on screen.
  async function getConnectionStatus(id: string): Promise<ConnectionStatus> {
    return await api<ConnectionStatus>(`/instances/${id}/status`)
  }

  // Rotate answers 200 with the fresh one-time plaintext key. Only the
  // global scope and admin sessions may call it; user sessions get 403 and
  // the UI keeps this action absent for them.
  async function rotateInstanceKey(id: string): Promise<RotatedInstanceKey> {
    return await api<RotatedInstanceKey>(`/instances/${id}/apikey/rotate`, { method: 'POST' })
  }

  // DELETE answers 204 with no envelope, so it goes through the raw client.
  // Revoking an already-keyless instance still succeeds.
  async function revokeInstanceKey(id: string): Promise<void> {
    await raw(`/instances/${id}/apikey`, { method: 'DELETE' })
  }

  // Admin-only account listing under data.items[].user, used to resolve the
  // owner column. It throws 403 for user sessions; callers gate it behind
  // isAdmin.
  async function listAccounts(): Promise<AccountUserList> {
    return await api<AccountUserList>('/users')
  }

  return {
    listInstances,
    getInstance,
    createInstance,
    updateInstance,
    updateInstanceWebhook,
    deleteInstance,
    disconnectInstance,
    connectInstance,
    getPairingQR,
    getConnectionStatus,
    rotateInstanceKey,
    revokeInstanceKey,
    listAccounts
  }
}
