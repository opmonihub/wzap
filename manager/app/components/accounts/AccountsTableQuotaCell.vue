<script setup lang="ts">
import type { AccountUser } from '~/types/api'

const props = defineProps<{
  user: AccountUser
  // Instances owned by this account, counted client-side from the instance
  // list (GET /users exposes no usage field). Quota 0 means unlimited.
  usage: number
}>()

const { t } = useI18n()

// Mirrors quotaLabel in useAccountsTable: quota 0 means unlimited, any other
// quota renders as its number. Keep both in sync.
const quotaText = computed(() => {
  return props.user.instance_quota === 0 ? t('accounts.unlimited') : String(props.user.instance_quota)
})

const percent = computed(() => {
  if (props.user.instance_quota === 0) {
    return 0
  }
  return Math.min(100, Math.round((props.usage / props.user.instance_quota) * 100))
})

const barColor = computed(() => {
  if (props.user.instance_quota !== 0 && props.usage >= props.user.instance_quota) {
    return 'error'
  }
  if (percent.value >= 80) {
    return 'warning'
  }
  return 'primary'
})
</script>

<template>
  <div class="flex min-w-36 flex-col gap-1">
    <span class="text-sm text-highlighted" :title="t('accounts.quota.usageTitle', { used: usage, quota: quotaText })">
      {{ usage }} / {{ quotaText }}
    </span>
    <UProgress
      v-if="user.instance_quota !== 0"
      :model-value="percent"
      :max="100"
      :color="barColor"
      size="xs"
      :aria-label="t('accounts.quota.usageTitle', { used: usage, quota: quotaText })"
    />
  </div>
</template>
