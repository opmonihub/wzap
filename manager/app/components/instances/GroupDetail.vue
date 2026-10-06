<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { groupLookupJIDOrNull } from '~/utils/groupLookup'
import type { Group, InstanceStatus } from '~/types/api'

// Group lookup/create/join plus the loaded-group detail: rename/description
// update, invite get/reset, octet-stream photo upload and participant
// management. A 201 with an empty invite_code reconciles via GET invite and
// never retries create (retry would duplicate the group).
const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const toast = useToast()
const { createGroup, getGroup, updateGroup, setGroupPhoto, updateParticipants, getInvite, resetInvite, joinGroup, leaveGroup } = useInstanceGroups()

const jid = ref('')
const group = ref<Group | null>(null)
const failure = ref<string | null>(null)
const notice = ref<string | null>(null)
const createOpen = ref(false)
const joinOpen = ref(false)
const newName = ref('')
const inviteCode = ref('')
const editName = ref('')
const editDescription = ref('')
const invite = ref<string | null>(null)
const photoFile = ref<File | null>(null)
const participantAction = ref<'add' | 'remove' | 'promote' | 'demote'>('add')
const participantJids = ref('')
const canAct = computed(() => props.status === 'connected')

const participantActions = computed(() => [
  { label: t('instances.groups.actionAdd'), value: 'add' },
  { label: t('instances.groups.actionRemove'), value: 'remove' },
  { label: t('instances.groups.actionPromote'), value: 'promote' },
  { label: t('instances.groups.actionDemote'), value: 'demote' }
])

