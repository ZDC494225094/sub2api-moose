import { ref, watch } from 'vue'
import { installPointerField } from './pointerField'
import './theme.css'

// Presentation-only module: it never admits or blocks business. The whole theme is
// one root attribute that switches CSS variables and scoped component styles, so
// removing the attribute restores the upstream look without a reload.
export const CONSOLE_THEME_ID = 'console-theme' as const
export const CONSOLE_THEME_ATTRIBUTE = 'data-console-theme'
export const CONSOLE_THEME_NAME = 'glass'
// Last confirmed switch state, used only to paint the first frame before the
// public extension state arrives. It never decides anything server-side.
export const CONSOLE_THEME_CACHE_KEY = 'custom-extensions.console-theme'

interface ConsoleThemeState {
  loaded: boolean
  error: string
  enabled: (id: typeof CONSOLE_THEME_ID) => boolean
}

const consoleThemeApplied = ref(false)
let stopPointerField: (() => void) | undefined

function root(): HTMLElement | null {
  return typeof document === 'undefined' ? null : document.documentElement
}

export function applyConsoleTheme(enabled: boolean): void {
  const element = root()
  if (!element) return
  if (enabled) element.setAttribute(CONSOLE_THEME_ATTRIBUTE, CONSOLE_THEME_NAME)
  else element.removeAttribute(CONSOLE_THEME_ATTRIBUTE)
  consoleThemeApplied.value = enabled
  if (enabled) {
    stopPointerField ??= installPointerField(isConsoleThemeApplied)
  } else {
    stopPointerField?.()
    stopPointerField = undefined
  }
}

// Keep the appearance control available while a cached theme survives an unknown
// or failed state read. Business slots never use this presentation-only fallback.
export function consoleThemePresentationEnabled(store: ConsoleThemeState): boolean {
  return store.enabled(CONSOLE_THEME_ID)
    || ((!store.loaded || Boolean(store.error)) && consoleThemeApplied.value)
}

export function readCachedConsoleTheme(): boolean {
  try { return localStorage.getItem(CONSOLE_THEME_CACHE_KEY) === 'on' } catch { return false }
}

function cacheConsoleTheme(enabled: boolean): void {
  try { localStorage.setItem(CONSOLE_THEME_CACHE_KEY, enabled ? 'on' : 'off') } catch { /* storage unavailable */ }
}

export function restoreCachedConsoleTheme(): void {
  applyConsoleTheme(readCachedConsoleTheme())
}

// Follows the shared extension store. While the state is unknown (first load or a
// transient read failure) the last confirmed appearance is kept, which avoids a
// flash between the upstream and brand themes every refresh interval.
export function syncConsoleTheme(store: ConsoleThemeState): () => void {
  return watch(
    () => [store.loaded, store.error, store.enabled(CONSOLE_THEME_ID)] as const,
    ([loaded, error, enabled]) => {
      if (!loaded || error) return
      applyConsoleTheme(enabled)
      cacheConsoleTheme(enabled)
    },
    { immediate: true },
  )
}

export function isConsoleThemeApplied(): boolean {
  return root()?.hasAttribute(CONSOLE_THEME_ATTRIBUTE) ?? false
}

// Paint the cached state as soon as the runtime module is imported (before mount).
restoreCachedConsoleTheme()

if (import.meta.hot) {
  import.meta.hot.dispose(() => stopPointerField?.())
}
