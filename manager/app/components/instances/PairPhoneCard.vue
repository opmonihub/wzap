<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, PairPhoneResult } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const { pairPhone } = useInstanceProfile()

const phone = ref('')
const requesting = ref(false)
const failure = ref<string | null>(null)
const result = ref<PairPhoneResult | null>(null)

const canRequest = computed(() => props.status !== 'connected' && !requesting.value)

async function onRequest() {
  if (requesting.value || phone.value.trim() === '') {
    return
  }
  requesting.value = true
  failure.value = null
  result.value = null
  try {
    result.value = await pairPhone(props.instanceId, phone.value)
  } catch (error) {
    if (error instanceof ApiError && error.status === 409) {
      failure.value = t('instances.pairPhone.connectFirst')
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.pairPhone.failed')
    }
  } finally {
    requesting.value = false
  }
}

function formatExpiry(value: string): string {
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}
</script>

<template>
  <UCard data-testid="pair-phone-card">
    <template #header>
      <h2 class="font-medium text-highlighted">
        {{ t('instances.pairPhone.cardTitle') }}
      </h2>
    </template>
    <div class="flex flex-col gap-3">
      <UAlert color="info" variant="subtle" :title="t('instances.pairPhone.connectFirst')" />
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />
      <div v-if="result" class="flex flex-col gap-1">
        <span class="font-mono text-2xl tracking-widest text-highlighted">{{ result.pairing_code }}</span>
        <span class="text-sm text-muted">{{ t('instances.pairPhone.expiresAt', { when: formatExpiry(result.expires_at) }) }}</span>
      </div>
      <div class="flex flex-col gap-2 sm:flex-row">
        <UInput
          v-model="phone"
          maxlength="32"
          :placeholder="t('instances.pairPhone.phone')"
          class="w-full font-mono"
        />
        <UButton
          :disabled="!canRequest"
          :loading="requesting"
          :label="requesting ? t('instances.pairPhone.requesting') : t('instances.pairPhone.request')"
          @click="onRequest"
        />
      </div>
    </div>
  </UCard>
</template>
