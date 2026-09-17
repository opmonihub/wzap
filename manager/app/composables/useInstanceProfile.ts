import type {
  PairPhoneResult,
  Privacy,
  Profile,
  ProfilePhotoResult,
  RejectCallInput,
  RejectCallResult,
  UpdateProfileInput
} from '~/types/api'

// Typed client for profile, privacy, pair-phone and call reject. Name/recado
// limits mirror profile.go (name trimmed 1..100, recado 0..500 with empty
// clearing). Photo is an octet-stream PUT with Content-Type image/*. Pairing
// codes require a prior Connect channel (409 otherwise); the phone number is
// never logged. Name/photo and reject-call may answer 501 not_supported,
// which callers surface as an explicit unsupported notice.
export function useInstanceProfile() {
  const { api } = useApi()

  async function getProfile(instanceId: string): Promise<Profile> {
    return await api<Profile>(`/instances/${instanceId}/profile`)
  }

  async function updateProfile(
    instanceId: string,
    input: UpdateProfileInput
  ): Promise<Profile> {
    return await api<Profile>(`/instances/${instanceId}/profile`, { method: 'PATCH', body: input })
  }

  async function setProfilePhoto(instanceId: string, file: File): Promise<ProfilePhotoResult> {
    return await api<ProfilePhotoResult>(`/instances/${instanceId}/profile/photo`, {
      method: 'PUT',
      headers: { 'Content-Type': file.type || 'image/jpeg' },
      body: file
    })
  }

  async function getPrivacy(instanceId: string): Promise<Privacy> {
    return await api<Privacy>(`/instances/${instanceId}/privacy`)
  }

  async function setPrivacy(instanceId: string, input: Partial<Privacy>): Promise<Privacy> {
    return await api<Privacy>(`/instances/${instanceId}/privacy`, { method: 'PUT', body: input })
  }

  async function pairPhone(instanceId: string, phone: string): Promise<PairPhoneResult> {
    return await api<PairPhoneResult>(`/instances/${instanceId}/pair-phone`, {
      method: 'POST',
      body: { phone: phone.trim() }
    })
  }

  async function rejectCall(instanceId: string, input: RejectCallInput): Promise<RejectCallResult> {
    return await api<RejectCallResult>(`/instances/${instanceId}/calls/reject`, {
      method: 'POST',
      body: { call_id: input.call_id, from: input.from }
    })
  }

  return { getProfile, updateProfile, setProfilePhoto, getPrivacy, setPrivacy, pairPhone, rejectCall }
}
