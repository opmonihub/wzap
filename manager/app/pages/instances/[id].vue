<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent, NavigationMenuItem } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import ChannelsCard from '~/components/instances/ChannelsCard.vue'
import ChatwootCard from '~/components/instances/ChatwootCard.vue'
import DeleteInstanceModal from '~/components/instances/DeleteInstanceModal.vue'
import DeviceActionsCard from '~/components/instances/DeviceActionsCard.vue'
import GroupDetail from '~/components/instances/GroupDetail.vue'
import MessageActionsCard from '~/components/instances/MessageActionsCard.vue'
import MessageComposer from '~/components/instances/MessageComposer.vue'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import InstanceStatusBadge from '~/components/instances/InstanceStatusBadge.vue'
import MessagesCard from '~/components/instances/MessagesCard.vue'
import OneTimeKeyDisplay from '~/components/instances/OneTimeKeyDisplay.vue'
import PairingCard from '~/components/instances/PairingCard.vue'
import PairPhoneCard from '~/components/instances/PairPhoneCard.vue'
import PrivacyCard from '~/components/instances/PrivacyCard.vue'
import ProfileCard from '~/components/instances/ProfileCard.vue'
import TestSendCard from '~/components/instances/TestSendCard.vue'
import WebhookCard from '~/components/instances/WebhookCard.vue'
import type { Instance, RotatedInstanceKey } from '~/types/api'

const { t } = useI18n()
const toast = useToast()
const route = useRoute()
const { isAdmin } = useAuth()
const { getInstance, updateInstance, disconnectInstance, rotateInstanceKey, revokeInstanceKey } = useInstances()
const { confirmDelete } = useConfirmDelete()

const id = computed(() => String(route.params.id ?? ''))

const instance = ref<Instance | null>(null)
const pending = ref(true)
const notFound = ref(false)
const failure = ref<string | null>(null)

const schema = z.object({
  name: z.string().min(1, t('instances.create.nameRequired')).max(255),
  external_ref: z.string().max(255)
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ name: '', external_ref: '' })
const saving = ref(false)
const saveFailure = ref<string | null>(null)

const deleteOpen = ref(false)
const disconnecting = ref(false)

const freshKey = ref<RotatedInstanceKey | null>(null)
const keySeen = ref(false)
const generating = ref(false)
const keyFailure = ref<string | null>(null)
const revoking = ref(false)
const messagesRefresh = ref(0)

const section = ref('overview')

// Section nav mirrors research/dashboard settings.vue: UNavigationMenu in a
// UDashboardToolbar with icons + highlight. Switching still rides on each
// item's onSelect (UNavigationMenu does not reliably emit update:modelValue
// for value-only items), while `active` keeps the highlight pill in sync
// with the local section state instead of the route.
const sections = computed<NavigationMenuItem[]>(() => [
  { label: t('instances.sections.overview'), icon: 'i-lucide-house', value: 'overview', active: section.value === 'overview', onSelect: () => { section.value = 'overview' } },
  { label: t('instances.sections.messages'), icon: 'i-lucide-message-square-text', value: 'messages', active: section.value === 'messages', onSelect: () => { section.value = 'messages' } },
  { label: t('instances.sections.groups'), icon: 'i-lucide-users', value: 'groups', active: section.value === 'groups', onSelect: () => { section.value = 'groups' } },
  { label: t('instances.sections.channels'), icon: 'i-lucide-megaphone', value: 'channels', active: section.value === 'channels', onSelect: () => { section.value = 'channels' } },
  { label: t('instances.sections.profile'), icon: 'i-lucide-user', value: 'profile', active: section.value === 'profile', onSelect: () => { section.value = 'profile' } },
  { label: t('instances.sections.integrations'), icon: 'i-lucide-plug', value: 'integrations', active: section.value === 'integrations', onSelect: () => { section.value = 'integrations' } },
  { label: t('instances.sections.settings'), icon: 'i-lucide-settings', value: 'settings', active: section.value === 'settings', onSelect: () => { section.value = 'settings' } }
])

useSeoMeta({
  title: 'Instance details'
})

async function load() {
  pending.value = true
  notFound.value = false
  failure.value = null
  try {
    instance.value = await getInstance(id.value)
    state.name = instance.value.name
    state.external_ref = instance.value.external_ref
    keySeen.value = hasSeenInstanceKey(instance.value.id)
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      notFound.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.detail.loadFailed')
    }
  } finally {
    pending.value = false
  }
}

