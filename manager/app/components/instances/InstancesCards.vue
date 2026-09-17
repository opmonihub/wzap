<script setup lang="ts">
import InstanceCard from '~/components/instances/InstanceCard.vue'
import type { Instance, InstanceStatus } from '~/types/api'

const props = defineProps<{
  items: Instance[]
  ownerEmails: Record<string, string>
  isAdmin: boolean
}>()

const emit = defineEmits<{
  open: [instance: Instance]
  connect: [instance: Instance]
  edit: [instance: Instance]
  remove: [instance: Instance]
  // Empty-state create button (the navbar create button stays in the page, so
  // this is the only create entry owned here).
  create: []
}>()

const { t } = useI18n()

// Card/grid view over the same loaded items as the table: search, status and
// sorting state stay local so the grid filters the accumulated cursor pages
// client-side (name/external_ref match + status equals), mirroring the table.
// Transcribed from pages/instances/index.vue (BASE 6b2d4e1).
const searchInput = ref('')

// Search input debounced into the grid filter (300ms): typing never
// re-filters per keystroke on 1000+ rows; the status select stays immediate.
const globalFilter = ref('')
watchDebounced(searchInput, (value) => {
  globalFilter.value = value
}, { debounce: 300 })

const columnFilters = ref<{ id: string, value: unknown }[]>([])
const sorting = ref<{ id: string, desc: boolean }[]>([])
const pagination = ref({ pageIndex: 0, pageSize: 10 })

// Status filter behind a USelect. Reka SelectItem forbids an empty value
// string (it throws and unmounts the page), so the "all" option uses the
// 'all' sentinel, mapped back to "no filter" in the setter.
const statusFilter = computed<string>({
  get: () => {
    const current = columnFilters.value.find(entry => entry.id === 'status')?.value
    return typeof current === 'string' && current !== '' ? current : 'all'
  },
  set: (value) => {
    const rest = columnFilters.value.filter(entry => entry.id !== 'status')
    columnFilters.value = value === '' || value === 'all' ? rest : [...rest, { id: 'status', value }]
  }
})

const instanceStatuses: InstanceStatus[] = ['disconnected', 'pairing', 'connected', 'error']

const statusFilterItems = computed(() => [
  { label: t('instances.table.allStatuses'), value: 'all' },
  ...instanceStatuses.map(status => ({ label: t(`instances.status.${status}`), value: status }))
])

const cardItems = computed(() => {
  const query = globalFilter.value.trim().toLowerCase()
  const status = columnFilters.value.find(entry => entry.id === 'status')?.value
  const filtered = props.items.filter((entry) => {
    if (typeof status === 'string' && status !== '' && entry.status !== status) {
      return false
    }
    if (query !== '' && ![entry.name, entry.external_ref].some(value => (value ?? '').toLowerCase().includes(query))) {
      return false
    }
    return true
  })
  const sort = sorting.value[0]
  if (!sort || (sort.id !== 'name' && sort.id !== 'status')) {
    return filtered
  }
  const direction = sort.desc ? -1 : 1
  const key = sort.id as 'name' | 'status'
  return [...filtered].sort((a, b) => String(a[key]).localeCompare(String(b[key])) * direction)
})

const cardPageCount = computed(() => Math.max(1, Math.ceil(cardItems.value.length / pagination.value.pageSize)))

const cardPageItems = computed(() => {
  const start = pagination.value.pageIndex * pagination.value.pageSize
  return cardItems.value.slice(start, start + pagination.value.pageSize)
})

// The toolbar owns the search input (debounced write-back into the grid
// filter), so clearing the search is a single globalFilter reset; the status
// filter stays immediate.
function clearFilters() {
  searchInput.value = ''
  globalFilter.value = ''
  columnFilters.value = []
}

function onUpdatePage(page: number) {
  pagination.value.pageIndex = page - 1
}

// Filter changes restart at the first page; a shrunken result only clamps an
// out-of-range page. Cursor accumulation (loadMore/onCreated) never resets
// the page the user is on.
watch([globalFilter, columnFilters, sorting], () => {
  pagination.value.pageIndex = 0
})

watch(cardPageCount, (count) => {
  if (pagination.value.pageIndex > count - 1) {
    pagination.value.pageIndex = count - 1
  }
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex flex-wrap items-center gap-1.5">
      <UInput
        v-model="searchInput"
        icon="i-lucide-search"
        :placeholder="t('instances.table.search')"
        :aria-label="t('instances.table.search')"
        class="max-w-xs"
      />
      <USelect
        v-model="statusFilter"
        :items="statusFilterItems"
        :aria-label="t('instances.table.statusFilter')"
        :placeholder="t('instances.table.statusFilter')"
        size="sm"
        class="w-36"
        :ui="{ trailingIcon: 'group-data-[state=open]:rotate-180 transition-transform duration-200' }"
      />
    </div>

    <div v-if="cardPageItems.length > 0" class="grid grid-cols-1 gap-4 sm:grid-cols-2 2xl:grid-cols-3">
      <InstanceCard
        v-for="entry in cardPageItems"
        :key="entry.id"
        :instance="entry"
        :email="ownerEmails[entry.owner_user_id ?? '']"
        :show-owner="isAdmin"
        @open="(instance: Instance) => emit('open', instance)"
        @connect="(instance: Instance) => emit('connect', instance)"
        @edit="(instance: Instance) => emit('edit', instance)"
        @remove="(instance: Instance) => emit('remove', instance)"
      />
    </div>

    <UEmpty
      v-else-if="items.length === 0"
      icon="i-lucide-search-x"
      :title="t('instances.empty')"
    >
      <template #actions>
        <UButton icon="i-lucide-plus" :label="t('instances.create.title')" @click="emit('create')" />
      </template>
    </UEmpty>
    <UEmpty
      v-else
      icon="i-lucide-search-x"
      :title="t('instances.table.noResults')"
    >
      <template #actions>
        <UButton
          color="neutral"
          variant="soft"
          :label="t('instances.table.clearFilters')"
          @click="clearFilters"
        />
      </template>
    </UEmpty>

    <div v-if="cardPageCount > 1" class="flex items-center justify-end gap-3 border-t border-default pt-4 mt-auto">
      <UPagination
        :page="pagination.pageIndex + 1"
        :items-per-page="pagination.pageSize"
        :total="cardItems.length"
        size="sm"
        @update:page="onUpdatePage"
      />
    </div>
  </div>
</template>
