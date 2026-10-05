<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import CreateInstanceModal from '~/components/instances/CreateInstanceModal.vue'
import DeleteInstanceModal from '~/components/instances/DeleteInstanceModal.vue'
import EditInstanceModal from '~/components/instances/EditInstanceModal.vue'
import type { CreatedInstance, Instance } from '~/types/api'

// Instance list orchestration only: fetch state, the cards/table view
// preference, create/connect/edit/delete flows and modal targets. Table
// state, toolbar, columns, selection and viewport rules live in
// InstancesTable; the card grid lives in InstancesCards; cell rendering lives
// in InstancesTable*Cell.
const { t } = useI18n()
const toast = useToast()
const { isAdmin } = useAuth()
const { listInstances, listAccounts, connectInstance } = useInstances()

const items = ref<Instance[]>([])
const pending = ref(true)
const failure = ref<string | null>(null)
const createOpen = ref(false)
const ownerEmails = ref<Record<string, string>>({})

// View preference persisted in a cookie (default cards). Restored verbatim
// from the pre-extraction page: the toggle lives in the page header while
// each view owns its filter/pagination state.
const viewCookie = useCookie<'cards' | 'table'>('wzap-instances-view', { default: () => 'cards' })
const view = computed<'cards' | 'table'>({
  get: () => viewCookie.value === 'table' ? 'table' : 'cards',
  set: value => viewCookie.value = value
})

// Row actions: connect starts pairing inline (the QR itself lives on the
// detail screen), edit opens the inline rename modal, remove opens the typed
// delete confirmation. All API work stays in these handlers; the table cell
// and the cards only emit.
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
    const listing = await listInstances()
    items.value = listing.items
    await loadOwners()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.loadFailed')
  } finally {
    pending.value = false
  }
}

function onCreated(instance: CreatedInstance) {
  // The one-time key lives in the modal only: strip it before the created
  // instance joins the list so it is never retained in list memory.
  const { instance_api_key: _omit, ...rest } = instance
  items.value = [rest, ...items.value]
  toast.add({ title: t('instances.create.createdToast'), icon: 'i-lucide-check', color: 'success' })
}

function openDetails(instance: Instance) {
  void navigateTo(`/instances/${instance.id}`)
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

function openEdit(instance: Instance) {
  editTarget.value = instance
  editOpen.value = true
}

function onUpdated(updated: Instance) {
  items.value = items.value.map(entry => entry.id === updated.id ? updated : entry)
  editTarget.value = null
  toast.add({ title: t('instances.edit.saved'), icon: 'i-lucide-check', color: 'success' })
}

function openDelete(instance: Instance) {
  deleteTarget.value = instance
  deleteOpen.value = true
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

      <UDashboardToolbar>
        <template #left>
          <div class="flex items-center gap-1.5" role="group" :aria-label="t('instances.table.viewLabel')">
            <UButton
              icon="i-lucide-layout-grid"
              size="sm"
              :color="view === 'cards' ? 'primary' : 'neutral'"
              :variant="view === 'cards' ? 'solid' : 'outline'"
              :aria-label="t('instances.table.viewCards')"
              :aria-pressed="view === 'cards'"
              @click="view = 'cards'"
            />
            <UButton
              icon="i-lucide-table"
              size="sm"
              :color="view === 'table' ? 'primary' : 'neutral'"
              :variant="view === 'table' ? 'solid' : 'outline'"
              :aria-label="t('instances.table.viewTable')"
              :aria-pressed="view === 'table'"
              @click="view = 'table'"
            />
          </div>
        </template>
      </UDashboardToolbar>
    </template>

    <template #body>
      <p class="mb-4 text-sm text-muted">
        {{ isAdmin ? t('instances.subtitleAdmin') : t('instances.subtitleUser') }}
      </p>

      <!-- Initial load renders per-view skeletons; both views mount only after
      load. -->
      <div v-if="pending && view === 'table'" class="flex flex-col gap-2">
        <USkeleton class="h-12 w-full" />
        <USkeleton class="h-12 w-full" />
        <USkeleton class="h-12 w-full" />
      </div>

      <div v-else-if="pending" class="grid grid-cols-1 gap-4 sm:grid-cols-2 2xl:grid-cols-3">
        <USkeleton v-for="n in 6" :key="n" class="h-36 w-full" />
      </div>

      <UAlert
        v-else-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      >
        <template #actions>
          <UButton
            color="error"
            variant="soft"
            :label="t('common.retry')"
            @click="loadFirst"
          />
        </template>
      </UAlert>

      <div v-else class="flex flex-col gap-4">
        <!-- KeepAlive preserves each view's filter/pagination/selection state
        across toggles, matching the pre-extraction shared-state behavior. -->
        <KeepAlive>
          <InstancesTable
            v-if="view === 'table'"
            :items="items"
            :owner-emails="ownerEmails"
            :is-admin="isAdmin"
            @connect="onConnect"
            @edit="openEdit"
            @remove="openDelete"
            @create="createOpen = true"
          />
          <InstancesCards
            v-else
            :items="items"
            :owner-emails="ownerEmails"
            :is-admin="isAdmin"
            @open="openDetails"
            @connect="onConnect"
            @edit="openEdit"
            @remove="openDelete"
            @create="createOpen = true"
          />
        </KeepAlive>
      </div>
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
