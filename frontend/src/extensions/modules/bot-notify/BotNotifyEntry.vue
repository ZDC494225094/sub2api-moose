<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore()
</script>
<template>
  <RouterLink v-if="auth.isAdmin" to="/admin/bot-notify" class="fixed bottom-6 right-6 z-30 rounded-xl bg-primary-600 px-5 py-3 text-sm font-medium text-white shadow-lg">管理消息机器人 →</RouterLink>
</template>
