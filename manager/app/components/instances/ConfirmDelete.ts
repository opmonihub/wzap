import { UButton, UModal } from '#components'

// Programmatic destructive confirmation behind useOverlay(): the modal only
// resolves a boolean (true = confirmed) and the caller runs the API after
// `await ... .result` turns true, so loading states and failure display stay
// on the trigger (card/bulk-bar buttons plus toasts). Dismiss (cancel button,
// escape, backdrop) resolves falsy and the caller does nothing. The typed-name
// instance delete keeps its own DeleteInstanceModal and never goes through
// here.
export interface ConfirmDeleteProps {
  title: string
  description: string
  confirmLabel: string
  confirmColor?: 'error' | 'warning'
}

const ConfirmDeleteModal = defineComponent({
  name: 'ConfirmDeleteModal',
  props: {
    title: {
      type: String,
      required: true
    },
    description: {
      type: String,
      required: true
    },
    confirmLabel: {
      type: String,
      required: true
    },
    confirmColor: {
      type: String as PropType<'error' | 'warning'>,
      default: 'error'
    }
  },
  emits: {
    close: (value: boolean) => typeof value === 'boolean'
  },
  setup(props, { emit }) {
    const { t } = useI18n()

    return () => h(UModal, {
      title: props.title,
      description: props.description,
      ui: { footer: 'justify-end' }
    }, {
      footer: ({ close }: { close: () => void }) => [
        h(UButton, {
          color: 'neutral',
          variant: 'outline',
          label: t('common.cancel'),
          onClick: () => close()
        }),
        h(UButton, {
          color: props.confirmColor,
          label: props.confirmLabel,
          onClick: () => emit('close', true)
        })
      ]
    })
  }
})

export function useConfirmDelete() {
  const overlay = useOverlay()

  async function confirmDelete(props: ConfirmDeleteProps): Promise<boolean> {
    const confirmed = await overlay.create(ConfirmDeleteModal).open(props).result
    return confirmed === true
  }

  return { confirmDelete }
}
