<template>
  <div class="relative mx-auto aspect-square w-full max-w-[420px]" aria-label="全球 AI 网络与站点 Logo">
    <canvas ref="canvas" class="h-full w-full" />
    <img v-if="failed" :src="logo" alt="站点 Logo" class="absolute left-1/2 top-1/2 h-24 w-24 -translate-x-1/2 -translate-y-1/2 object-contain" />
  </div>
</template>
<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import type { PremiumHomeGlobeController } from '@/features/premium-home/runtime/premium-home-globe'
const props = defineProps<{ logo: string }>()
const canvas = ref<HTMLCanvasElement>()
const failed = ref(false)
let controller: PremiumHomeGlobeController | undefined
let disposed = false
onMounted(async () => {
  try {
    const { mountPremiumHomeGlobe } = await import('@/features/premium-home/runtime/premium-home-globe')
    if (disposed) return
    controller = await mountPremiumHomeGlobe(canvas.value!, { maxSize: 420, maxPixelRatio: 1.5, isDark: false, animate: false })
    if (disposed) { controller(); return }
    await controller.setSiteLogo(props.logo)
  } catch { failed.value = true }
})
watch(() => props.logo, value => { void controller?.setSiteLogo(value) })
onBeforeUnmount(() => { disposed = true; controller?.() })
</script>
