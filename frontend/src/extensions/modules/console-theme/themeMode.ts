import { computed, onBeforeUnmount, onMounted, readonly, ref } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'

// Shared with main.ts, the brand homepage and the upstream sidebar toggle, so a
// choice made in any of them is honoured everywhere.
export const THEME_STORAGE_KEY = 'theme'

export function readThemeMode(): ThemeMode {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY)
    if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  } catch { /* storage unavailable */ }
  // main.ts treats a missing value as "follow the system".
  return 'system'
}

function systemQuery(): MediaQueryList | null {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia('(prefers-color-scheme: dark)')
    : null
}

// One preference is shared by the homepage, sidebar and extension switcher.
// The app runtime keeps its listeners alive on pages without any theme control.
const mode = ref<ThemeMode>(readThemeMode())
const systemDark = ref(!!systemQuery()?.matches)
const isDark = computed(() => mode.value === 'dark' || (mode.value === 'system' && systemDark.value))
let runtimeUsers = 0
let stopListeners: (() => void) | undefined

function apply() {
  if (typeof document !== 'undefined') document.documentElement.classList.toggle('dark', isDark.value)
}

function synchronize(media = systemQuery()) {
  mode.value = readThemeMode()
  systemDark.value = !!media?.matches
  apply()
}

function setMode(next: ThemeMode) {
  mode.value = next
  systemDark.value = !!systemQuery()?.matches
  try { localStorage.setItem(THEME_STORAGE_KEY, next) } catch { /* storage unavailable */ }
  apply()
}

function retainRuntime(): () => void {
  runtimeUsers += 1
  if (runtimeUsers === 1) {
    const media = systemQuery()
    synchronize(media)

    const onSystemChange = (event: MediaQueryListEvent) => {
      systemDark.value = event.matches
      if (mode.value === 'system') apply()
    }
    const onStorage = (event: StorageEvent) => {
      // localStorage.clear() emits a null key and restores the system default.
      if (event.key !== null && event.key !== THEME_STORAGE_KEY) return
      synchronize(media)
    }

    media?.addEventListener?.('change', onSystemChange)
    window.addEventListener('storage', onStorage)
    stopListeners = () => {
      media?.removeEventListener?.('change', onSystemChange)
      window.removeEventListener('storage', onStorage)
    }
  }

  let released = false
  return () => {
    if (released) return
    released = true
    runtimeUsers -= 1
    if (runtimeUsers === 0) {
      stopListeners?.()
      stopListeners = undefined
    }
  }
}

export function useThemeModeRuntime(): void {
  if (runtimeUsers === 0) synchronize()
  let release: (() => void) | undefined
  onMounted(() => { release = retainRuntime() })
  onBeforeUnmount(() => release?.())
}

export function useThemeMode() {
  // Controls also retain the runtime so they work when mounted independently;
  // reference counting keeps exactly one set of browser listeners installed.
  useThemeModeRuntime()
  return { mode: readonly(mode), isDark, setMode }
}
