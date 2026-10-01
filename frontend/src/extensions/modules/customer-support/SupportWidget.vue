<script setup lang="ts">
import { ref } from 'vue'
import { useExtensionStore } from '@/extensions/store'
import CustomerServiceFloat from './CustomerServiceFloat.vue'

withDefaults(defineProps<{ alwaysVisible?: boolean; directLink?: boolean }>(), { alwaysVisible: false, directLink: false })
const extensions = useExtensionStore()
const widget = ref<InstanceType<typeof CustomerServiceFloat> | null>(null)
defineExpose({ openPanel: () => {
  if (extensions.enabled('customer-support')) widget.value?.openPanel()
} })
</script>

<template>
  <CustomerServiceFloat v-if="extensions.enabled('customer-support')" ref="widget" :always-visible="alwaysVisible" :direct-link="directLink" />
</template>
