<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'
import type { Instance } from '~/types/api'

// Row actions for the instances table. The cell only emits: the page keeps
// the connect call, the edit modal and the delete modal (and all API logic),
// mapping open -> detail navigation, connect -> pairing start,
// edit -> openEdit and remove -> openDelete.
withDefaults(defineProps<{
  instance: Instance
  icon?: string
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
}>(), {
  icon: 'i-lucide-ellipsis',
  size: 'md'
})

const emit = defineEmits<{
  (e: 'open' | 'connect' | 'edit' | 'remove', instance: Instance): void
}>()

const { t } = useI18n()

function menuItems(instance: Instance): DropdownMenuItem[] {
  const items: DropdownMenuItem[] = [
    {
      label: t('instances.actions.open'),
      icon: 'i-lucide-eye',
      onSelect: () => emit('open', instance)
    }
  ]
  if (instance.connection.status !== 'connected') {
    items.push({
      label: t('instances.actions.connect'),
      icon: 'i-lucide-qr-code',
      onSelect: () => emit('connect', instance)
    })
  }
  items.push(
    {
      label: t('instances.actions.edit'),
      icon: 'i-lucide-pencil',
      onSelect: () => emit('edit', instance)
    },
    {
      label: t('instances.actions.delete'),
      icon: 'i-lucide-trash-2',
      onSelect: () => emit('remove', instance)
    }
  )
  return items
}
</script>

<template>
  <UDropdownMenu :items="menuItems(instance)" :content="{ align: 'end' }">
    <UButton
      color="neutral"
      variant="ghost"
      :icon="icon"
      :size="size"
      :aria-label="t('instances.table.actions')"
    />
  </UDropdownMenu>
</template>
