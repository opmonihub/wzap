<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, Privacy } from '~/types/api'

// Privacy get/put in one subtle card: every field renders its allowlist
// select (last_seen/profile_photo/status/groups_add take the four-valued
// literals, read_receipts takes all|none) and save ships the patch with at
// least one field.
const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const toast = useToast()
const { getPrivacy, setPrivacy } = useInstanceProfile()

const privacy = ref<Privacy | null>(null)
const lastSeen = ref('all')
const profilePhoto = ref('all')
const statusPrivacy = ref('all')
const readReceipts = ref('all')
const groupsAdd = ref('all')
const failure = ref<string | null>(null)
const canAct = computed(() => props.status === 'connected')

const allowOptions = computed(() => [
  { label: t('instances.privacy.allowAll'), value: 'all' },
  { label: t('instances.privacy.allowContacts'), value: 'contacts' },
  { label: t('instances.privacy.allowBlacklist'), value: 'contact_blacklist' },
  { label: t('instances.privacy.allowNone'), value: 'none' }
])

const receiptOptions = computed(() => [
  { label: t('instances.privacy.allowAll'), value: 'all' },
  { label: t('instances.privacy.allowNone'), value: 'none' }
])

async function load() {
  failure.value = null
  try {
    privacy.value = await getPrivacy(props.instanceId)
    lastSeen.value = privacy.value.last_seen
    profilePhoto.value = privacy.value.profile_photo
    statusPrivacy.value = privacy.value.status
    readReceipts.value = privacy.value.read_receipts
    groupsAdd.value = privacy.value.groups_add
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.privacy.loadFailed')
  }
}

async function onSave() {
  failure.value = null
  try {
    privacy.value = await setPrivacy(props.instanceId, {
      last_seen: lastSeen.value as Privacy['last_seen'],
      profile_photo: profilePhoto.value as Privacy['profile_photo'],
      status: statusPrivacy.value as Privacy['status'],
      read_receipts: readReceipts.value as Privacy['read_receipts'],
      groups_add: groupsAdd.value as Privacy['groups_add']
    })
    toast.add({ title: t('instances.privacy.saved'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.privacy.saveFailed')
  }
}

watch(() => props.instanceId, () => void load(), { immediate: true })
</script>

<template>
  <UPageCard :title="t('instances.privacy.cardTitle')" variant="subtle" data-testid="privacy-card">
    <div class="flex flex-col gap-3">
      <UAlert
        v-if="!canAct"
        color="warning"
        variant="subtle"
        :title="t('instances.actions.notConnected')"
      />
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />
      <UFormField :label="t('instances.privacy.lastSeen')">
        <USelect v-model="lastSeen" :items="allowOptions" class="w-full" />
      </UFormField>
      <UFormField :label="t('instances.privacy.profilePhoto')">
        <USelect v-model="profilePhoto" :items="allowOptions" class="w-full" />
      </UFormField>
      <UFormField :label="t('instances.privacy.status')">
        <USelect v-model="statusPrivacy" :items="allowOptions" class="w-full" />
      </UFormField>
      <UFormField :label="t('instances.privacy.readReceipts')">
        <USelect v-model="readReceipts" :items="receiptOptions" class="w-full" />
      </UFormField>
      <UFormField :label="t('instances.privacy.groupsAdd')">
        <USelect v-model="groupsAdd" :items="allowOptions" class="w-full" />
      </UFormField>
      <div class="flex justify-end">
        <UButton :disabled="!canAct" :label="t('common.save')" @click="onSave" />
      </div>
    </div>
  </UPageCard>
</template>
