<template>
  <div class="provider-network globe-network" :class="{ 'is-entering': introActive }" role="group" aria-label="全球 AI 连接网络">
    <Teleport to="body">
      <div v-if="introActive" ref="introBackdrop" class="globe-intro-backdrop" :class="{ 'is-dark': isDark }" aria-hidden="true"></div>
    </Teleport>
    <div class="network-grid" aria-hidden="true"></div>
    <div ref="globeStage" class="gateway-globe-stage">
      <div class="network-caption"><span class="signal-dot"></span> ONE API. MORE POSSIBILITIES.</div>
      <div class="gateway-globe-aura" aria-hidden="true"></div>
      <canvas ref="globeCanvas" class="gateway-globe-canvas" :style="{ visibility: globeReady ? 'visible' : 'hidden', background: 'transparent' }" role="img" aria-label="可拖动旋转的点阵地球，光点沿全球航线流动"></canvas>
      <div ref="logoSources" hidden aria-hidden="true">
        <div v-for="provider in globeProviderMarkers" :key="provider.id" :data-logo-source="provider.id">
          <ModelIcon :model="provider.model" size="26px" />
        </div>
      </div>
      <span v-if="globeFailed" class="globe-fallback-label">GLOBAL AI NETWORK</span>
    </div>
    <div class="network-request" aria-hidden="true"><span class="request-method">POST</span><code>/v1/chat/completions</code><span class="request-spark">↗</span></div>
    <div class="network-footnote">UNIFIED ACCESS <span>·</span> BUILT FOR CREATORS</div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import { mountPremiumHomeGlobe, globeProviderMarkers, type PremiumHomeGlobeController } from './premium-home-globe'

const props = defineProps<{ isDark: boolean; siteLogo?: string }>()
const globeCanvas = ref<HTMLCanvasElement | null>(null)
const globeStage = ref<HTMLDivElement | null>(null)
const logoSources = ref<HTMLDivElement | null>(null)
const globeFailed = ref(false)
const globeReady = ref(false)
const introActive = ref(false)
const introBackdrop = ref<HTMLDivElement | null>(null)
let controller: PremiumHomeGlobeController | undefined
let unmounted = false
let entrance: Animation | undefined
let backdropAnimation: Animation | undefined
function finishEntrance() {
  entrance?.finish()
  backdropAnimation?.finish()
  introActive.value = false
  window.removeEventListener('scroll', finishEntrance)
  window.removeEventListener('resize', finishEntrance)
}

watch(() => props.isDark, (dark) => controller?.setTheme(dark))
watch(() => props.siteLogo, (logo) => controller?.setSiteLogo(logo || '/logo.svg'))
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
    const mounted = await mountPremiumHomeGlobe(globeCanvas.value, { maxSize: 760, maxPixelRatio: 1.75, isDark: props.isDark, animate: true, providerLogos })
    if (unmounted) { mounted(); return }
    controller = mounted
    controller.setTheme(props.isDark)
    controller.setAnimating(true)
    controller.setSiteLogo(props.siteLogo || '/logo.svg')
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    if (unmounted) return
    globeReady.value = true
    const stage = globeStage.value
    if (stage && window.scrollY < 100) {
      const rect = stage.getBoundingClientRect()
      const headerHeight = stage.closest('.premium-home')?.querySelector('.topbar')?.getBoundingClientRect().height || 0
      const availableHeight = Math.max(1, window.innerHeight - headerHeight)
      const offsetX = window.innerWidth / 2 - rect.left - rect.width / 2
      const offsetY = headerHeight + availableHeight / 2 - rect.top - rect.height / 2
      // Fit the entire canvas, including flight paths, below the navigation with breathing room.
      const scale = Math.min(window.innerWidth, availableHeight) * 0.9 / Math.max(rect.width, 1)
      const fullScreen = `translate(calc(-50% + ${offsetX}px), calc(-50% + ${offsetY}px)) scale(${scale})`
      introActive.value = true
      await nextTick()
      if (unmounted) return
      entrance = stage.animate([
        { transform: fullScreen, opacity: 0, offset: 0 },
        { transform: fullScreen, opacity: 1, offset: 0.12 },
        { transform: fullScreen, opacity: 1, offset: 0.26, easing: 'cubic-bezier(.65,0,.2,1)' },
        { transform: 'translate(-50%, -50%) scale(1)', opacity: 1, offset: 1 },
      ], { duration: 3400, easing: 'linear' })
      backdropAnimation = introBackdrop.value?.animate([
        { opacity: 1, offset: 0 }, { opacity: 1, offset: 0.35 }, { opacity: 0, offset: 0.85 }, { opacity: 0, offset: 1 },
      ], { duration: 3400, fill: 'forwards' })
      window.addEventListener('scroll', finishEntrance, { passive: true })
      window.addEventListener('resize', finishEntrance, { passive: true })
      void entrance.finished.then(() => { if (!unmounted) finishEntrance() }).catch(() => {})
    }
  } catch (error) {
    finishEntrance()
    globeFailed.value = true
    console.warn('Premium home globe unavailable:', error)
  }
})
onBeforeUnmount(() => {
  unmounted = true
  window.removeEventListener('scroll', finishEntrance)
  window.removeEventListener('resize', finishEntrance)
  entrance?.cancel()
  backdropAnimation?.cancel()
  controller?.()
})
</script>

<style scoped>
.provider-network.is-entering { z-index: 60; pointer-events: none; }
.globe-intro-backdrop { position: fixed; inset: 0; z-index: 50; background: #f8fafc; pointer-events: none; }
.globe-intro-backdrop.is-dark { background: #0c1220; }
.is-entering .gateway-globe-stage { z-index: 1; }
.is-entering :is(.network-caption, .network-request, .network-footnote) { visibility: hidden; }
</style>
