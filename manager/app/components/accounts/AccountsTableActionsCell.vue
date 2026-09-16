<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import type { AccountUser } from '~/types/api'

// Row actions for the accounts table behind an overflow menu. The cell only
// emits: the page keeps openQuota / openDelete (and all API logic), mapping
// edit-quota -> openQuota and remove -> openDelete. Deleting the signed-in
// account is blocked: the item stays disabled with a hint.
const props = defineProps<{
  user: AccountUser
  isSelf: boolean
}>()

const emit = defineEmits<{
  (e: 'edit-quota' | 'remove', user: AccountUser): void
}>()

const { t } = useI18n()

const items = computed<DropdownMenuItem[]>(() => [
  {
    label: t('accounts.quota.edit'),
    icon: 'i-lucide-pencil',
    onSelect: () => emit('edit-quota', props.user)
  },
  {
    label: props.isSelf ? t('accounts.delete.selfBlocked') : t('accounts.delete.action'),
    icon: 'i-lucide-trash-2',
    disabled: props.isSelf,
    onSelect: () => emit('remove', props.user)
  }
])
</script>

<template>
  <UDropdownMenu :items="items" :content="{ align: 'end' }">
    <UButton
      color="neutral"
      variant="ghost"
      icon="i-lucide-ellipsis"
      :aria-label="t('accounts.table.actions')"
    />
  </UDropdownMenu>
</template>
