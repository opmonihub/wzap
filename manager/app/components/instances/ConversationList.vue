<script setup lang="ts" generic="T extends { id: string }">
const props = defineProps<{
  items: T[]
  emptyTitle: string
}>()

const selected = defineModel<string | null>({ default: null })

function titleOf(item: T): string {
  return (item as unknown as Record<string, string>).title
    ?? (item as unknown as Record<string, string>).name
    ?? (item as unknown as Record<string, string>).recipient
    ?? item.id
}

function subtitleOf(item: T): string {
  const record = item as unknown as Record<string, unknown>
  if (typeof record.description === 'string' && record.description !== '') {
    return record.description
  }
  if (typeof record.participant_count === 'number') {
    return String(record.participant_count)
  }
  return (item as unknown as { recipient?: string }).recipient ?? item.id
}

function select(id: string) {
  selected.value = id
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') {
    return
  }
  event.preventDefault()
  const ids = props.items.map(item => item.id)
  const current = selected.value === null ? -1 : ids.indexOf(selected.value)
  const next = event.key === 'ArrowDown'
    ? Math.min(ids.length - 1, current + 1)
    : Math.max(0, current - 1)
  const id = ids[next]
  if (id) {
    selected.value = id
    document.getElementById(`row-${id}`)?.scrollIntoView({ block: 'nearest' })
  }
}

defineShortcuts({
  arrowdown: () => onKeydown(new KeyboardEvent('keydown', { key: 'ArrowDown' })),
  arrowup: () => onKeydown(new KeyboardEvent('keydown', { key: 'ArrowUp' }))
})
</script>

<template>
  <ul v-if="props.items.length > 0" class="divide-y divide-default" @keydown="onKeydown">
    <li v-for="item in props.items" :id="`row-${item.id}`" :key="item.id">
      <button
        type="button"
        class="flex w-full items-center gap-3 border-l-2 border-transparent px-4 py-3 text-left"
        :class="selected === item.id ? 'border-primary bg-primary/10' : ''"
        @click="select(item.id)"
      >
        <UAvatar :alt="titleOf(item)" size="md" />
        <span class="min-w-0 flex-1">
          <span class="block truncate font-medium text-highlighted">{{ titleOf(item) }}</span>
          <span class="block truncate text-sm text-muted">{{ subtitleOf(item) }}</span>
        </span>
      </button>
    </li>
  </ul>
  <UEmpty v-else icon="i-lucide-inbox" :title="props.emptyTitle" />
</template>
