import { ApiError } from '~/composables/useApi'
import type { ChatwootCommandResult, ChatwootConfig, ChatwootConfigEnvelope, ChatwootImportResult, ChatwootSetInput } from '~/types/api'

// Typed client for the Chatwoot connector. GET/PUT answer under
// data.chatwoot_config; PUT sends the full object with the write-only token
// (absent from every response — never log it). The global gate answers 400
// chatwoot_disabled; import answers 202 {"imported":N} and may carry a
// partial count alongside the error, which the caller shows together.
export function useInstanceChatwoot() {
  const { api } = useApi()

  function isChatwootDisabled(error: unknown): boolean {
    return error instanceof ApiError && error.status === 400 && error.code === 'chatwoot_disabled'
  }

  async function getChatwoot(instanceId: string): Promise<ChatwootConfig> {
    return (await api<ChatwootConfigEnvelope>(`/instances/${instanceId}/chatwoot`)).chatwoot_config
  }

  async function setChatwoot(instanceId: string, input: ChatwootSetInput): Promise<ChatwootConfig> {
    return (await api<ChatwootConfigEnvelope>(`/instances/${instanceId}/chatwoot`, { method: 'PUT', body: input })).chatwoot_config
  }

  async function importHistory(instanceId: string): Promise<ChatwootImportResult> {
    return await api<ChatwootImportResult>(`/instances/${instanceId}/chatwoot/import`, { method: 'POST' })
  }

  async function sendCommand(instanceId: string, command: string, conversationId: number): Promise<ChatwootCommandResult> {
    return await api<ChatwootCommandResult>(`/instances/${instanceId}/chatwoot/command`, {
      method: 'POST',
      body: { command, conversation_id: conversationId }
    })
  }

  return { getChatwoot, setChatwoot, importHistory, sendCommand, isChatwootDisabled }
}
