<script setup lang="ts">
import type { Instance } from '~/types/api'

const props = defineProps<{
  instance: Instance
  email?: string
}>()

// Mirrors ownerLabel in useInstancesTable: the resolved account email when
// known, the short id as best-effort fallback, an em dash when ownerless or
// still unresolved (empty table cells always render as —).
const label = computed(() => {
  if (props.email) {
    return props.email
  }
  if (!props.instance.owner_user_id) {
    return '—'
  }
  return props.instance.owner_user_id.slice(0, 8)
})
</script>

<template>
  <span class="block min-w-0 max-w-48 truncate text-highlighted" :title="label">
    {{ label }}
  </span>
</template>
