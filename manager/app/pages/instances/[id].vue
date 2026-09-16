<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import DeleteInstanceModal from '~/components/instances/DeleteInstanceModal.vue'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import InstanceStatusBadge from '~/components/instances/InstanceStatusBadge.vue'
import MessagesCard from '~/components/instances/MessagesCard.vue'
import OneTimeKeyDisplay from '~/components/instances/OneTimeKeyDisplay.vue'
import PairingCard from '~/components/instances/PairingCard.vue'
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
    toast.add({ title: t('instances.detail.saved'), color: 'success' })
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
    toast.add({ title: t('instances.detail.disconnected'), color: 'success' })
    await load()
  } catch (error) {
    toast.add({ title: error instanceof ApiError ? error.message : t('instances.detail.disconnectFailed'), color: 'error' })
  } finally {
    disconnecting.value = false
  }
}

function onDeleted() {
  toast.add({ title: t('instances.detail.deleted'), color: 'success' })
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
    toast.add({ title: t('instances.key.generated'), color: 'success' })
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
    toast.add({ title: t('instances.key.revoked'), color: 'success' })
  } catch (error) {
    toast.add({ title: error instanceof ApiError ? error.message : t('instances.key.revokeFailed'), color: 'error' })
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
  <UDashboardPanel id="instance-detail">
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
    </template>

    <template #body>
      <div v-if="pending" class="flex flex-col gap-2">
        <USkeleton class="h-32 w-full" />
        <USkeleton class="h-48 w-full" />
      </div>

      <UEmpty
        v-else-if="notFound"
        icon="i-lucide-search-x"
        :title="t('instances.detail.notFound')"
      >
        <template #actions>
          <UButton :label="t('instances.title')" @click="navigateTo('/instances')" />
        </template>
      </UEmpty>

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
            @click="load"
          />
        </template>
      </UAlert>

      <div v-else-if="instance" class="flex w-full flex-col gap-4">
        <UCard>
          <template #header>
            <h2 class="font-medium text-highlighted">
              {{ instance.name }}
            </h2>
          </template>
          <dl class="flex flex-col gap-2 text-sm">
            <div class="flex justify-between gap-4">
              <dt class="text-muted">
                {{ t('instances.fields.jid') }}
              </dt>
              <dd class="font-mono text-highlighted">
                {{ instance.whatsapp_jid || t('common.notSet') }}
              </dd>
            </div>
            <div v-if="instance.last_error" class="flex justify-between gap-4">
              <dt class="text-muted">
                {{ t('instances.fields.lastError') }}
              </dt>
              <dd class="text-right text-highlighted">
                {{ instance.last_error }}
              </dd>
            </div>
            <div class="flex justify-between gap-4">
              <dt class="text-muted">
                {{ t('instances.fields.createdAt') }}
              </dt>
              <dd class="text-highlighted">
                {{ formatDateTime(instance.created_at) }}
              </dd>
            </div>
            <div class="flex justify-between gap-4">
              <dt class="text-muted">
                {{ t('instances.fields.updatedAt') }}
              </dt>
              <dd class="text-highlighted">
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
        </UCard>

        <PairingCard
          :instance-id="instance.id"
          :status="instance.status"
          :whatsapp-jid="instance.whatsapp_jid"
          @paired="onPaired"
        />

        <UCard>
          <template #header>
            <h2 class="font-medium text-highlighted">
              {{ t('instances.fields.name') }}
            </h2>
          </template>
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
        </UCard>

        <UCard v-if="isAdmin">
          <template #header>
            <h2 class="font-medium text-highlighted">
              {{ t('instances.key.cardTitle') }}
            </h2>
          </template>

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
        </UCard>

        <WebhookCard
          :instance="instance"
          @updated="onWebhookUpdated"
        />

        <TestSendCard
          :instance-id="instance.id"
          :status="instance.status"
          @sent="onMessagesRefresh"
          @settled="onMessagesRefresh"
        />

        <MessagesCard
          :instance-id="instance.id"
          :refresh-key="messagesRefresh"
        />

        <UCard>
          <template #header>
            <h2 class="font-medium text-error">
              {{ t('instances.detail.delete') }}
            </h2>
          </template>
          <div class="flex items-center justify-between gap-4">
            <p class="text-sm text-muted">
              {{ t('instances.delete.warning') }}
            </p>
            <UButton
              color="error"
              variant="soft"
              icon="i-lucide-trash-2"
              :label="t('instances.detail.delete')"
              @click="deleteOpen = true"
            />
          </div>
        </UCard>
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
