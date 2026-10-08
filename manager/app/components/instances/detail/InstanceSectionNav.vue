<script setup lang="ts">
import type { NavigationMenuItem } from '#ui/types'

const { t } = useI18n()
const props = defineProps<{ section: string }>()
const emit = defineEmits<{ select: [value: string] }>()

const sections = computed(() => [
  { label: t('instances.sections.overview'), icon: 'i-lucide-house', value: 'overview' },
  { label: t('instances.sections.messages'), icon: 'i-lucide-message-square-text', value: 'messages' },
  { label: t('instances.sections.groups'), icon: 'i-lucide-users', value: 'groups' },
  { label: t('instances.sections.channels'), icon: 'i-lucide-megaphone', value: 'channels' },
  { label: t('instances.sections.profile'), icon: 'i-lucide-user', value: 'profile' },
  { label: t('instances.sections.integrations'), icon: 'i-lucide-plug', value: 'integrations' },
  { label: t('instances.sections.settings'), icon: 'i-lucide-settings', value: 'settings' }
])

// NavigationMenu value-only items need onSelect; the mobile select uses its
// model update instead so choosing one section emits only once.
const items = computed<NavigationMenuItem[]>(() => sections.value.map(item => ({
  ...item,
  active: props.section === item.value,
  onSelect: () => emit('select', item.value)
})))
</script>

<template>
  <UDashboardToolbar>
    <USelect
      :model-value="section"
      :items="sections"
      :aria-label="t('instances.sections.label')"
      size="lg"
      class="min-h-11 w-full min-w-0 lg:hidden"
      @update:model-value="value => emit('select', value)"
    />
    <!-- NOTE: The `-mx-1` class is used to align with the `DashboardSidebarCollapse` button here. -->
    <UNavigationMenu
      :model-value="section"
      highlight
      class="-mx-1 hidden max-w-full min-w-0 flex-1 overflow-x-auto lg:flex"
      :items="items"
    />
  </UDashboardToolbar>
</template>
