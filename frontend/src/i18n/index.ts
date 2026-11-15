import { createI18n } from 'vue-i18n'

type LocaleCode = 'en' | 'zh'
type LocaleScope = 'public' | 'full'

type LocaleMessages = Record<string, any>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'en'

const localeLoaders: Record<LocaleCode, () => Promise<{ default: LocaleMessages }>> = {
  en: () => import('./locales/en'),
  zh: () => import('./locales/zh')
}
const publicLocaleLoaders: typeof localeLoaders = {
  en: () => import('./locales/en/public'),
  zh: () => import('./locales/zh/public')
}
const publicLocalePaths = new Set(['/', '/home', '/login', '/register'])
const scopeForPath = (path: string): LocaleScope => publicLocalePaths.has(path) ? 'public' : 'full'

function isLocaleCode(value: string): value is LocaleCode {
  return value === 'en' || value === 'zh'
}

function getDefaultLocale(): LocaleCode {
  const saved = localStorage.getItem(LOCALE_KEY)
  if (saved && isLocaleCode(saved)) {
    return saved
  }

  const browserLang = navigator.language.toLowerCase()
  if (browserLang.startsWith('zh')) {
    return 'zh'
  }

  return DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {},
  // 禁用 HTML 消息警告 - 引导步骤使用富文本内容（driver.js 支持 HTML）
  // 这些内容是内部定义的，不存在 XSS 风险
  warnHtmlMessage: false
})

const loadedLocales = new Map<LocaleCode, LocaleScope>()
const pendingLocales = new Map<string, Promise<void>>()

export async function loadLocaleMessages(locale: LocaleCode, scope: LocaleScope = 'full'): Promise<void> {
  const loaded = loadedLocales.get(locale)
  if (loaded === 'full' || loaded === scope) {
    return
  }

  const key = `${locale}:${scope}`
  const pending = pendingLocales.get(key)
  if (pending) return pending
  const loader = scope === 'public' ? publicLocaleLoaders[locale] : localeLoaders[locale]
  const request = loader().then((module) => {
    // A slower public request must not replace a full dictionary loaded meanwhile.
    if (loadedLocales.get(locale) === 'full') return
    i18n.global.setLocaleMessage(locale, module.default)
    loadedLocales.set(locale, scope)
  }).finally(() => pendingLocales.delete(key))
  pendingLocales.set(key, request)
  return request
}

export async function loadRouteLocale(path: string): Promise<void> {
  await loadLocaleMessages(getLocale(), scopeForPath(path))
}

export async function initI18n(): Promise<void> {
  const current = getLocale()
  await loadRouteLocale(window.location.pathname)
  document.documentElement.setAttribute('lang', current)
}

export async function setLocale(locale: string): Promise<void> {
  if (!isLocaleCode(locale)) {
    return
  }

  await loadLocaleMessages(locale, scopeForPath(window.location.pathname))
  i18n.global.locale.value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.setAttribute('lang', locale)

  // 同步更新浏览器页签标题，使其跟随语言切换
  const { resolveRouteDocumentTitle } = await import('@/router/title')
  const { default: router } = await import('@/router')
  const { useAppStore } = await import('@/stores/app')
  const { useAuthStore } = await import('@/stores/auth')
  const { useAdminSettingsStore } = await import('@/stores/adminSettings')
  const route = router.currentRoute.value
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems)
}

export function getLocale(): LocaleCode {
  const current = i18n.global.locale.value
  return isLocaleCode(current) ? current : DEFAULT_LOCALE
}

export const availableLocales = [
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'zh', name: '中文', flag: '🇨🇳' }
] as const

export default i18n
