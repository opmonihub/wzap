<script setup lang="ts">
import type { NavigationMenuItem } from '#ui/types'

const { t } = useI18n()
const props = defineProps<{ section: string }>()
const emit = defineEmits<{ select: [value: string] }>()

// Section nav mirrors research/dashboard settings.vue: UNavigationMenu in a
// UDashboardToolbar with icons + highlight. Switching still rides on each
// item's onSelect (UNavigationMenu does not reliably emit update:modelValue
// for value-only items), while `active` + `model-value` keep the highlight
// pill in sync with the local section state instead of the route.
const items = computed<NavigationMenuItem[]>(() => [
  { label: t('instances.sections.overview'), icon: 'i-lucide-house', value: 'overview', active: props.section === 'overview', onSelect: () => emit('select', 'overview') },
  { label: t('instances.sections.messages'), icon: 'i-lucide-message-square-text', value: 'messages', active: props.section === 'messages', onSelect: () => emit('select', 'messages') },
  { label: t('instances.sections.groups'), icon: 'i-lucide-users', value: 'groups', active: props.section === 'groups', onSelect: () => emit('select', 'groups') },
  { label: t('instances.sections.channels'), icon: 'i-lucide-megaphone', value: 'channels', active: props.section === 'channels', onSelect: () => emit('select', 'channels') },
  { label: t('instances.sections.profile'), icon: 'i-lucide-user', value: 'profile', active: props.section === 'profile', onSelect: () => emit('select', 'profile') },
  { label: t('instances.sections.integrations'), icon: 'i-lucide-plug', value: 'integrations', active: props.section === 'integrations', onSelect: () => emit('select', 'integrations') },
  { label: t('instances.sections.settings'), icon: 'i-lucide-settings', value: 'settings', active: props.section === 'settings', onSelect: () => emit('select', 'settings') }
])
</script>

<template>
  <UDashboardToolbar>
    <!-- NOTE: The `-mx-1` class is used to align with the `DashboardSidebarCollapse` button here. -->
    <UNavigationMenu
      :model-value="section"
      highlight
      class="-mx-1 max-w-full min-w-0 flex-1 overflow-x-auto"
      :items="items"
    />
  </UDashboardToolbar>
</template>
