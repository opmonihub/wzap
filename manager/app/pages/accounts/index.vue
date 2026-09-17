<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import type { AccountUser } from '~/types/api'

// Account management is admin-only: the sidebar hides this screen for user
// accounts and the guard below turns a direct visit back to the overview.
// Quotas are per user (0 means unlimited); deleting an owner that still owns
// instances answers 409 and removes nothing.
const { t } = useI18n()
const toast = useToast()
const { isAdmin, user: sessionUser } = useAuth()
const { listUsers, deleteUser } = useAccounts()
const { confirmDelete } = useConfirmDelete()
const { listInstances } = useInstances()

if (!isAdmin.value) {
  await navigateTo('/')
}

const users = ref<AccountUser[]>([])
const pending = ref(true)
const failure = ref<string | null>(null)
// Instances owned per account, counted client-side: GET /users exposes no
// usage field, so the quota cell pairs the stored quota with this count.
const usageByOwner = ref<Record<string, number>>({})

const accountsTable = useTemplateRef<{ clearSelection: () => void }>('accountsTable')

const createOpen = ref(false)

const quotaTarget = ref<AccountUser | null>(null)
const quotaOpen = ref(false)

const deleteTarget = ref<AccountUser | null>(null)
const deleteOpen = ref(false)

useSeoMeta({
  title: 'Accounts'
})

async function load() {
  pending.value = true
  failure.value = null
  try {
    users.value = await listUsers()
    await loadUsage()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('accounts.loadFailed')
  } finally {
    pending.value = false
  }
}

// Usage comes from the instance list (no usage field on GET /users):
// accumulate every cursor page best-effort and count owners. A failed usage
// load never fails the accounts list; cells fall back to 0 used.
async function loadUsage() {
  const counts: Record<string, number> = {}
  try {
    let cursor: string | undefined
    do {
      const page = await listInstances(cursor)
      for (const instance of page.items) {
        if (instance.owner_user_id) {
          counts[instance.owner_user_id] = (counts[instance.owner_user_id] ?? 0) + 1
        }
      }
      cursor = page.next_cursor === '' ? undefined : page.next_cursor
    } while (cursor !== undefined)
    usageByOwner.value = counts
  } catch {
    usageByOwner.value = counts
  }
}

function openDelete(user: AccountUser) {
  // Self-delete is blocked client-side (the row menu also disables it): an
  // admin removing their own account would lock themselves out.
  if (sessionUser.value && user.id === sessionUser.value.id) {
    toast.add({ title: t('accounts.delete.selfBlocked'), icon: 'i-lucide-triangle-alert', color: 'error' })
    return
  }
  deleteTarget.value = user
  deleteOpen.value = true
}

// Bulk delete runs behind the programmatic confirm like the detail-page
// confirms: the overlay resolves true only on the confirm button, then the
// loop runs here with the loading state on the bulk-bar button. The table
// emits the selected users and clears its selection through the exposed
// method once the run settles.
const bulkDeleting = ref(false)

async function openBulkDelete(targets: AccountUser[]) {
  if (bulkDeleting.value) {
    return
  }
  const confirmed = await confirmDelete({
    title: t('accounts.bulkDelete.title'),
    description: t('accounts.bulkDelete.body', { count: targets.length }),
    confirmLabel: t('accounts.bulkDelete.submit')
  })
  if (!confirmed) {
    return
  }
  await onBulkDelete(targets)
}

async function onBulkDelete(targets: AccountUser[]) {
  if (bulkDeleting.value) {
    return
  }
  const selfId = sessionUser.value?.id
  const eligible = targets.filter(user => user.id !== selfId)
  if (eligible.length === 0) {
    toast.add({ title: t('accounts.bulkDelete.selfSkipped'), icon: 'i-lucide-triangle-alert', color: 'warning' })
    return
  }
  bulkDeleting.value = true
  try {
    let deleted = 0
    let failed = 0
    for (const target of eligible) {
      try {
        await deleteUser(target.id)
        deleted += 1
      } catch {
        failed += 1
      }
    }
    if (failed === 0) {
      // Clean run: drop the rows locally without a reload.
      const removedIds = new Set(eligible.map(target => target.id))
      users.value = users.value.filter(user => !removedIds.has(user.id))
      toast.add({ title: t('accounts.bulkDelete.deleted', { count: deleted }), icon: 'i-lucide-check', color: 'success' })
    } else {
      // Partial run: reload so the rows reflect exactly what the server kept.
      await load()
      toast.add({ title: t('accounts.bulkDelete.partial', { deleted, failed }), icon: 'i-lucide-triangle-alert', color: 'warning' })
    }
    accountsTable.value?.clearSelection()
  } finally {
    bulkDeleting.value = false
  }
}

if (isAdmin.value) {
  await load()
}
</script>

<template>
  <UDashboardPanel id="accounts">
    <template #header>
      <UDashboardNavbar :title="t('accounts.title')">
        <template #leading>
          <UDashboardSidebarCollapse />
        </template>
        <template #right>
          <UButton icon="i-lucide-plus" :label="t('accounts.create.title')" @click="createOpen = true" />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <p class="mb-4 text-sm text-muted">
        {{ t('accounts.subtitle') }}
      </p>

      <PageState :pending="pending" :error="failure" @retry="load">
        <AccountsTable
          ref="accountsTable"
          :users="users"
          :usage-by-owner="usageByOwner"
          :session-user-id="sessionUser?.id"
          :bulk-deleting="bulkDeleting"
          @edit-quota="(user: AccountUser) => { quotaTarget = user; quotaOpen = true }"
          @remove="(user: AccountUser) => openDelete(user)"
          @bulk-delete="(selected: AccountUser[]) => openBulkDelete(selected)"
          @create="createOpen = true"
        />
      </PageState>
    </template>
  </UDashboardPanel>

  <CreateAccountModal v-model:open="createOpen" @created="(user: AccountUser) => { users = [user, ...users] }" />

  <EditQuotaModal v-model:open="quotaOpen" :target="quotaTarget" @updated="(user: AccountUser) => { users = users.map(entry => entry.id === user.id ? user : entry) }" />

  <DeleteAccountModal v-model:open="deleteOpen" :target="deleteTarget" @deleted="(id: string) => { users = users.filter(user => user.id !== id); deleteTarget = null }" />
</template>
