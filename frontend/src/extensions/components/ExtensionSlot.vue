<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { extensionSlots, type ExtensionSlotName } from '../slots'
import { useExtensionStore } from '../store'
import ExtensionMount from './ExtensionMount.vue'

const props = defineProps<{ name: ExtensionSlotName; bindings?: Readonly<Record<string, unknown>> }>()
const route = useRoute()
const extensions = useExtensionStore()
const entries = computed(() => extensionSlots.filter(entry =>
  entry.slot === props.name && (entry.readiness === 'pending' || extensions.enabled(entry.extension)) && (!entry.visible || entry.visible(route))))
</script>

<template>
  <ExtensionMount v-for="entry in entries" :key="entry.key" :component="entry.component" :bindings="bindings" />
</template>
