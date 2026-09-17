<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import CreateInstanceModal from '~/components/instances/CreateInstanceModal.vue'
import DeleteInstanceModal from '~/components/instances/DeleteInstanceModal.vue'
import EditInstanceModal from '~/components/instances/EditInstanceModal.vue'
import type { CreatedInstance, Instance } from '~/types/api'

// Instance list orchestration only: fetch/cursor state, create/connect/edit/
// delete flows and modal targets. Table state, toolbar, columns, selection
// and viewport rules live in InstancesTable; cell rendering lives in
// InstancesTable*Cell.
const { t } = useI18n()
const toast = useToast()
const { isAdmin } = useAuth()
const { listInstances, listAccounts, connectInstance } = useInstances()

const items = ref<Instance[]>([])
const nextCursor = ref('')
const pending = ref(true)
const loadingMore = ref(false)
const failure = ref<string | null>(null)
const createOpen = ref(false)
const ownerEmails = ref<Record<string, string>>({})

// Row actions: connect starts pairing inline (the QR itself lives on the
// detail screen), edit opens the inline rename modal, remove opens the typed
// delete confirmation. All API work stays in these handlers; the actions cell
// only emits.
const connectingId = ref<string | null>(null)
const editTarget = ref<Instance | null>(null)
const editOpen = ref(false)
const deleteTarget = ref<Instance | null>(null)
const deleteOpen = ref(false)

useSeoMeta({
  title: 'Instances'
})

async function loadOwners() {
  if (!isAdmin.value) {
    return
  }
  try {
    const accounts = await listAccounts()
    ownerEmails.value = Object.fromEntries(accounts.map(account => [account.id, account.email]))
  } catch {
    // Owner resolution is best-effort: the list still renders with short ids.
  }
}

async function loadFirst() {
  pending.value = true
  failure.value = null
  try {
    const page = await listInstances()
    items.value = page.items
    nextCursor.value = page.next_cursor
    await loadOwners()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.loadFailed')
  } finally {
    pending.value = false
  }
}

async function loadMore() {
  if (loadingMore.value || nextCursor.value === '') {
    return
  }
  loadingMore.value = true
  try {
    const page = await listInstances(nextCursor.value)
    items.value = [...items.value, ...page.items]
    nextCursor.value = page.next_cursor
  } catch (error) {
    toast.add({
      title: error instanceof ApiError ? error.message : t('instances.loadFailed'),
      icon: 'i-lucide-triangle-alert',
      color: 'error'
    })
  } finally {
    loadingMore.value = false
  }
}

function onCreated(instance: CreatedInstance) {
  // The one-time key lives in the modal only: strip it before the created
  // instance joins the list so it is never retained in list memory.
  const { instance_api_key: _omit, ...rest } = instance
  items.value = [rest, ...items.value]
  toast.add({ title: t('instances.create.createdToast'), icon: 'i-lucide-check', color: 'success' })
}

async function onConnect(instance: Instance) {
  if (connectingId.value) {
    return
  }
  connectingId.value = instance.id
  try {
    const result = await connectInstance(instance.id)
    items.value = items.value.map(entry =>
      entry.id === instance.id ? { ...entry, status: result.status } : entry
    )
    if (result.status === 'connected' || !result.qr_code) {
      toast.add({ title: t('instances.connect.alreadyConnected'), icon: 'i-lucide-check', color: 'success' })
      return
    }
    toast.add({ title: t('instances.connect.pairingStarted'), icon: 'i-lucide-check', color: 'success' })
    await navigateTo(`/instances/${instance.id}`)
  } catch (error) {
    toast.add({
      title: error instanceof ApiError ? error.message : t('instances.connect.failed'),
      icon: 'i-lucide-triangle-alert',
      color: 'error'
    })
  } finally {
    connectingId.value = null
  }
}

function onUpdated(updated: Instance) {
  items.value = items.value.map(entry => entry.id === updated.id ? updated : entry)
  editTarget.value = null
  toast.add({ title: t('instances.edit.saved'), icon: 'i-lucide-check', color: 'success' })
}

function onDeleted(id: string) {
  items.value = items.value.filter(entry => entry.id !== id)
  deleteTarget.value = null
  toast.add({ title: t('instances.detail.deleted'), icon: 'i-lucide-check', color: 'success' })
}

await loadFirst()
</script>

<template>
  <UDashboardPanel id="instances">
    <template #header>
      <UDashboardNavbar :title="t('instances.title')">
        <template #leading>
          <UDashboardSidebarCollapse />
        </template>
        <template #right>
          <UButton icon="i-lucide-plus" :label="t('instances.create.title')" @click="createOpen = true" />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <p class="mb-4 text-sm text-muted">
        {{ isAdmin ? t('instances.subtitleAdmin') : t('instances.subtitleUser') }}
      </p>

      <PageState :pending="pending" :error="failure" @retry="loadFirst">
        <InstancesTable
          :items="items"
          :owner-emails="ownerEmails"
          :is-admin="isAdmin"
          :loading-more="loadingMore"
          :has-more="nextCursor !== ''"
          @connect="onConnect"
          @edit="(instance: Instance) => { editTarget = instance; editOpen = true }"
          @remove="(instance: Instance) => { deleteTarget = instance; deleteOpen = true }"
          @load-more="loadMore"
          @create="createOpen = true"
        />
      </PageState>
    </template>
  </UDashboardPanel>

  <CreateInstanceModal v-model:open="createOpen" @created="onCreated" />

  <EditInstanceModal v-model:open="editOpen" :target="editTarget" @updated="onUpdated" />

  <DeleteInstanceModal
    v-if="deleteTarget"
    v-model:open="deleteOpen"
    :instance="deleteTarget"
    @deleted="onDeleted"
  />
</template>
