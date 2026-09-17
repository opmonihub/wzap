<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, Newsletter, OwnStatus } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const channels = useInstanceChannels()

const channel = ref('')
const followed = ref<Newsletter[]>([])
const statuses = ref<OwnStatus[]>([])
const statusText = ref('')
const failure = ref<string | null>(null)
const canAct = computed(() => props.status === 'connected')

async function refresh() {
  failure.value = null
  try {
    const page = await channels.listNewsletters(props.instanceId)
    followed.value = page.items
    const listed = await channels.listStatuses(props.instanceId)
    statuses.value = listed.items
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.messages.loadFailed')
  }
}

async function onFollow() {
  try {
    await channels.followNewsletter(props.instanceId, channel.value)
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onPublish() {
  const text = statusText.value.trim()
  if ([...text].length === 0 || [...text].length > 700) {
    return
  }
  try {
    await channels.publishTextStatus(props.instanceId, text)
    statusText.value = ''
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onDeleteStatus(id: string) {
  try {
    await channels.deleteStatus(props.instanceId, id)
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

watch(() => props.instanceId, () => void refresh(), { immediate: true })
</script>

<template>
  <UPageCard variant="subtle" data-testid="channels-card" :ui="{ container: 'p-0 sm:p-0 gap-y-0', wrapper: 'items-stretch', header: 'p-4 mb-0 border-b border-default' }">
    <template #header>
      <div class="flex items-center gap-2">
        <UInput v-model="channel" :placeholder="t('instances.groups.search')" class="w-full font-mono" />
        <UButton :disabled="!canAct" :label="t('instances.channels.follow')" @click="onFollow" />
      </div>
    </template>
    <UAlert
      v-if="failure"
      color="error"
      variant="subtle"
      :title="failure"
    />
    <ul class="divide-y divide-default">
      <li v-for="item in followed" :key="item.channel" class="px-4 py-3 sm:px-6">
        <p class="truncate font-medium text-highlighted">
          {{ item.title || item.channel }}
        </p>
        <p class="truncate text-sm text-muted">
          {{ item.channel }} · {{ item.follower_count }}
        </p>
      </li>
      <li v-for="item in statuses" :key="item.id" class="px-4 py-3 sm:px-6">
        <p v-if="item.text || item.caption" class="text-sm text-highlighted">
          {{ item.text || item.caption }}
        </p>
        <UButton
          size="xs"
          variant="ghost"
          color="error"
          label="Delete"
          @click="onDeleteStatus(item.id)"
        />
      </li>
    </ul>
    <template #footer>
      <div class="flex gap-2">
        <UInput
          v-model="statusText"
          maxlength="700"
          class="w-full"
          :placeholder="t('instances.channels.publish')"
        />
        <UButton :disabled="!canAct" :label="t('instances.channels.publish')" @click="onPublish" />
      </div>
    </template>
  </UPageCard>
</template>
