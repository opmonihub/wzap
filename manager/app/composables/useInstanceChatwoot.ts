import { ApiError } from '~/composables/useApi'
import type { ChatwootConfig, ChatwootImportResult } from '~/types/api'

// Typed client for the Chatwoot connector. PUT sends the full object; GET
// masks the token at display time (never log it). The global gate answers
// 400 chatwoot_disabled; import answers 202 {"imported":N} and may carry a
// partial count alongside the error, which the caller shows together.
export function useInstanceChatwoot() {
  const { api } = useApi()

  function isChatwootDisabled(error: unknown): boolean {
    return error instanceof ApiError && error.status === 400 && error.code === 'chatwoot_disabled'
  }

  async function getChatwoot(instanceId: string): Promise<ChatwootConfig> {
    return await api<ChatwootConfig>(`/instances/${instanceId}/chatwoot`)
  }

  async function setChatwoot(instanceId: string, input: Partial<ChatwootConfig>): Promise<ChatwootConfig> {
    return await api<ChatwootConfig>(`/instances/${instanceId}/chatwoot`, { method: 'PUT', body: input })
  }

  async function importHistory(instanceId: string): Promise<ChatwootImportResult> {
    return await api<ChatwootImportResult>(`/instances/${instanceId}/chatwoot/import`, { method: 'POST' })
  }

  async function sendCommand(instanceId: string, command: string, conversationId: number): Promise<{ handled: number }> {
    return await api<{ handled: number }>(`/instances/${instanceId}/chatwoot/command`, {
      method: 'POST',
      body: { command, conversation_id: conversationId }
    })
  }

  return { getChatwoot, setChatwoot, importHistory, sendCommand, isChatwootDisabled }
}
