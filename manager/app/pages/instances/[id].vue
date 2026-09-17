<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import ChannelsCard from '~/components/instances/ChannelsCard.vue'
import ChatwootCard from '~/components/instances/ChatwootCard.vue'
import DeleteInstanceModal from '~/components/instances/DeleteInstanceModal.vue'
import DeviceActionsCard from '~/components/instances/DeviceActionsCard.vue'
import GroupDetail from '~/components/instances/GroupDetail.vue'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import InstanceMessagesSection from '~/components/instances/detail/InstanceMessagesSection.vue'
import InstanceOverviewSection from '~/components/instances/detail/InstanceOverviewSection.vue'
import InstanceSectionNav from '~/components/instances/detail/InstanceSectionNav.vue'
import InstanceStatusBadge from '~/components/instances/InstanceStatusBadge.vue'
import OneTimeKeyDisplay from '~/components/instances/OneTimeKeyDisplay.vue'
import PrivacyCard from '~/components/instances/PrivacyCard.vue'
import ProfileCard from '~/components/instances/ProfileCard.vue'
import WebhookCard from '~/components/instances/WebhookCard.vue'
import type { Instance, RotatedInstanceKey } from '~/types/api'

const { t } = useI18n()
const toast = useToast()
const route = useRoute()
const { isAdmin } = useAuth()
const { getInstance, rotateInstanceKey, revokeInstanceKey } = useInstances()
const { confirmDelete } = useConfirmDelete()

const id = computed(() => String(route.params.id ?? ''))

const instance = ref<Instance | null>(null)
const pending = ref(true)
const notFound = ref(false)
const failure = ref<string | null>(null)

const deleteOpen = ref(false)

const freshKey = ref<RotatedInstanceKey | null>(null)
const keySeen = ref(false)
const generating = ref(false)
const keyFailure = ref<string | null>(null)
const revoking = ref(false)

const section = ref('overview')

useSeoMeta({
  title: 'Instance details'
})

async function load() {
  pending.value = true
  notFound.value = false
  failure.value = null
  try {
    instance.value = await getInstance(id.value)
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

function onDeleted() {
  toast.add({ title: t('instances.detail.deleted'), icon: 'i-lucide-check', color: 'success' })
  navigateTo('/instances')
}

// The webhook card emits the stored instance answered by its PATCH; the
// detail keeps showing the persisted configuration.
function onWebhookUpdated(updated: Instance) {
  instance.value = updated
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

      <InstanceSectionNav
        v-if="instance && !pending && !notFound && !failure"
        :section="section"
        @select="(value: string) => { section = value }"
      />
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
          <InstanceOverviewSection
            v-if="section === 'overview'"
            :instance="instance"
            @paired="load"
            @updated="(value: Instance) => { instance = value }"
            @changed="load"
          />

          <InstanceMessagesSection
            v-else-if="section === 'messages'"
            :instance-id="instance.id"
            :status="instance.status"
          />

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
