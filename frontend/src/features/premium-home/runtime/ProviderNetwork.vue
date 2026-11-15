<template>
  <div class="provider-network globe-network" role="group" aria-label="全球 AI 连接网络">
    <div class="network-grid" aria-hidden="true"></div>
    <div class="gateway-globe-stage">
      <div class="network-caption"><span class="signal-dot"></span> ONE API. MORE POSSIBILITIES.</div>
      <div class="gateway-globe-aura" aria-hidden="true"></div>
      <canvas ref="globeCanvas" class="gateway-globe-canvas" :style="{ visibility: globeReady ? 'visible' : 'hidden', background: 'transparent' }" role="img" aria-label="可拖动旋转的点阵地球，光点沿全球航线流动"></canvas>
      <div ref="logoSources" hidden aria-hidden="true">
        <div v-for="provider in globeProviderMarkers" :key="provider.id" :data-logo-source="provider.id">
          <ModelIcon :model="provider.model" size="26px" />
        </div>
      </div>
      <span v-if="!globeReady" class="globe-fallback-label">GLOBAL AI NETWORK</span>
    </div>
    <div class="network-request" aria-hidden="true"><span class="request-method">POST</span><code>/v1/chat/completions</code><Icon name="arrowRight" size="sm" class="request-spark" /></div>
    <div class="network-footnote">UNIFIED ACCESS <span>·</span> BUILT FOR CREATORS</div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { useConsoleThemeApplied } from '@/extensions/modules/console-theme/useConsoleThemeApplied'
import type { PremiumHomeGlobeController, PremiumHomeGlobePalette } from './premium-home-globe'
import { globeProviderMarkers } from './globe-providers'

const props = defineProps<{ isDark: boolean; siteLogo?: string }>()
const globeCanvas = ref<HTMLCanvasElement | null>(null)
const logoSources = ref<HTMLDivElement | null>(null)
const globeReady = ref(false)
// Neutral globe colours while the console theme is on.
const glassTheme = useConsoleThemeApplied()
const palette = (): PremiumHomeGlobePalette => (glassTheme.value ? 'glass' : 'default')
let controller: PremiumHomeGlobeController | undefined
let unmounted = false
let idleHandle: number | undefined
let startTimer: ReturnType<typeof setTimeout> | undefined

watch(() => props.isDark, (dark) => controller?.setTheme(dark))
watch(glassTheme, () => controller?.setPalette(palette()))
watch(() => props.siteLogo, (logo) => controller?.setSiteLogo(logo || '/logo.svg'))
async function loadGlobe() {
  const providerLogos: Record<string, string> = {}
  logoSources.value?.querySelectorAll<HTMLElement>('[data-logo-source]').forEach((element) => {
    const svg = element.querySelector('svg')?.cloneNode(true) as SVGElement | undefined
    if (!svg) return
    svg.setAttribute('width', '256')
    svg.setAttribute('height', '256')
    providerLogos[element.dataset.logoSource!] = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(new XMLSerializer().serializeToString(svg))}`
  })
  try {
    const { mountPremiumHomeGlobe } = await import('./premium-home-globe')
    if (unmounted) return
    const mounted = await mountPremiumHomeGlobe(globeCanvas.value, { maxSize: 760, maxPixelRatio: 1.75, isDark: props.isDark, palette: palette(), animate: true, providerLogos })
    if (unmounted) { mounted(); return }
    controller = mounted
    controller.setPalette(palette())
    controller.setTheme(props.isDark)
    controller.setAnimating(true)
    controller.setSiteLogo(props.siteLogo || '/logo.svg')
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    if (unmounted) return
    globeReady.value = true
  } catch (error) {
    console.warn('Premium home globe unavailable:', error)
  }
}
onMounted(() => {
  // Paint navigation and text before fetching the decorative WebGL dependency.
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  if (connection?.saveData || /(^|-)2g$/.test(connection?.effectiveType || '')) return
  startTimer = setTimeout(() => {
    if ('requestIdleCallback' in window) {
      idleHandle = window.requestIdleCallback(() => { void loadGlobe() }, { timeout: 2000 })
    } else {
      void loadGlobe()
    }
  }, 200)
})
onBeforeUnmount(() => {
  unmounted = true
  clearTimeout(startTimer)
  if (idleHandle !== undefined) window.cancelIdleCallback(idleHandle)
  controller?.()
})
</script>
