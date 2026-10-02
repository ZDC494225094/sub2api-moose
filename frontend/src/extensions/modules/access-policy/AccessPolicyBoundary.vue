<script setup lang="ts">
defineProps<{ resolved: boolean; loading: boolean; setup: boolean }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <slot v-if="resolved || setup" />
  <main v-else class="flex min-h-screen items-center justify-center bg-gray-50 p-6 dark:bg-dark-900">
    <section role="status" class="text-center text-gray-700 dark:text-gray-200">
      <p>{{ loading ? '正在确认页面访问策略… / Checking access policy…' : '暂时无法确认页面访问策略，请重试。 / Unable to verify access policy. Please retry.' }}</p>
      <button v-if="!loading" type="button" class="mt-4 rounded bg-primary-600 px-4 py-2 text-white" @click="$emit('retry')">重试 / Retry</button>
    </section>
  </main>
</template>
