export default defineAppConfig({
  ui: {
    colors: {
      primary: 'green',
      neutral: 'zinc'
    },
    button: {
      compoundVariants: [
        {
          color: 'primary',
          variant: 'solid',
          class: 'hover:bg-primary-900 active:bg-primary-900 dark:hover:bg-primary/75 dark:active:bg-primary/75'
        },
        {
          color: 'error',
          variant: 'solid',
          class: 'dark:hover:bg-error/90 dark:active:bg-error/90'
        },
        {
          color: 'error',
          variant: 'soft',
          class: 'dark:hover:bg-error/10 dark:active:bg-error/10'
        }
      ]
    }
  }
})
