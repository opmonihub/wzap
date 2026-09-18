<script setup lang="ts">
import LoginForm from '~/components/auth/LoginForm.vue'

definePageMeta({ layout: 'auth' })
useSeoMeta({ title: 'Sign in' })
</script>

<template>
  <LoginForm />
</template>
