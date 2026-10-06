<script setup lang="ts">
import type { AccountUser } from '~/types/api'

const props = defineProps<{
  user: AccountUser
}>()

const { t } = useI18n()

// Usage comes from the backend-computed instances_used field of the user
// DTO (every owned instance, any state); instance_limit 0 means unlimited.
const usage = computed(() => props.user.instances_used)

// Mirrors quotaLabel in useAccountsTable: limit 0 means unlimited, any other
// limit renders as its number. Keep both in sync.
const quotaText = computed(() => {
  return props.user.instance_limit === 0 ? t('accounts.unlimited') : String(props.user.instance_limit)
})

const percent = computed(() => {
  if (props.user.instance_limit === 0) {
    return 0
  }
  return Math.min(100, Math.round((usage.value / props.user.instance_limit) * 100))
})

const barColor = computed(() => {
  if (props.user.instance_limit !== 0 && usage.value >= props.user.instance_limit) {
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
      v-if="user.instance_limit !== 0"
      :model-value="percent"
      :max="100"
      :color="barColor"
      size="xs"
      :aria-label="t('accounts.quota.usageTitle', { used: usage, quota: quotaText })"
    />
  </div>
</template>
