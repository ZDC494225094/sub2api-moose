<template>
  <div class="provider-network globe-network" role="group" aria-label="全球 AI 连接网络">
    <div class="network-grid" aria-hidden="true"></div>
    <div class="network-caption"><span class="signal-dot"></span> ONE API. MORE POSSIBILITIES.</div>
    <div ref="globeStage" class="gateway-globe-stage">
      <div class="gateway-globe-aura" aria-hidden="true"></div>
      <canvas ref="globeCanvas" class="gateway-globe-canvas" role="img" aria-label="可拖动旋转的点阵地球，光点沿全球航线流动"></canvas>
      <div ref="logoSources" hidden aria-hidden="true">
        <div v-for="provider in globeProviderMarkers" :key="provider.id" :data-logo-source="provider.id">
          <ModelIcon :model="provider.model" size="26px" />
        </div>
      </div>
      <span v-if="globeFailed" class="globe-fallback-label">GLOBAL AI NETWORK</span>
    </div>
    <div class="network-request" aria-hidden="true"><span class="request-method">POST</span><code>/v1/chat/completions</code><span class="request-spark">↗</span></div>
    <div class="network-footnote">UNIFIED ACCESS <span>·</span> BUILT FOR CREATORS</div>
    <button class="globe-motion-toggle" type="button" :aria-label="animating ? '暂停页面动画' : '播放页面动画'" :title="animating ? '暂停页面动画' : '播放页面动画'" @click="animating = !animating">
      <svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true"><path v-if="animating" d="M4 3h3v10H4zm5 0h3v10H9z" /><path v-else d="M4 2.5 13 8l-9 5.5z" /></svg>
    </button>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import { mountPremiumHomeGlobe, globeProviderMarkers, type PremiumHomeGlobeController } from './premium-home-globe'

const props = defineProps<{ isDark: boolean; siteLogo?: string }>()
const emit = defineEmits<{ 'motion-change': [enabled: boolean] }>()
const globeCanvas = ref<HTMLCanvasElement | null>(null)
const globeStage = ref<HTMLDivElement | null>(null)
const logoSources = ref<HTMLDivElement | null>(null)
const globeFailed = ref(false)
const animating = ref(true)
let controller: PremiumHomeGlobeController | undefined
let unmounted = false
let entrance: Animation | undefined

watch(() => props.isDark, (dark) => controller?.setTheme(dark))
watch(() => props.siteLogo, (logo) => controller?.setSiteLogo(logo || '/logo.svg'))
watch(animating, (enabled) => {
  controller?.setAnimating(enabled)
  if (!enabled) entrance?.finish()
  emit('motion-change', enabled)
})
onMounted(async () => {
  const providerLogos: Record<string, string> = {}
  logoSources.value?.querySelectorAll<HTMLElement>('[data-logo-source]').forEach((element) => {
    const svg = element.querySelector('svg')?.cloneNode(true) as SVGElement | undefined
    if (!svg) return
    svg.setAttribute('width', '256')
    svg.setAttribute('height', '256')
    providerLogos[element.dataset.logoSource!] = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(new XMLSerializer().serializeToString(svg))}`
  })
  try {
    const mounted = await mountPremiumHomeGlobe(globeCanvas.value, { maxSize: 760, maxPixelRatio: 1.75, isDark: props.isDark, animate: animating.value, providerLogos })
    if (unmounted) { mounted(); return }
    controller = mounted
    controller.setTheme(props.isDark)
    controller.setAnimating(animating.value)
    controller.setSiteLogo(props.siteLogo || '/logo.svg')
    const stage = globeStage.value
    if (stage && animating.value && window.scrollY < 100) {
      const rect = stage.getBoundingClientRect()
      const compact = window.innerWidth <= 760
      const offset = compact ? 0 : (window.innerWidth / 2 - rect.left - rect.width / 2) * 0.55
      entrance = stage.animate([
        { transform: `translate(calc(-50% + ${offset}px), -43%) scale(${compact ? 1.18 : 1.65})`, opacity: 0, offset: 0 },
        { transform: `translate(calc(-50% + ${offset}px), -43%) scale(${compact ? 1.18 : 1.65})`, opacity: 1, offset: 0.18 },
        { transform: 'translate(-50%, -50%) scale(1)', opacity: 1, offset: 1 },
      ], { duration: 2200, easing: 'cubic-bezier(.22, 1, .36, 1)' })
    }
  } catch (error) {
    globeFailed.value = true
    console.warn('Premium home globe unavailable:', error)
  }
})
onBeforeUnmount(() => { unmounted = true; entrance?.cancel(); controller?.() })
</script>
