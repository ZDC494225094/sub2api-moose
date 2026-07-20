import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'
import { updateFavicon } from '@/utils/branding'
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

function initThemeClass() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark =
    savedTheme === 'dark' ||
    ((savedTheme === 'system' || !savedTheme) && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', shouldUseDark)
}

async function bootstrap() {
  // Apply theme class globally before app mount to keep all routes consistent.
  initThemeClass()
  initInjectedConfigFromMeta()

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

  await initI18n()

  app.use(router)
  app.use(i18n)

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()
  app.mount('#app')
}

bootstrap()
