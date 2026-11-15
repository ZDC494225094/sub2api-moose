import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'
import { updateFavicon } from '@/utils/branding'
import { isIOSDevice } from '@/utils/device'
import './style.css'

function initInjectedConfigFromMeta() {
  if (window.__APP_CONFIG__) return

  const meta = document.querySelector<HTMLMetaElement>('meta[name="app-config"]')
  const encoded = meta?.content
  if (!encoded) return

  try {
    const bytes = Uint8Array.from(atob(encoded), (char) => char.charCodeAt(0))
    const json = new TextDecoder().decode(bytes)
    window.__APP_CONFIG__ = JSON.parse(json) as PublicSettings
  } catch (error) {
    console.warn('Failed to parse injected app config:', error)
  }
}

function initIOSViewportZoomFix() {
  // iOS Safari 在输入框字号小于 16px 时聚焦会自动放大页面，且失焦后不会恢复。
  // 限制 maximum-scale 可阻止该行为；iOS 10+ 用户仍可双指手动缩放，不影响可访问性。
  // 仅在 iOS 设备上注入，避免影响 Android Chrome 的手动缩放能力。
  if (!isIOSDevice()) return

  const viewport = document.querySelector('meta[name="viewport"]')
  if (!viewport) return

  const content = viewport.getAttribute('content') || ''
  if (/maximum-scale/i.test(content)) return
  viewport.setAttribute('content', `${content}, maximum-scale=1.0`)
}

function initThemeClass() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark =
    savedTheme === 'dark' ||
    ((savedTheme === 'system' || !savedTheme) && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', shouldUseDark)
}

async function waitForApplicationStyles() {
  await Promise.all(Array.from(document.querySelectorAll<HTMLLinkElement>('link[data-app-styles]')).map(link => {
    if (link.dataset.appStyles === 'ready') return Promise.resolve()
    if (link.dataset.appStyles === 'error') return Promise.reject(new Error('Application stylesheet unavailable'))
    return new Promise<void>((resolve, reject) => {
      link.addEventListener('load', () => resolve(), { once: true })
      link.addEventListener('error', () => reject(new Error('Application stylesheet unavailable')), { once: true })
    })
  }))
}

async function bootstrap() {
  // Apply theme class globally before app mount to keep all routes consistent.
  initThemeClass()
  initInjectedConfigFromMeta()
  initIOSViewportZoomFix()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Initialize settings from injected config BEFORE mounting (prevents flash)
  // This must happen after pinia is installed but before router and i18n
  const appStore = useAppStore()
  appStore.initFromInjectedConfig()

  // Set document title immediately after config is loaded
  if (appStore.siteName && appStore.siteName !== 'Sub2API') {
    document.title = `${appStore.siteName} - AI API Gateway`
  }
  updateFavicon(appStore.siteLogo)

  app.use(router)
  app.use(i18n)

  // The route guard loads the same locale promise. Run routing/admission in
  // parallel with initialization instead of starting it after the locale fetch.
  await initI18n()

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()
  await waitForApplicationStyles()
  app.mount('#app')
}

bootstrap().catch((error: unknown) => {
  console.error('Application startup failed:', error)
  const title = document.getElementById('app-boot-title')
  const message = document.getElementById('app-boot-message')
  const retry = document.getElementById('app-boot-retry')
  if (title) title.textContent = '页面暂未加载完成'
  if (message) message.textContent = '请检查网络连接后重新加载。'
  if (retry) retry.hidden = false
})
