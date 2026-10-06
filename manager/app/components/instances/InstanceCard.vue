<script setup lang="ts">
import InstanceStatusBadge from '~/components/instances/InstanceStatusBadge.vue'
import InstancesTableActionsCell from '~/components/instances/InstancesTableActionsCell.vue'
import type { Instance, InstanceStatus } from '~/types/api'

// Card tile for the instances list, mirroring research/wzap/web sessions
// index: an identity row (avatar + status dot, name, kebab + status badge),
// a metadata row (updated date) and an Open footer with connect/edit/delete
// icon actions. The remodeled public DTO hides owner_user_id, external_ref
// and whatsapp_jid, so the card no longer renders them. The card only emits:
// the page keeps the detail navigation, the connect call and the edit/delete
// modals.
const props = defineProps<{
  instance: Instance
}>()

const emit = defineEmits<{
  (e: 'open' | 'connect' | 'edit' | 'remove', instance: Instance): void
}>()

const { t } = useI18n()

const dotClass: Record<InstanceStatus, string> = {
  connected: 'bg-success',
  pairing: 'bg-info',
  error: 'bg-error',
  disconnected: 'bg-muted'
}

const initials = computed(() => {
  const words = props.instance.name.trim().split(/\s+/).filter(Boolean)
  const picked = words.slice(0, 2).map(word => [...word][0] ?? '').join('')
  return (picked || props.instance.name.slice(0, 2)).toUpperCase()
})

// Mirrors InstancesTableStatusCell: disconnected/error offer Connect, pairing
// links to the QR on the detail screen, connected shows nothing extra.
const showConnect = computed(() => props.instance.connection.status === 'disconnected' || props.instance.connection.status === 'error')
const showViewQr = computed(() => props.instance.connection.status === 'pairing')

function formatDate(value: string): string {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }
  const day = String(parsed.getDate()).padStart(2, '0')
  const month = String(parsed.getMonth() + 1).padStart(2, '0')
  return `${day}/${month}/${parsed.getFullYear()}`
}

// Whole-card navigation like the table's onSelect, except when the click lands
// on an interactive element (links, buttons, menu items).
function onCardClick(event: Event) {
  const target = event.target as HTMLElement | null
  if (target?.closest('a, button, input, [role="menuitem"], [role="menuitemcheckbox"]')) {
    return
  }
  emit('open', props.instance)
}
</script>

<template>
  <UCard
    class="flex min-w-0 cursor-pointer flex-col transition-colors hover:border-primary/40"
    :ui="{ body: 'flex-1 !p-0' }"
    @click="onCardClick"
  >
    <div class="flex items-center gap-3 px-4 py-3">
      <div class="relative shrink-0">
        <span
          class="flex size-9 items-center justify-center rounded-full bg-elevated text-xs font-bold ring-1 ring-default"
          :title="instance.name"
        >
          {{ initials }}
        </span>
        <span
          class="absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full ring-2 ring-white dark:ring-gray-900"
          :class="dotClass[instance.connection.status]"
          aria-hidden="true"
        />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center justify-between gap-1">
          <NuxtLink
            :to="`/instances/${instance.id}`"
            class="truncate text-sm font-semibold text-highlighted hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            :title="instance.name"
            @click.stop
          >
            {{ instance.name }}
          </NuxtLink>
          <InstancesTableActionsCell
            :instance="instance"
            class="shrink-0"
            icon="i-lucide-ellipsis-vertical"
            size="xs"
            @open="(item) => emit('open', item)"
            @connect="(item) => emit('connect', item)"
            @edit="(item) => emit('edit', item)"
            @remove="(item) => emit('remove', item)"
          />
        </div>
        <div class="mt-0.5 flex min-w-0 items-center gap-1.5 text-xs text-muted">
          <InstanceStatusBadge :status="instance.connection.status" size="xs" class="shrink-0 capitalize" />
          <UButton
            v-if="showConnect"
            color="primary"
            variant="ghost"
            size="xs"
            class="-ml-1 shrink-0 px-1"
            :label="t('instances.statusCell.connect')"
            @click.stop="emit('connect', instance)"
          />
          <NuxtLink
            v-else-if="showViewQr"
            :to="`/instances/${instance.id}`"
            class="shrink-0 text-xs font-medium text-primary underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            @click.stop
          >
            {{ t('instances.statusCell.viewQr') }}
          </NuxtLink>
        </div>
      </div>
    </div>

    <div class="border-t border-default px-4 py-2">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
        <span class="flex shrink-0 items-center gap-1">
          <UIcon name="i-lucide-clock" class="size-3" aria-hidden="true" />
          {{ formatDate(instance.updated_at) }}
        </span>
      </div>
    </div>

    <div class="flex items-center gap-1.5 border-t border-default px-4 py-2">
      <UButton
        icon="i-lucide-arrow-right"
        :label="t('instances.actions.open')"
        size="xs"
        color="primary"
        variant="soft"
        class="flex-1"
        @click="emit('open', instance)"
      />
      <UButton
        v-if="showConnect"
        icon="i-lucide-plug"
        size="xs"
        color="neutral"
        variant="ghost"
        :aria-label="t('instances.actions.connect')"
        @click="emit('connect', instance)"
      />
      <UButton
        icon="i-lucide-pencil"
        size="xs"
        color="neutral"
        variant="ghost"
        :aria-label="t('instances.actions.edit')"
        @click="emit('edit', instance)"
      />
      <UButton
        icon="i-lucide-trash-2"
        size="xs"
        color="error"
        variant="ghost"
        :aria-label="t('instances.actions.delete')"
        @click="emit('remove', instance)"
      />
    </div>
  </UCard>
</template>
