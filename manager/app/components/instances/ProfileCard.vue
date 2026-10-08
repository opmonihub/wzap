<script setup lang="ts">
import FileUploadPreview from '~/components/shared/FileUploadPreview.vue'
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, Profile, UpdateProfileInput } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const toast = useToast()
const { getProfile, updateProfile, setProfilePhoto } = useInstanceProfile()

const profile = ref<Profile | null>(null)
const name = ref('')
const recado = ref('')
const photoFile = ref<File | null>(null)
const failure = ref<string | null>(null)
const unsupported = ref(false)
const canAct = computed(() => props.status === 'connected')

async function load() {
  try {
    profile.value = await getProfile(props.instanceId)
    name.value = profile.value.name ?? ''
    recado.value = profile.value.status_text ?? ''
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.messages.loadFailed')
  }
}

async function onSave() {
  failure.value = null
  unsupported.value = false
  const trimmedName = name.value.trim()
  const input: UpdateProfileInput = {}
  if (trimmedName !== (profile.value?.name ?? '').trim()) {
    input.name = trimmedName
  }
  if (recado.value !== (profile.value?.status_text ?? '')) {
    input.status_text = recado.value
  }
  if (input.name !== undefined && ([...input.name].length === 0 || [...input.name].length > 100)) {
    failure.value = t('instances.profile.nameHint')
    return
  }
  if (input.status_text !== undefined && [...input.status_text].length > 500) {
    failure.value = t('instances.profile.recadoHint')
    return
  }
  if (input.name === undefined && input.status_text === undefined) {
    return
  }
  try {
    profile.value = await updateProfile(props.instanceId, input)
    name.value = profile.value.name ?? ''
    recado.value = profile.value.status_text ?? ''
    toast.add({ title: t('instances.profile.saved'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      unsupported.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
    }
  }
}

async function onPhoto() {
  if (!photoFile.value) {
    failure.value = t('instances.send.fileRequired')
    return
  }
  if (!photoFile.value.type.startsWith('image/')) {
    failure.value = t('instances.send.unsupportedFile')
    return
  }
  failure.value = null
  unsupported.value = false
  try {
    await setProfilePhoto(props.instanceId, photoFile.value)
    photoFile.value = null
    toast.add({ title: t('instances.profile.photoUpdated'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      unsupported.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.profile.photoFailed')
    }
  }
}

// GET /profile answers 409 while disconnected; skip the load so section
// visits stay quiet until the session can answer.
watch(() => [props.instanceId, props.status] as const, () => {
  if (props.status !== 'connected') {
    profile.value = null
    failure.value = null
    return
  }
  void load()
}, { immediate: true })
</script>

<template>
  <UPageCard :title="t('instances.profile.cardTitle')" variant="subtle" data-testid="profile-card">
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
      <UFormField :label="t('instances.profile.recado')" :hint="t('instances.profile.recadoHint')">
        <UInput v-model="recado" maxlength="500" class="w-full" />
      </UFormField>
      <div class="flex justify-end">
        <UButton :disabled="!canAct" :label="t('common.save')" @click="onSave" />
      </div>
      <UFormField :label="t('instances.profile.photo')" :hint="t('instances.profile.photoHint')">
        <UFileUpload
          v-model="photoFile"
          :label="t('instances.profile.photo')"
          accept="image/*"
          variant="area"
        >
          <template #file-leading="{ file, ui }">
            <FileUploadPreview :file="file" :class="ui.fileLeadingAvatar()" />
          </template>
        </UFileUpload>
      </UFormField>
      <div class="flex justify-end">
        <UButton
          :disabled="!canAct || !photoFile"
          variant="soft"
          :label="t('instances.profile.uploadPhoto')"
          @click="onPhoto"
        />
      </div>
    </div>
  </UPageCard>
</template>