async function onSave(event: FormSubmitEvent<Schema>) {
  if (!instance.value || saving.value) {
    return
  }
  const trimmedName = (event.data.name ?? '').trim()
  if (trimmedName === '') {
    saveFailure.value = t('instances.create.nameRequired')
    return
  }
  saving.value = true
  saveFailure.value = null
  try {
    instance.value = await updateInstance(instance.value.id, {
      name: trimmedName,
      external_ref: (event.data.external_ref ?? '').trim()
    })
    toast.add({ title: t('instances.detail.saved'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    saveFailure.value = friendlySaveError(error)
  } finally {
    saving.value = false
  }
}

function friendlySaveError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) {
      return t('instances.detail.externalRefTaken')
    }
    return error.message
  }
  return t('instances.detail.saveFailed')
}

// Disconnect runs behind the programmatic confirm: the overlay resolves true
// only on the confirm button, and the API (with its loading state and toasts)
// runs on the trigger afterwards.
async function onDisconnect() {
  if (!instance.value || disconnecting.value) {
    return
  }
  const confirmed = await confirmDelete({
    title: t('instances.detail.disconnectConfirmTitle'),
    description: t('instances.detail.disconnectConfirmBody'),
    confirmLabel: t('instances.detail.disconnect'),
    confirmColor: 'warning'
  })
  if (!confirmed || !instance.value || disconnecting.value) {
    return
  }
  const instanceId = instance.value.id
  disconnecting.value = true
  try {
    await disconnectInstance(instanceId)
    toast.add({ title: t('instances.detail.disconnected'), icon: 'i-lucide-check', color: 'success' })
    await load()
  } catch (error) {
    toast.add({ title: error instanceof ApiError ? error.message : t('instances.detail.disconnectFailed'), icon: 'i-lucide-triangle-alert', color: 'error' })
  } finally {
    disconnecting.value = false
  }
}

function onDeleted() {
  toast.add({ title: t('instances.detail.deleted'), icon: 'i-lucide-check', color: 'success' })
  navigateTo('/instances')
}

// The pairing card emits after the phone scan flips the session to
// connected; reloading refreshes the badge, JID and disconnect action.
async function onPaired() {
  await load()
}

// The webhook card emits the stored instance answered by its PATCH; the
// detail keeps showing the persisted configuration.
function onWebhookUpdated(updated: Instance) {
  instance.value = updated
}

// A test send emits on accept and again when the message settles; either
// refresh bumps the message history so the new row shows up.
function onMessagesRefresh() {
  messagesRefresh.value += 1
}

