<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import DeleteInstanceModal from '~/components/instances/DeleteInstanceModal.vue'
import InstanceChannelsSection from '~/components/instances/detail/InstanceChannelsSection.vue'
import InstanceGroupsSection from '~/components/instances/detail/InstanceGroupsSection.vue'
import InstanceIntegrationsSection from '~/components/instances/detail/InstanceIntegrationsSection.vue'
import InstanceMessagesSection from '~/components/instances/detail/InstanceMessagesSection.vue'
import InstanceOverviewSection from '~/components/instances/detail/InstanceOverviewSection.vue'
import InstanceProfileSection from '~/components/instances/detail/InstanceProfileSection.vue'
import InstanceSectionNav from '~/components/instances/detail/InstanceSectionNav.vue'
import InstanceSettingsSection from '~/components/instances/detail/InstanceSettingsSection.vue'
import InstanceStatusBadge from '~/components/instances/InstanceStatusBadge.vue'
import type { Instance } from '~/types/api'

const { t } = useI18n()
const toast = useToast()
const route = useRoute()
const { isAdmin } = useAuth()
const { getInstance } = useInstances()

const id = computed(() => String(route.params.id ?? ''))

const instance = ref<Instance | null>(null)
const pending = ref(true)
const notFound = ref(false)
const failure = ref<string | null>(null)

const deleteOpen = ref(false)

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

          <InstanceGroupsSection
            v-else-if="section === 'groups'"
            :instance-id="instance.id"
            :status="instance.status"
          />

          <InstanceChannelsSection
            v-else-if="section === 'channels'"
            :instance-id="instance.id"
            :status="instance.status"
          />

          <InstanceProfileSection
            v-else-if="section === 'profile'"
            :instance-id="instance.id"
            :status="instance.status"
          />

          <InstanceIntegrationsSection
            v-else-if="section === 'integrations'"
            :instance="instance"
            @webhook-updated="(value: Instance) => { instance = value }"
          />

          <InstanceSettingsSection
            v-else
            :instance="instance"
            :is-admin="isAdmin"
            @changed="load"
            @delete-requested="deleteOpen = true"
          />
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
