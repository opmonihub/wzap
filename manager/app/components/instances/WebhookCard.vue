<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import { WEBHOOK_EVENT_TYPES } from '~/types/api'
import type { Instance } from '~/types/api'

// Webhook editor for one instance. Anyone operating the instance may save:
// the PATCH carries the nested webhook block only so name and external_ref
// stay stored. An explicit empty url unsets the webhook and an explicit
// empty events list clears the subscription; both mean no deliveries, as
// does enabled off. Validation failures (422) mirror the server message
// verbatim and persist nothing server-side.
const props = defineProps<{
  instance: Instance
}>()

const emit = defineEmits<{
  updated: [instance: Instance]
}>()

const { t } = useI18n()
const toast = useToast()
const { updateInstanceWebhook } = useInstances()

const schema = z.object({
  url: z.string().max(2048),
  enabled: z.boolean(),
  events: z.array(z.string())
}).refine(data => !data.enabled || /^https?:\/\//.test(data.url), {
  message: t('instances.webhook.urlRequired'),
  path: ['url']
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({
  url: props.instance.integration.webhook.url ?? '',
  enabled: props.instance.integration.webhook.enabled,
  events: [...props.instance.integration.webhook.events]
})
const saving = ref(false)
const failure = ref<string | null>(null)

const subscribedCount = computed(() => state.events?.length ?? 0)

function isSubscribed(type: string): boolean {
  return state.events?.includes(type) ?? false
}

function setSubscribed(type: string, value: boolean) {
  const current = new Set(state.events ?? [])
  if (value) {
    current.add(type)
  } else {
    current.delete(type)
  }
  state.events = WEBHOOK_EVENT_TYPES.filter(entry => current.has(entry))
}

function selectAll() {
  state.events = [...WEBHOOK_EVENT_TYPES]
}

function selectNone() {
  state.events = []
}

async function onSave(event: FormSubmitEvent<Schema>) {
  if (saving.value) {
    return
  }
  saving.value = true
  failure.value = null
  try {
    const updated = await updateInstanceWebhook(props.instance.id, {
      url: (event.data.url ?? '').trim(),
      enabled: event.data.enabled ?? false,
      events: event.data.events ?? []
    })
    emit('updated', updated)
    toast.add({ title: t('instances.webhook.saved'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.webhook.saveFailed')
  } finally {
    saving.value = false
  }
}

// A fresh instance row (after pairing reloads or navigation) replaces the
// edited values with the stored configuration.
watch(() => props.instance.id, () => {
  state.url = props.instance.integration.webhook.url ?? ''
  state.enabled = props.instance.integration.webhook.enabled
  state.events = [...props.instance.integration.webhook.events]
  failure.value = null
})
</script>

<template>
  <UPageCard :title="t('instances.webhook.cardTitle')" variant="subtle" data-testid="webhook-card">
    <UForm
      id="webhook"
      :schema="schema"
      :state="state"
      class="flex flex-col gap-4"
      @submit="onSave"
    >
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />

      <UFormField :label="t('instances.webhook.url')" :hint="t('instances.webhook.urlHint')" name="url">
        <UInput
          v-model="state.url"
          type="url"
          maxlength="2048"
          placeholder="https://hooks.example.com/wzap"
          class="w-full font-mono"
        />
      </UFormField>

      <UFormField :label="t('instances.webhook.enabled')" name="enabled">
        <USwitch v-model="state.enabled" />
      </UFormField>

      <UFormField name="events">
        <div class="flex flex-col gap-2">
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-medium text-highlighted">{{ t('instances.webhook.events') }}</span>
            <div class="flex gap-1">
              <UButton
                type="button"
                color="neutral"
                variant="ghost"
                size="xs"
                :label="t('instances.webhook.selectAll')"
                @click="selectAll"
              />
              <UButton
                type="button"
                color="neutral"
                variant="ghost"
                size="xs"
                :label="t('instances.webhook.selectNone')"
                @click="selectNone"
              />
            </div>
          </div>
          <p class="text-sm text-muted">
            {{ t('instances.webhook.eventsHint') }}
          </p>
          <div class="flex flex-col gap-2">
            <UCheckbox
              v-for="type in WEBHOOK_EVENT_TYPES"
              :key="type"
              :model-value="isSubscribed(type)"
              :label="type"
              @update:model-value="(value: boolean | 'indeterminate') => setSubscribed(type, value === true)"
            />
          </div>
          <UAlert
            v-if="subscribedCount === 0"
            color="warning"
            variant="subtle"
            :title="t('instances.webhook.noEventsTitle')"
            :description="t('instances.webhook.noEventsBody')"
          />
        </div>
      </UFormField>

      <div class="flex justify-end">
        <UButton
          type="submit"
          data-testid="webhook-save"
          :loading="saving"
          :label="saving ? t('common.saving') : t('common.save')"
        />
      </div>
    </UForm>
  </UPageCard>
</template>
