import type {
  CreateGroupInput,
  Group,
  GroupInvite,
  GroupJoinResult,
  GroupLeaveResult,
  GroupParticipantsInput,
  GroupUpdatedResult,
  JoinGroupInput,
  UpdateGroupInput
} from '~/types/api'

// Typed client for the 8 group routes. Name validation mirrors
// handleCreateGroup (trimmed, 1..25 runes); a 201 with an empty invite_code
// is partial (post-create invite lookup failed) and the caller reconciles via
// getInvite instead of retrying create, which would duplicate the group.
export function useInstanceGroups() {
  const { api } = useApi()

  async function createGroup(instanceId: string, input: CreateGroupInput): Promise<Group> {
    return await api<Group>(`/instances/${instanceId}/groups`, {
      method: 'POST',
      body: { name: input.name.trim(), participants: input.participants ?? [] }
    })
  }

  async function getGroup(instanceId: string, groupJid: string): Promise<Group> {
    return await api<Group>(`/instances/${instanceId}/groups/${encodeURIComponent(groupJid)}`)
  }

  async function updateGroup(instanceId: string, groupJid: string, input: UpdateGroupInput): Promise<Group> {
    return await api<Group>(`/instances/${instanceId}/groups/${encodeURIComponent(groupJid)}`, {
      method: 'PATCH',
      body: input
    })
  }

  // PUT octet-stream with Content-Type: image/*; mirrors handleSetGroupPhoto.
  async function setGroupPhoto(instanceId: string, groupJid: string, file: File): Promise<GroupUpdatedResult> {
    return await api<GroupUpdatedResult>(`/instances/${instanceId}/groups/${encodeURIComponent(groupJid)}/photo`, {
      method: 'PUT',
      headers: { 'Content-Type': file.type || 'image/jpeg' },
      body: file
    })
  }

  async function updateParticipants(
    instanceId: string,
    groupJid: string,
    input: GroupParticipantsInput
  ): Promise<GroupUpdatedResult> {
    return await api<GroupUpdatedResult>(
      `/instances/${instanceId}/groups/${encodeURIComponent(groupJid)}/participants`,
      { method: 'POST', body: { action: input.action, participants: input.participants } }
    )
  }

  async function getInvite(instanceId: string, groupJid: string): Promise<GroupInvite> {
    return await api<GroupInvite>(`/instances/${instanceId}/groups/${encodeURIComponent(groupJid)}/invite`)
  }

  async function resetInvite(instanceId: string, groupJid: string): Promise<GroupInvite> {
    return await api<GroupInvite>(
      `/instances/${instanceId}/groups/${encodeURIComponent(groupJid)}/invite/reset`,
      { method: 'POST' }
    )
  }

  async function joinGroup(instanceId: string, inviteCode: string): Promise<GroupJoinResult> {
    const body: JoinGroupInput = { invite_code: inviteCode.trim() }
    return await api<GroupJoinResult>(`/instances/${instanceId}/groups/join`, {
      method: 'POST',
      body
    })
  }

  async function leaveGroup(instanceId: string, groupJid: string): Promise<GroupLeaveResult> {
    return await api<GroupLeaveResult>(`/instances/${instanceId}/groups/${encodeURIComponent(groupJid)}/leave`, { method: 'POST' })
  }

  return { createGroup, getGroup, updateGroup, setGroupPhoto, updateParticipants, getInvite, resetInvite, joinGroup, leaveGroup }
}
