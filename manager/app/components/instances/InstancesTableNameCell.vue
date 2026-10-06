<script setup lang="ts">
import type { Instance } from '~/types/api'

defineProps<{
  instance: Instance
}>()
</script>

<template>
  <div class="min-w-0 flex-1">
    <p class="truncate font-medium text-highlighted" :title="instance.name">
      {{ instance.name }}
    </p>
  </div>
</template>
