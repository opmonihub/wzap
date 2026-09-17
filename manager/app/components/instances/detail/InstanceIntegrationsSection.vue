<script setup lang="ts">
import ChatwootCard from '~/components/instances/ChatwootCard.vue'
import WebhookCard from '~/components/instances/WebhookCard.vue'
import type { Instance } from '~/types/api'

defineProps<{
  instance: Instance
}>()

const emit = defineEmits<{
  'webhook-updated': [instance: Instance]
}>()
</script>

<template>
  <div class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
    <WebhookCard
      :instance="instance"
      @updated="(value: Instance) => emit('webhook-updated', value)"
    />

    <ChatwootCard :instance-id="instance.id" :status="instance.status" />
  </div>
</template>
