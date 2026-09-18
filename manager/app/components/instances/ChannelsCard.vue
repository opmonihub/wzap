<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, Newsletter, OwnStatus } from '~/types/api'

// Newsletters (follow/unfollow + lookup + followed list with title/channel +
// follower count) and own statuses (text publish, image|video media publish
// fire-and-forget, own list rendered as bubbles only where text/caption
// exists, delete). Text/caption validate 1..700 characters.
const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const channels = useInstanceChannels()

const channel = ref('')
const lookedUp = ref<Newsletter | null>(null)
const followed = ref<Newsletter[]>([])
const statuses = ref<OwnStatus[]>([])
const statusText = ref('')
const statusKind = ref<'image' | 'video'>('image')
const statusFile = ref<File | null>(null)
const statusCaption = ref('')
const failure = ref<string | null>(null)
const canAct = computed(() => props.status === 'connected')

const statusKinds = computed(() => [
  { label: 'image', value: 'image' },
  { label: 'video', value: 'video' }
])

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

async function onLookup() {
  failure.value = null
  lookedUp.value = null
  if (channel.value.trim() === '') {
    return
  }
  try {
    lookedUp.value = await channels.getNewsletter(props.instanceId, channel.value.trim())
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.channels.lookupFailed')
  }
}

async function onFollow() {
  if (channel.value.trim() === '') {
    return
  }
  failure.value = null
  try {
    await channels.followNewsletter(props.instanceId, channel.value.trim())
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.channels.followFailed')
  }
}

async function onUnfollow(channelId: string) {
  failure.value = null
  try {
    await channels.unfollowNewsletter(props.instanceId, channelId)
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.channels.unfollowFailed')
  }
}

async function onPublish() {
  const text = statusText.value.trim()
  if ([...text].length === 0 || [...text].length > 700) {
    failure.value = t('instances.channels.statusHint')
    return
  }
  failure.value = null
  try {
    await channels.publishTextStatus(props.instanceId, text)
    statusText.value = ''
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.channels.publishFailed')
  }
}

async function onPublishMedia() {
  if (!statusFile.value) {
    failure.value = t('instances.send.fileRequired')
    return
  }
  const caption = statusCaption.value.trim()
  if ([...caption].length > 700) {
    failure.value = t('instances.channels.statusHint')
    return
  }
  failure.value = null
  try {
    await channels.publishMediaStatus(props.instanceId, statusKind.value, statusFile.value, caption)
    statusFile.value = null
    statusCaption.value = ''
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.channels.publishFailed')
  }
}

async function onDeleteStatus(id: string) {
  failure.value = null
  try {
    await channels.deleteStatus(props.instanceId, id)
    await refresh()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.channels.deleteFailed')
  }
}

// Session-backed lists answer 409 while disconnected, which the browser
// logs as a failed request on every section visit — skip the fetch and
// clear stale rows until the session is connected.
watch(() => [props.instanceId, props.status] as const, () => {
  lookedUp.value = null
  if (props.status !== 'connected') {
    followed.value = []
    statuses.value = []
    failure.value = null
    return
  }
  void refresh()
}, { immediate: true })
</script>

<template>
  <UPageCard variant="subtle" data-testid="channels-card" :ui="{ container: 'p-0 sm:p-0 gap-y-0', wrapper: 'items-stretch', header: 'p-4 mb-0 border-b border-default' }">
    <template #header>
      <div class="flex min-w-0 flex-col gap-2">
        <UAlert
          v-if="!canAct"
          color="warning"
          variant="subtle"
          :title="t('instances.actions.notConnected')"
        />
        <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
          <UInput v-model="channel" :placeholder="t('instances.channels.channelPlaceholder')" class="w-full font-mono" />
          <div class="flex w-fit shrink-0 gap-2">
            <UButton
              :disabled="!canAct"
              variant="soft"
              :label="t('instances.channels.lookup')"
              @click="onLookup"
            />
            <UButton :disabled="!canAct" :label="t('instances.channels.follow')" @click="onFollow" />
          </div>
        </div>
      </div>
    </template>
    <UAlert
      v-if="failure"
      color="error"
      variant="subtle"
      :title="failure"
    />
    <ul class="divide-y divide-default">
      <li v-if="lookedUp" class="flex min-w-0 items-center gap-3 px-4 py-3 sm:px-6">
        <UAvatar :alt="lookedUp.title || lookedUp.channel" size="md" />
        <span class="min-w-0 flex-1">
          <span class="block truncate font-medium text-highlighted">{{ lookedUp.title || lookedUp.channel }}</span>
          <span class="block truncate text-sm text-muted">{{ lookedUp.channel }} · {{ lookedUp.follower_count }}</span>
        </span>
        <UButton
          :disabled="!canAct"
          size="xs"
          variant="soft"
          :label="t('instances.channels.follow')"
          @click="onFollow"
        />
      </li>
      <li v-for="item in followed" :key="item.channel" class="flex min-w-0 items-center gap-3 px-4 py-3 sm:px-6">
        <UAvatar :alt="item.title || item.channel" size="md" />
        <span class="min-w-0 flex-1">
          <span class="block truncate font-medium text-highlighted">{{ item.title || item.channel }}</span>
          <span class="block truncate text-sm text-muted">{{ item.channel }} · {{ item.follower_count }}</span>
        </span>
        <UButton
          :disabled="!canAct"
          size="xs"
          variant="ghost"
          color="error"
          :label="t('instances.channels.unfollow')"
          @click="onUnfollow(item.channel)"
        />
      </li>
      <li v-for="item in statuses.filter(entry => (entry.text ?? '') !== '' || (entry.caption ?? '') !== '')" :key="item.id" class="flex min-w-0 items-center gap-3 px-4 py-3 sm:px-6">
        <span class="min-w-0 flex-1 rounded-2xl bg-elevated px-3 py-2 text-sm text-highlighted">
          {{ item.text || item.caption }}
        </span>
        <UButton
          :disabled="!canAct"
          size="xs"
          variant="ghost"
          color="error"
          :label="t('instances.channels.delete')"
          @click="onDeleteStatus(item.id)"
        />
      </li>
    </ul>
    <UEmpty v-if="!lookedUp && followed.length === 0 && statuses.length === 0" icon="i-lucide-rss" :title="t('instances.channels.empty')" />
    <template #footer>
      <div class="flex min-w-0 flex-col gap-2">
        <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
          <UInput
            v-model="statusText"
            maxlength="700"
            class="w-full"
            :placeholder="t('instances.channels.statusText')"
            :hint="t('instances.channels.statusHint')"
          />
          <UButton
            :disabled="!canAct"
            class="w-fit shrink-0"
            :label="t('instances.channels.publish')"
            @click="onPublish"
          />
        </div>
        <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
          <USelect
            v-model="statusKind"
            :items="statusKinds"
            :placeholder="t('instances.channels.statusKind')"
            class="w-full sm:w-32"
          />
          <UFileUpload v-model="statusFile" accept="image/*,video/*" class="w-full" />
          <UInput
            v-model="statusCaption"
            maxlength="700"
            class="w-full"
            :placeholder="t('instances.send.caption')"
          />
          <UButton
            :disabled="!canAct || !statusFile"
            class="w-fit shrink-0"
            variant="soft"
            :label="t('instances.channels.publish')"
            @click="onPublishMedia"
          />
        </div>
      </div>
    </template>
  </UPageCard>
</template>
