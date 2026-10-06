<script setup lang="ts">
import InstanceStatusBadge from './InstanceStatusBadge.vue'
import type { Instance } from '~/types/api'

// Status badge with a next-step action: disconnected and error rows offer
// Connect inline, pairing rows link to the QR on the detail screen, and
// connected rows show the badge alone. The page owns the connect call.
defineProps<{
  instance: Instance
}>()

defineEmits<{
  (e: 'connect', instance: Instance): void
}>()

const { t } = useI18n()
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <InstanceStatusBadge :status="instance.connection.status" />
    <UButton
      v-if="instance.connection.status === 'disconnected' || instance.connection.status === 'error'"
      color="primary"
      variant="ghost"
      size="xs"
      icon="i-lucide-qr-code"
      :label="t('instances.statusCell.connect')"
      @click.stop="$emit('connect', instance)"
    />
    <NuxtLink
      v-else-if="instance.connection.status === 'pairing'"
      :to="`/instances/${instance.id}`"
      class="text-xs font-medium text-primary underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      @click.stop
    >
      {{ t('instances.statusCell.viewQr') }}
    </NuxtLink>
  </div>
</template>
