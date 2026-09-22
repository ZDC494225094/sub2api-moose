<template>
  <BaseDialog :show="!!campaign" title="分享充值活动" @close="$emit('close')">
    <div class="space-y-4">
      <p class="text-sm text-gray-500">{{ affiliateCode ? '二维码已包含你的邀请码，新用户注册时自动绑定邀请关系。' : '通用宣传卡片。用户在充值页分享时可自动附带自己的邀请码。' }}</p>
      <div ref="logoSources" hidden aria-hidden="true"><div v-for="provider in providers" :key="provider.model" :data-model="provider.model"><ModelIcon :model="provider.model" size="64px" /></div></div>
      <canvas ref="canvas" class="mx-auto w-full max-w-80 rounded-lg shadow-lg" aria-label="含站点标识、厂商标识和点阵地球的充值活动宣传卡片" />
      <p v-if="!ready && !error" class="text-center text-sm text-gray-500">正在生成宣传卡片…</p>
      <p v-if="warning" class="text-sm text-amber-600" role="status">{{ warning }}</p>
      <p v-if="error" class="text-sm text-red-500" role="alert">{{ error }}</p>
      <div class="flex gap-3"><button class="btn btn-primary" :disabled="!ready" @click="download">下载 PNG 卡片</button><button class="btn btn-secondary" @click="copy">复制活动链接</button></div>
    </div>
  </BaseDialog>
</template>
<script setup lang="ts">
import { ref, watch, nextTick, computed, onBeforeUnmount } from 'vue'
import QRCode from 'qrcode'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { RechargeCampaign } from '@/api/rechargeCampaigns'
import { useAppStore } from '@/stores/app'
import ModelIcon from '@/components/common/ModelIcon.vue'
import { sanitizeUrl } from '@/utils/url'
import { drawCampaignPoster, loadPosterImage } from './campaignPoster'
const props = defineProps<{ campaign: RechargeCampaign | null; affiliateCode?: string; sharer?: { name: string; avatarUrl?: string | null } }>()
defineEmits<{ close: [] }>()
const app = useAppStore()
const canvas = ref<HTMLCanvasElement>()
const ready = ref(false)
const error = ref('')
const warning = ref('')
const logoSources = ref<HTMLElement>()
const providers = [{ name: 'OpenAI', model: 'gpt' }, { name: 'Claude', model: 'claude' }, { name: 'Gemini', model: 'gemini' }, { name: 'DeepSeek', model: 'deepseek' }, { name: 'Qwen', model: 'qwen' }, { name: 'Grok', model: 'grok' }]
const siteLogo = computed(() => sanitizeUrl(app.cachedPublicSettings?.site_logo || app.siteLogo || '/logo.svg', { allowRelative: true, allowDataUrl: true }) || '/logo.svg')
const link = computed(() => {
  const url = new URL(`/recharge-campaigns/${props.campaign?.id}`, window.location.origin)
  if (props.affiliateCode) url.searchParams.set('aff', props.affiliateCode)
  return url.toString()
})
let generation = 0
onBeforeUnmount(() => { generation++ })
watch(() => [props.campaign, props.affiliateCode, props.sharer?.name, props.sharer?.avatarUrl, app.siteName, siteLogo.value], async () => {
  const current = ++generation
  ready.value = false
  error.value = ''
  warning.value = ''
  await nextTick()
  const a = props.campaign, c = canvas.value
  if (!a || !c) return
  let cleanup: (() => void) | undefined
  const stage = document.createElement('div')
  stage.style.cssText = 'position:fixed;left:-10000px;top:0;width:640px;height:640px;pointer-events:none'
  stage.setAttribute('aria-hidden', 'true')
  try {
    const qr = document.createElement('canvas')
    await QRCode.toCanvas(qr, link.value, { width: 216, margin: 4, errorCorrectionLevel: 'M' })
    if (current !== generation) return
    const logo = await loadPosterImage(siteLogo.value).catch(async () => {
      if (current === generation) warning.value = '站点 Logo 暂时无法导出，已使用默认 Logo。自定义图片需允许跨域访问。'
      return loadPosterImage('/logo.svg')
    })
    const providerImages = await Promise.all(providers.map(async provider => {
      const svg = logoSources.value?.querySelector(`[data-model="${provider.model}"] svg`)?.cloneNode(true) as SVGElement | undefined
      if (!svg) throw new Error('Provider logo unavailable')
      svg.setAttribute('width', '128'); svg.setAttribute('height', '128')
      return { name: provider.name, image: await loadPosterImage(`data:image/svg+xml;charset=utf-8,${encodeURIComponent(new XMLSerializer().serializeToString(svg))}`) }
    }))
    if (current !== generation) return
    const globe = document.createElement('canvas')
    stage.appendChild(globe); document.body.appendChild(stage)
    const { mountPremiumHomeGlobe } = await import('@/features/premium-home/runtime/premium-home-globe')
    const controller = await mountPremiumHomeGlobe(globe, { maxSize: 640, maxPixelRatio: 1, isDark: false, animate: false })
    cleanup = controller
    await controller.setSiteLogo(logo.src)
    if (current !== generation) return
    await document.fonts.ready
    if (current !== generation) return
    const avatarUrl = sanitizeUrl(props.sharer?.avatarUrl || '', { allowRelative: true, allowDataUrl: true })
    const avatar = avatarUrl ? await loadPosterImage(avatarUrl).catch(() => undefined) : undefined
    if (current !== generation) return
    drawCampaignPoster(c, { campaign: a, siteName: app.siteName, logo, globe, providers: providerImages, qr, host: window.location.host, sharer: props.sharer ? { name: props.sharer.name, avatar } : undefined })
    c.toDataURL('image/png')
    ready.value = true
  } catch { if (current === generation) error.value = '卡片生成失败，请确认浏览器支持 WebGL 及图片加载后重试。' }
  finally { cleanup?.(); stage.remove() }
}, { immediate: true })
function download() {
  if (!canvas.value || !ready.value) return
  const anchor = document.createElement('a'); anchor.download = `充值活动-${props.campaign?.id}.png`; anchor.href = canvas.value.toDataURL('image/png'); anchor.click()
}
async function copy() {
  try { await navigator.clipboard.writeText(link.value); app.showSuccess('活动链接已复制') } catch { error.value = '无法自动复制，请使用下载卡片分享。' }
}
</script>
