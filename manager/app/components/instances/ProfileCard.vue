<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, Profile } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const { getProfile, updateProfile } = useInstanceProfile()

const profile = ref<Profile | null>(null)
const name = ref('')
const recado = ref('')
const failure = ref<string | null>(null)
const unsupported = ref(false)
const canAct = computed(() => props.status === 'connected')

async function load() {
  try {
    profile.value = await getProfile(props.instanceId)
    name.value = profile.value.name
    recado.value = profile.value.status_text
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.messages.loadFailed')
  }
}

async function onSave() {
  failure.value = null
  unsupported.value = false
  const trimmedName = name.value.trim()
  if ([...trimmedName].length === 0 || [...trimmedName].length > 100 || [...recado.value].length > 500) {
    return
  }
  try {
    profile.value = await updateProfile(props.instanceId, { name: trimmedName, status_text: recado.value })
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      unsupported.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
    }
  }
}

watch(() => props.instanceId, () => void load(), { immediate: true })
</script>

<template>
  <UPageCard variant="subtle" data-testid="profile-card">
    <template #header>
      <h3 class="font-medium text-highlighted">
        {{ t('instances.profile.cardTitle') }}
      </h3>
    </template>
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
      <UAlert
        v-if="unsupported"
        color="warning"
        variant="subtle"
        :title="t('instances.profile.unsupported')"
      />
      <UFormField :label="t('instances.fields.name')" :hint="t('instances.profile.nameHint')">
        <UInput v-model="name" maxlength="100" class="w-full" />
      </UFormField>
      <UFormField :label="t('instances.fields.lastError')" :hint="t('instances.profile.recadoHint')">
        <UInput v-model="recado" maxlength="500" class="w-full" />
      </UFormField>
      <div class="flex justify-end">
        <UButton :disabled="!canAct" :label="t('common.save')" @click="onSave" />
      </div>
    </div>
  </UPageCard>
</template>