async function onGenerate() {
  if (!instance.value || generating.value) {
    return
  }
  generating.value = true
  keyFailure.value = null
  try {
    freshKey.value = await rotateInstanceKey(instance.value.id)
    markInstanceKeySeen(instance.value.id)
    keySeen.value = true
    toast.add({ title: t('instances.key.generated'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    keyFailure.value = error instanceof ApiError ? error.message : t('instances.key.generateFailed')
  } finally {
    generating.value = false
  }
}

// Revoke runs behind the programmatic confirm like disconnect: failures
// surface as toasts, while keyFailure stays owned by the generate path whose
// alert renders in the card.
async function onRevoke() {
  if (!instance.value || revoking.value) {
    return
  }
  const confirmed = await confirmDelete({
    title: t('instances.key.revokeConfirmTitle'),
    description: t('instances.key.revokeConfirmBody'),
    confirmLabel: t('instances.key.revoke')
  })
  if (!confirmed || !instance.value || revoking.value) {
    return
  }
  const instanceId = instance.value.id
  revoking.value = true
  try {
    await revokeInstanceKey(instanceId)
    forgetInstanceKeySeen(instanceId)
    keySeen.value = false
    freshKey.value = null
    toast.add({ title: t('instances.key.revoked'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    toast.add({ title: error instanceof ApiError ? error.message : t('instances.key.revokeFailed'), icon: 'i-lucide-triangle-alert', color: 'error' })
  } finally {
    revoking.value = false
  }
}

function formatDateTime(value: string | null): string {
  if (!value) {
    return t('common.notSet')
  }
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

await load()
</script>

<template>
  <UDashboardPanel id="instance-detail" :ui="{ body: 'lg:py-12' }">
    <template #header>
      <UDashboardNavbar :title="instance?.name ?? t('instances.title')">
        <template #leading>
          <UDashboardSidebarCollapse />
          <UButton
            color="neutral"
            variant="ghost"
            icon="i-lucide-arrow-left"
            @click="navigateTo('/instances')"
          />
        </template>
        <template v-if="instance" #right>
          <InstanceStatusBadge :status="instance.status" />
        </template>
      </UDashboardNavbar>

      <UDashboardToolbar v-if="instance && !pending && !notFound && !failure">
        <!-- NOTE: The `-mx-1` class is used to align with the `DashboardSidebarCollapse` button here. -->
        <UNavigationMenu
          highlight
          class="-mx-1 max-w-full min-w-0 flex-1 overflow-x-auto"
          :items="sections"
        />
      </UDashboardToolbar>
    </template>

    <template #body>
      <!-- Single width for every tab, mirroring settings.vue: the body
      centers one max-w-2xl column on lg+ and stretches full-width below. -->
      <div class="mx-auto flex w-full min-w-0 max-w-full flex-col gap-4 sm:gap-6 lg:max-w-2xl lg:gap-12">
        <div v-if="pending" class="flex w-full flex-col gap-2">
          <USkeleton class="h-32 w-full" />
          <USkeleton class="h-48 w-full" />
        </div>

        <UEmpty
          v-else-if="notFound"
          class="w-full"
          icon="i-lucide-search-x"
          :title="t('instances.detail.notFound')"
        >
          <template #actions>
            <UButton :label="t('instances.title')" @click="navigateTo('/instances')" />
          </template>
        </UEmpty>

        <UAlert
          v-else-if="failure"
          class="w-full"
          color="error"
          variant="subtle"
          :title="failure"
        >
          <template #actions>
            <UButton
              color="error"
              variant="soft"
              :label="t('common.retry')"
              @click="load"
            />
          </template>
        </UAlert>

        <template v-else-if="instance">
          <div v-if="section === 'overview'" class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
            <UPageCard :title="instance.name" variant="subtle">
              <dl class="flex flex-col gap-3 text-sm sm:gap-2">
                <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
                  <dt class="shrink-0 text-muted">
                    {{ t('instances.fields.jid') }}
                  </dt>
                  <dd class="min-w-0 font-mono break-all text-highlighted sm:text-right">
                    {{ instance.whatsapp_jid || t('common.notSet') }}
                  </dd>
                </div>
                <div v-if="instance.last_error" class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
                  <dt class="shrink-0 text-muted">
                    {{ t('instances.fields.lastError') }}
                  </dt>
                  <dd class="min-w-0 break-all text-highlighted sm:text-right">
                    {{ instance.last_error }}
                  </dd>
                </div>
                <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
                  <dt class="shrink-0 text-muted">
                    {{ t('instances.fields.createdAt') }}
                  </dt>
                  <dd class="min-w-0 break-all text-highlighted sm:text-right">
                    {{ formatDateTime(instance.created_at) }}
                  </dd>
                </div>
                <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
                  <dt class="shrink-0 text-muted">
                    {{ t('instances.fields.updatedAt') }}
                  </dt>
                  <dd class="min-w-0 break-all text-highlighted sm:text-right">
                    {{ formatDateTime(instance.updated_at) }}
                  </dd>
                </div>
              </dl>
              <template v-if="instance.status === 'connected'" #footer>
                <UButton
                  color="warning"
                  variant="soft"
                  icon="i-lucide-unplug"
                  :loading="disconnecting"
                  :label="disconnecting ? t('instances.detail.disconnecting') : t('instances.detail.disconnect')"
                  @click="onDisconnect"
                />
              </template>
            </UPageCard>

            <PairingCard
              :instance-id="instance.id"
              :status="instance.status"
              :whatsapp-jid="instance.whatsapp_jid"
              @paired="onPaired"
            />

            <PairPhoneCard :instance-id="instance.id" :status="instance.status" />

            <UPageCard :title="t('instances.fields.name')" variant="subtle">
              <UForm
                id="instance-name"
                :schema="schema"
                :state="state"
                class="flex flex-col gap-4"
                @submit="onSave"
              >
                <UAlert
                  v-if="saveFailure"
                  color="error"
                  variant="subtle"
                  :title="saveFailure"
                />

                <UFormField :label="t('instances.fields.name')" name="name" required>
                  <UInput
                    v-model="state.name"
                    maxlength="255"
                    class="w-full"
                  />
                </UFormField>

                <UFormField :label="t('instances.fields.externalRef')" :hint="t('instances.fields.externalRefHint')" name="external_ref">
                  <UInput v-model="state.external_ref" maxlength="255" class="w-full" />
                </UFormField>

                <div class="flex justify-end">
                  <UButton type="submit" :loading="saving" :label="saving ? t('common.saving') : t('common.save')" />
                </div>
              </UForm>
            </UPageCard>
          </div>

          <!-- Single-column stack shared by every tab: one card per row at
          the same max-w-2xl width, so sections never shift horizontally. -->
          <div v-else-if="section === 'messages'" class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
            <TestSendCard
              :instance-id="instance.id"
              :status="instance.status"
              @sent="onMessagesRefresh"
              @settled="onMessagesRefresh"
            />

            <MessageComposer
              :instance-id="instance.id"
              :status="instance.status"
              @sent="onMessagesRefresh"
              @settled="onMessagesRefresh"
            />

            <MessageActionsCard :instance-id="instance.id" :status="instance.status" />

            <ClientOnly>
              <MessagesCard :instance-id="instance.id" :refresh-key="messagesRefresh" />
            </ClientOnly>
          </div>

          <div v-else-if="section === 'groups'" class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
            <GroupDetail :instance-id="instance.id" :status="instance.status" />
          </div>

          <div v-else-if="section === 'channels'" class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
            <ChannelsCard :instance-id="instance.id" :status="instance.status" />
          </div>

          <div v-else-if="section === 'profile'" class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
            <ProfileCard :instance-id="instance.id" :status="instance.status" />
            <PrivacyCard :instance-id="instance.id" :status="instance.status" />
            <DeviceActionsCard :instance-id="instance.id" :status="instance.status" />
          </div>

          <div v-else-if="section === 'integrations'" class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
            <WebhookCard
              :instance="instance"
              @updated="onWebhookUpdated"
            />

            <ChatwootCard :instance-id="instance.id" :status="instance.status" />
          </div>

          <div v-else class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
            <UPageCard v-if="isAdmin" :title="t('instances.key.cardTitle')" variant="subtle">
              <div class="flex flex-col gap-4">
                <UAlert
                  v-if="keyFailure"
                  color="error"
                  variant="subtle"
                  :title="keyFailure"
                />

                <OneTimeKeyDisplay v-if="freshKey" :api-key="freshKey.instance_api_key" />

                <template v-else>
                  <!-- Debt: no has-key flag exists in the API, so the banner is
              driven by the browser-side key-seen marker. A future API
              field (e.g. has_api_key) should replace this condition. -->
                  <UAlert
                    v-if="!keySeen"
                    color="info"
                    variant="subtle"
                    :title="t('instances.key.keylessTitle')"
                    :description="t('instances.key.keylessBody')"
                  />

                  <p v-else class="text-sm text-muted">
                    {{ t('instances.key.rotateHint') }}
                  </p>

                  <div class="flex flex-wrap gap-2">
                    <UButton
                      icon="i-lucide-key-round"
                      :loading="generating"
                      :label="generating ? t('instances.key.generating') : t('instances.key.generate')"
                      @click="onGenerate"
                    />
                    <UButton
                      v-if="keySeen"
                      color="error"
                      variant="soft"
                      :loading="revoking"
                      :label="revoking ? t('instances.key.revoking') : t('instances.key.revoke')"
                      @click="onRevoke"
                    />
                  </div>
                </template>
              </div>
            </UPageCard>

            <UPageCard :title="t('instances.detail.delete')" variant="subtle" :ui="{ title: 'text-error' }">
              <div class="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
                <p class="min-w-0 text-sm text-muted">
                  {{ t('instances.delete.warning') }}
                </p>
                <UButton
                  color="error"
                  variant="soft"
                  icon="i-lucide-trash-2"
                  class="w-fit shrink-0"
                  :label="t('instances.detail.delete')"
                  @click="deleteOpen = true"
                />
              </div>
            </UPageCard>
          </div>
        </template>
      </div>
    </template>
  </UDashboardPanel>

  <DeleteInstanceModal
    v-if="instance"
    v-model:open="deleteOpen"
    :instance="instance"
    @deleted="onDeleted"
  />
</template>
