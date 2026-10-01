<script setup lang="ts">
import { onErrorCaptured, ref, type Component } from 'vue'

defineProps<{ component: Component; bindings?: Readonly<Record<string, unknown>> }>()
const failed = ref(false)
// An optional widget must never take the upstream page down. Toggling it off/on
// remounts this boundary, permitting recovery after a transient chunk error.
onErrorCaptured((error) => {
  failed.value = true
  console.warn('[custom-extensions] Optional widget failed; upstream page remains available.', error)
  return false
})
</script>

<template>
  <component :is="component" v-if="!failed" v-bind="bindings" />
</template>