async function onLookup() {
  failure.value = null
  notice.value = null
  const lookupJID = groupLookupJIDOrNull(jid.value)
  if (lookupJID === null) {
    failure.value = t('instances.groups.jidRequired')
    return
  }
  try {
    group.value = await getGroup(props.instanceId, lookupJID)
    editName.value = group.value.name
    editDescription.value = group.value.description ?? ''
    invite.value = group.value.invite_code ?? null
  } catch (error) {
    group.value = null
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
    jid.value = created.jid
    editName.value = created.name
    editDescription.value = created.description ?? ''
    createOpen.value = false
    newName.value = ''
    if (!created.invite_code) {
      notice.value = t('instances.groups.reconcileInvite')
      try {
        const fetched = await getInvite(props.instanceId, created.jid)
        invite.value = fetched.invite_code
        group.value = { ...created, invite_code: fetched.invite_code }
      } catch (error) {
        failure.value = error instanceof ApiError ? error.message : t('instances.groups.inviteFailed')
      }
    } else {
      invite.value = created.invite_code
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
    inviteCode.value = ''
    await onLookup()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onUpdate() {
  if (!group.value) {
    return
  }
  failure.value = null
  const name = editName.value.trim()
  if ([...name].length === 0 || [...name].length > 25) {
    failure.value = t('instances.groups.nameTooLong')
    return
  }
  try {
    group.value = await updateGroup(props.instanceId, group.value.jid, {
      name,
      description: editDescription.value
    })
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.groups.updateFailed')
  }
}

async function onGetInvite() {
  if (!group.value) {
    return
  }
  failure.value = null
  try {
    const fetched = await getInvite(props.instanceId, group.value.jid)
    invite.value = fetched.invite_code
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.groups.inviteFailed')
  }
}

async function onResetInvite() {
  if (!group.value) {
    return
  }
  failure.value = null
  try {
    const fetched = await resetInvite(props.instanceId, group.value.jid)
    invite.value = fetched.invite_code
    failure.value = null
    notice.value = t('instances.groups.inviteResetDone')
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.groups.inviteFailed')
  }
}

async function onCopyInvite() {
  if (!invite.value) {
    return
  }
  try {
    await navigator.clipboard.writeText(invite.value)
    toast.add({ title: t('instances.groups.inviteCopied'), icon: 'i-lucide-check', color: 'success' })
  } catch {
    failure.value = t('instances.groups.inviteFailed')
  }
}

async function onPhoto() {
  if (!group.value || !photoFile.value) {
    failure.value = t('instances.send.fileRequired')
    return
  }
  if (!photoFile.value.type.startsWith('image/')) {
    failure.value = t('instances.send.unsupportedFile')
    return
  }
  failure.value = null
  try {
    await setGroupPhoto(props.instanceId, group.value.jid, photoFile.value)
    photoFile.value = null
    toast.add({ title: t('instances.groups.photoUpdated'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.groups.photoFailed')
  }
}

async function onParticipants() {
  if (!group.value) {
    return
  }
  const jids = participantJids.value.split('\n').map(line => line.trim()).filter(line => line !== '')
  if (jids.length === 0) {
    failure.value = t('instances.groups.participantsHint')
    return
  }
  failure.value = null
  try {
    await updateParticipants(props.instanceId, group.value.jid, { action: participantAction.value, participants: jids })
    participantJids.value = ''
    toast.add({ title: t('instances.groups.participantsUpdated'), icon: 'i-lucide-check', color: 'success' })
    await onLookup()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.groups.participantsFailed')
  }
}

async function onLeave() {
  if (!group.value) {
    return
  }
  try {
    await leaveGroup(props.instanceId, group.value.jid)
    group.value = null
    invite.value = null
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}
</script>

<template>
  <UPageCard variant="subtle" data-testid="group-detail" :ui="{ container: 'p-0 sm:p-0 gap-y-0', wrapper: 'items-stretch', header: 'p-4 mb-0 border-b border-default' }">
    <template #header>
      <div class="flex min-w-0 flex-col gap-2">
        <UAlert
          v-if="!canAct"
          color="warning"
          variant="subtle"
          :title="t('instances.actions.notConnected')"
        />
        <UInput v-model="jid" :placeholder="t('instances.groups.search')" class="w-full font-mono" />
        <div class="flex flex-wrap gap-2">
          <UButton :disabled="!canAct" :label="t('instances.groups.lookup')" @click="onLookup" />
          <UButton
            :disabled="!canAct"
            variant="soft"
            :label="t('instances.groups.create')"
            @click="createOpen = true"
          />
          <UButton
            :disabled="!canAct"
            variant="soft"
            :label="t('instances.groups.join')"
            @click="joinOpen = true"
          />
        </div>
      </div>
    </template>
    <UAlert
      v-if="failure"
      color="error"
      variant="subtle"
      :title="failure"
    />
    <UAlert
      v-if="notice"
      color="warning"
      variant="subtle"
      :title="notice"
    />
    <div v-if="group" class="flex flex-col gap-4 px-4 py-3 sm:px-6">
      <div class="flex min-w-0 items-center gap-3">
        <UAvatar :alt="group.name" size="md" />
        <span class="min-w-0 flex-1">
          <span class="block truncate font-medium text-highlighted">{{ group.name }}</span>
          <span class="block truncate text-sm text-muted">{{ group.participant_count }} · {{ group.description || group.jid }}</span>
        </span>
      </div>
      <div class="flex flex-col gap-2">
        <UInput
          v-model="editName"
          maxlength="25"
          :placeholder="t('instances.groups.name')"
          class="w-full"
        />
        <UInput v-model="editDescription" :placeholder="t('instances.groups.description')" class="w-full" />
        <div class="flex justify-end">
          <UButton
            :disabled="!canAct"
            variant="soft"
            :label="t('instances.groups.update')"
            @click="onUpdate"
          />
        </div>
      </div>
      <div class="flex flex-col gap-2">
        <p class="font-mono text-sm break-all text-muted">
          {{ t('instances.groups.inviteCode') }}: {{ invite || t('common.notSet') }}
        </p>
        <div class="flex flex-wrap gap-2">
          <UButton
            :disabled="!canAct"
            variant="soft"
            :label="t('instances.groups.getInvite')"
            @click="onGetInvite"
          />
          <UButton
            :disabled="!canAct"
            variant="soft"
            :label="t('instances.groups.resetInvite')"
            @click="onResetInvite"
          />
          <UButton
            :disabled="!canAct || !invite"
            variant="soft"
            :label="t('instances.groups.copyInvite')"
            @click="onCopyInvite"
          />
        </div>
      </div>
      <div class="flex flex-col gap-2">
        <UFileUpload
          v-model="photoFile"
          accept="image/*"
          :hint="t('instances.groups.photoHint')"
          variant="area"
        />
        <div class="flex justify-end">
          <UButton
            :disabled="!canAct || !photoFile"
            variant="soft"
            :label="t('instances.groups.uploadPhoto')"
            @click="onPhoto"
          />
        </div>
      </div>
      <div class="flex flex-col gap-2">
        <USelect
          v-model="participantAction"
          :items="participantActions"
          :placeholder="t('instances.groups.participantsAction')"
          class="w-full"
        />
        <UTextarea
          v-model="participantJids"
          :rows="3"
          :placeholder="t('instances.groups.participants')"
          :hint="t('instances.groups.participantsHint')"
          class="w-full font-mono"
        />
        <div class="flex justify-end">
          <UButton
            :disabled="!canAct"
            variant="soft"
            :label="t('instances.groups.participants')"
            @click="onParticipants"
          />
        </div>
      </div>
      <ul class="divide-y divide-default">
        <li v-for="member in group.participants" :key="member.jid" class="flex min-w-0 items-center gap-3 py-2">
          <UAvatar :alt="member.jid" size="xs" />
          <span class="min-w-0 flex-1 truncate font-mono text-xs text-muted">{{ member.jid }}</span>
        </li>
      </ul>
    </div>
    <UEmpty v-else icon="i-lucide-users" :title="t('instances.groups.empty')" />
    <UModal v-model:open="createOpen" :title="t('instances.groups.create')">
      <template #body>
        <UInput
          v-model="newName"
          maxlength="25"
          :placeholder="t('instances.groups.name')"
          class="w-full"
        />
      </template>
      <template #footer>
        <UButton :label="t('instances.groups.create')" @click="onCreate" />
      </template>
    </UModal>
    <UModal v-model:open="joinOpen" :title="t('instances.groups.join')">
      <template #body>
        <UInput v-model="inviteCode" :placeholder="t('instances.groups.invitePlaceholder')" class="w-full font-mono" />
      </template>
      <template #footer>
        <UButton :label="t('instances.groups.join')" @click="onJoin" />
      </template>
    </UModal>
    <template v-if="group" #footer>
      <UButton
        :disabled="!canAct"
        color="error"
        variant="soft"
        :label="t('instances.groups.leave')"
        @click="onLeave"
      />
    </template>
  </UPageCard>
</template>
