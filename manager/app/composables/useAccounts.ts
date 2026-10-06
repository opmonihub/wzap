import type { AccountUser, AccountUserEnvelope, AccountUserList, CreateUserInput } from '~/types/api'

// Typed client for the admin-only users endpoints. Every call runs in the
// session scope (the wzap_session cookie travels automatically); user
// sessions get 403 and the UI keeps these screens absent for them. Failures
// throw ApiError with the envelope code (conflict on duplicate email or on
// deleting an owner with instances, unprocessable_entity on invalid fields)
// so screens can render scoped messages. Single reads/writes nest the user
// under data.user; the collection nests each element under items[].user.
export function useAccounts() {
  const { api, raw } = useApi()

  async function listUsers(): Promise<AccountUser[]> {
    const page = await api<AccountUserList>('/users')
    return page.items.map(item => item.user)
  }

  // POST /users answers 201 with the created user under data.user. An
  // omitted instance_limit applies the server default; 0 means unlimited.
  async function createUser(input: CreateUserInput): Promise<AccountUser> {
    const body: Record<string, unknown> = {
      email: input.email,
      password: input.password,
      role: input.role
    }
    if (input.instance_limit !== undefined) {
      body.instance_limit = input.instance_limit
    }
    return (await api<AccountUserEnvelope>('/users', { method: 'POST', body })).user
  }

  // DELETE answers 204 with no envelope, so it goes through the raw client.
  // Deleting an owner that still owns instances answers 409 and removes
  // nothing; there is no transfer and no cascade.
  async function deleteUser(id: string): Promise<void> {
    await raw(`/users/${id}`, { method: 'DELETE' })
  }

  // PATCH /users/{id} edits the per-user instance limit and answers 200 with
  // the updated user under data.user. 0 means unlimited.
  async function updateUserQuota(id: string, quota: number): Promise<AccountUser> {
    return (await api<AccountUserEnvelope>(`/users/${id}`, {
      method: 'PATCH',
      body: { instance_limit: quota }
    })).user
  }

  return {
    listUsers,
    createUser,
    deleteUser,
    updateUserQuota
  }
}
