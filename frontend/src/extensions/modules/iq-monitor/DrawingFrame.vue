<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// A drawing is laid out in a desktop-sized window and scaled down to fit,
// so models that size their SVG for a full page look the same everywhere.
// The document comes from a signed URL that sets its own sandboxing CSP;
// the frame adds sandbox="allow-scripts" (no same-origin, forms or popups).
const props = withDefaults(defineProps<{
  src: string
  title: string
  baseWidth?: number
  baseHeight?: number
  interactive?: boolean
}>(), { baseWidth: 1280, baseHeight: 800, interactive: false })

const box = ref<HTMLElement | null>(null)
const scale = ref(0.25)
let observer: ResizeObserver | null = null

function measure() {
  const el = box.value
  if (!el) return
  scale.value = Math.min(el.clientWidth / props.baseWidth, el.clientHeight / props.baseHeight) || 0.25
}

onMounted(() => {
  measure()
  if (typeof ResizeObserver !== 'undefined' && box.value) {
    observer = new ResizeObserver(measure)
    observer.observe(box.value)
  }
})
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <div
    ref="box"
    class="relative overflow-hidden bg-white"
    :style="{ aspectRatio: `${baseWidth} / ${baseHeight}` }"
  >
    <iframe
      :src="src"
      :title="title"
      sandbox="allow-scripts"
      referrerpolicy="no-referrer"
      loading="lazy"
      class="absolute left-0 top-0 origin-top-left border-0"
      :class="interactive ? '' : 'pointer-events-none'"
      :style="{ width: `${baseWidth}px`, height: `${baseHeight}px`, transform: `scale(${scale})` }"
    ></iframe>
  </div>
</template>
