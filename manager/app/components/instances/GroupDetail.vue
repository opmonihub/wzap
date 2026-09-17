<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { Group, InstanceStatus } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const { createGroup, getGroup, getInvite, joinGroup, leaveGroup } = useInstanceGroups()

const jid = ref('')
const group = ref<Group | null>(null)
const failure = ref<string | null>(null)
const notice = ref<string | null>(null)
const createOpen = ref(false)
const joinOpen = ref(false)
const newName = ref('')
const inviteCode = ref('')
const canAct = computed(() => props.status === 'connected')

async function onLookup() {
  failure.value = null
  notice.value = null
  try {
    group.value = await getGroup(props.instanceId, jid.value.trim())
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.messages.loadFailed')
  }
}

async function onCreate() {
  failure.value = null
  notice.value = null
  const name = newName.value.trim()
  if ([...name].length === 0 || [...name].length > 25) {
    failure.value = t('instances.groups.nameTooLong')
    return
  }
  try {
    const created = await createGroup(props.instanceId, { name })
    group.value = created
    createOpen.value = false
    if (!created.invite_code) {
      notice.value = t('instances.groups.reconcileInvite')
      const invite = await getInvite(props.instanceId, created.jid)
      group.value = { ...created, invite_code: invite.invite_code }
    }
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onJoin() {
  failure.value = null
  try {
    const joined = await joinGroup(props.instanceId, inviteCode.value)
    jid.value = joined.jid
    joinOpen.value = false
    await onLookup()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onLeave() {
  if (!group.value) {
    return
  }
  try {
    await leaveGroup(props.instanceId, group.value.jid)
    group.value = null
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}
</script>

<template>
  <UPageCard variant="subtle" data-testid="group-detail" :ui="{ container: 'p-0 sm:p-0 gap-y-0', wrapper: 'items-stretch', header: 'p-4 mb-0 border-b border-default' }">
    <template #header>
      <div class="flex items-center gap-2">
        <UInput v-model="jid" :placeholder="t('instances.groups.search')" class="w-full font-mono" />
        <UButton :disabled="!canAct" :label="t('common.refresh')" @click="onLookup" />
        <UButton :disabled="!canAct" :label="t('instances.groups.create')" @click="createOpen = true" />
        <UButton :disabled="!canAct" variant="soft" :label="t('instances.groups.join')" @click="joinOpen = true" />
      </div>
    </template>
    <UAlert v-if="failure" color="error" variant="subtle" :title="failure" />
    <UAlert v-if="notice" color="warning" variant="subtle" :title="notice" />
    <ul v-if="group" class="divide-y divide-default">
      <li class="px-4 py-3 sm:px-6">
        <p class="truncate font-medium text-highlighted">
          {{ group.name }}
        </p>
        <p class="truncate text-sm text-muted">
          {{ group.participant_count }} · {{ group.description || group.jid }}
        </p>
      </li>
    </ul>
    <UEmpty v-else icon="i-lucide-users" :title="t('instances.groups.empty')" />
    <UModal v-model:open="createOpen" :title="t('instances.groups.create')">
      <template #body>
        <UInput v-model="newName" maxlength="25" class="w-full" />
      </template>
      <template #footer>
        <UButton :label="t('instances.groups.create')" @click="onCreate" />
      </template>
    </UModal>
    <UModal v-model:open="joinOpen" :title="t('instances.groups.join')">
      <template #body>
        <UInput v-model="inviteCode" class="w-full font-mono" />
      </template>
      <template #footer>
        <UButton :label="t('instances.groups.join')" @click="onJoin" />
      </template>
    </UModal>
    <template v-if="group" #footer>
      <UButton color="error" variant="soft" label="Leave" @click="onLeave" />
    </template>
  </UPageCard>
</template>
