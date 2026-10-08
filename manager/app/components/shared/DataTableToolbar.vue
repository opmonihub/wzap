<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'

withDefaults(defineProps<{
  searchPlaceholder: string
  searchAria: string
  filtersLabel: string
  displayLabel: string
  columnItems: DropdownMenuItem[]
}>(), {
  columnItems: () => []
})

// External filter state (the table's globalFilter). The visible input is
// local so keystrokes never re-filter per keystroke; it syncs back debounced.
const filter = defineModel<string>('filter', { default: '' })
const input = ref(filter.value)
watchDebounced(input, (value) => {
  filter.value = value
}, { debounce: 300 })
watch(filter, (value) => {
  if (value !== input.value) {
    input.value = value
  }
})
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-1.5" role="group" :aria-label="filtersLabel">
    <UInput
      v-model="input"
      icon="i-lucide-search"
      :placeholder="searchPlaceholder"
      :aria-label="searchAria"
      class="max-w-sm"
    />

    <div class="flex flex-wrap items-center gap-1.5">
      <slot name="bulk" />
      <slot name="filters" />
      <UDropdownMenu
        v-if="columnItems.length > 0"
        :items="[columnItems]"
        :content="{ align: 'end' }"
      >
        <UButton
          :label="displayLabel"
          color="neutral"
          variant="outline"
          trailing-icon="i-lucide-settings-2"
        />
      </UDropdownMenu>
    </div>
  </div>
</template>
