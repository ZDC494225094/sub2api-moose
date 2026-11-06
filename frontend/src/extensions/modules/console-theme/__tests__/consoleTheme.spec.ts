import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent, h, nextTick, reactive } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import {
  CONSOLE_THEME_ATTRIBUTE,
  CONSOLE_THEME_CACHE_KEY,
  CONSOLE_THEME_ID,
  CONSOLE_THEME_NAME,
  readCachedConsoleTheme,
  restoreCachedConsoleTheme,
  syncConsoleTheme,
} from '..'
import ThemeSwitcher from '../ThemeSwitcher.vue'
import { THEME_STORAGE_KEY, readThemeMode, useThemeMode, useThemeModeRuntime } from '../themeMode'
import ExtensionSlot from '../../../components/ExtensionSlot.vue'
import { useExtensionStore } from '../../../store'

vi.mock('vue-router', () => ({ useRoute: () => reactive({ name: 'Dashboard' }) }))
enableAutoUnmount(afterEach)

// The test alias uses the runtime-only vue-i18n build, which needs precompiled
// (function) messages instead of strings.
const messages = { consoleTheme: { switcher: () => '切换主题', light: () => '浅色', dark: () => '深色', system: () => '跟随系统' } }
const i18n = () => createI18n({ legacy: false, locale: 'zh', messages: { zh: messages } })

function fakeStore(initial: { loaded: boolean; error?: string; enabled?: boolean }) {
  const state = reactive({ loaded: initial.loaded, error: initial.error ?? '', flags: { [CONSOLE_THEME_ID]: initial.enabled === true } as Record<string, boolean> })
  return Object.assign(state, { enabled: (id: string) => state.flags[id] === true })
}

let systemDark = false
const mediaListeners = new Set<(event: MediaQueryListEvent) => void>()
const originalMatchMedia = window.matchMedia

const root = () => document.documentElement
const RuntimeOnly = defineComponent({
  setup() {
    useThemeModeRuntime()
    return () => null
  },
})
const SidebarThemeToggle = defineComponent({
  setup() {
    const { mode, isDark, setMode } = useThemeMode()
    return () => h('button', {
      'data-mode': mode.value,
      onClick: () => setMode(isDark.value ? 'light' : 'dark'),
    }, isDark.value ? 'light' : 'dark')
  },
})

describe('console theme extension', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    root().removeAttribute(CONSOLE_THEME_ATTRIBUTE)
    root().classList.remove('dark')
    systemDark = false
    mediaListeners.clear()
    window.matchMedia = ((query: string) => ({
      get matches() { return query.includes('dark') && systemDark },
      media: query,
      onchange: null,
      addEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) => mediaListeners.add(listener),
      removeEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) => mediaListeners.delete(listener),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })) as unknown as typeof window.matchMedia
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
    root().removeAttribute(CONSOLE_THEME_ATTRIBUTE)
    root().classList.remove('dark')
  })

  it('applies the brand theme only while the module is enabled', async () => {
    const store = fakeStore({ loaded: true, enabled: true })
    const stop = syncConsoleTheme(store)
    expect(root().getAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(CONSOLE_THEME_NAME)
    expect(readCachedConsoleTheme()).toBe(true)

    store.flags[CONSOLE_THEME_ID] = false
    await nextTick()
    expect(root().hasAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(false)
    expect(localStorage.getItem(CONSOLE_THEME_CACHE_KEY)).toBe('off')

    stop()
    store.flags[CONSOLE_THEME_ID] = true
    await nextTick()
    expect(root().hasAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(false)
  })

  it('keeps the last confirmed appearance while the switch state is unknown', async () => {
    localStorage.setItem(CONSOLE_THEME_CACHE_KEY, 'on')
    restoreCachedConsoleTheme()
    expect(root().getAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(CONSOLE_THEME_NAME)

    const store = fakeStore({ loaded: false })
    syncConsoleTheme(store)
    expect(root().getAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(CONSOLE_THEME_NAME)

    // A failed refresh clears flags; presentation must not flash back to upstream.
    store.loaded = true
    store.error = 'state unavailable'
    await nextTick()
    expect(root().getAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(CONSOLE_THEME_NAME)

    store.error = ''
    await nextTick()
    expect(root().hasAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(false)
    expect(readCachedConsoleTheme()).toBe(false)
  })

  it('treats a missing or corrupt cache as the upstream look', () => {
    localStorage.setItem(CONSOLE_THEME_CACHE_KEY, 'yes')
    restoreCachedConsoleTheme()
    expect(root().hasAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(false)
  })

  it('reads only valid stored theme modes and defaults to the system', () => {
    expect(readThemeMode()).toBe('system')
    localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    expect(readThemeMode()).toBe('dark')
    localStorage.setItem(THEME_STORAGE_KEY, 'sepia')
    expect(readThemeMode()).toBe('system')
  })

  it('switches light, dark and system modes through the shared preference', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    const wrapper = mount(ThemeSwitcher, { global: { plugins: [i18n()] }, attachTo: document.body })
    const trigger = wrapper.get('[data-test="console-theme-trigger"]')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    await trigger.trigger('click')
    const items = wrapper.findAll('[role="menuitemradio"]')
    expect(items.map(item => item.text())).toEqual(['浅色', '深色', '跟随系统'])
    expect(wrapper.get('[data-mode="light"]').attributes('aria-checked')).toBe('true')

    await wrapper.get('[data-mode="dark"]').trigger('click')
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
    expect(root().classList.contains('dark')).toBe(true)
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    await trigger.trigger('click')
    await wrapper.get('[data-mode="system"]').trigger('click')
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('system')
    expect(root().classList.contains('dark')).toBe(false)

    // Following the system: an OS switch to dark applies without a reload.
    systemDark = true
    mediaListeners.forEach(listener => listener({ matches: true } as MediaQueryListEvent))
    await nextTick()
    expect(root().classList.contains('dark')).toBe(true)

    wrapper.unmount()
    expect(mediaListeners.size).toBe(0)
  })

  it('applies the initial preference and follows the system without a mounted switcher', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'system')
    systemDark = true
    const runtime = mount(RuntimeOnly)
    expect(root().classList.contains('dark')).toBe(true)
    expect(mediaListeners.size).toBe(1)

    systemDark = false
    mediaListeners.forEach(listener => listener({ matches: false } as MediaQueryListEvent))
    await nextTick()
    expect(root().classList.contains('dark')).toBe(false)

    const switcher = mount(ThemeSwitcher, { global: { plugins: [i18n()] } })
    expect(mediaListeners.size).toBe(1)
    switcher.unmount()
    expect(mediaListeners.size).toBe(1)
    runtime.unmount()
    expect(mediaListeners.size).toBe(0)
  })

  it('keeps the sidebar action and all switchers in sync in the same tab', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    const sidebar = mount(SidebarThemeToggle)
    const first = mount(ThemeSwitcher, { global: { plugins: [i18n()] } })
    const second = mount(ThemeSwitcher, { global: { plugins: [i18n()] } })
    expect(mediaListeners.size).toBe(1)

    await first.get('[data-test="console-theme-trigger"]').trigger('click')
    await first.get('[data-mode="dark"]').trigger('click')
    expect(sidebar.text()).toBe('light')
    expect(sidebar.get('button').attributes('data-mode')).toBe('dark')
    await second.get('[data-test="console-theme-trigger"]').trigger('click')
    expect(second.get('[data-mode="dark"]').attributes('aria-checked')).toBe('true')

    // Closing the extension reveals the native sidebar control; its first click
    // must invert the current appearance, not a setup-time snapshot.
    first.unmount()
    second.unmount()
    await sidebar.get('button').trigger('click')
    expect(root().classList.contains('dark')).toBe(false)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
  })

  it('synchronizes cross-tab changes, removal and storage clearing on pages without controls', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    const runtime = mount(RuntimeOnly)
    localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: 'dark' }))
    expect(root().classList.contains('dark')).toBe(true)

    const sidebar = mount(SidebarThemeToggle)
    expect(sidebar.get('button').attributes('data-mode')).toBe('dark')
    expect(sidebar.text()).toBe('light')
    systemDark = true
    localStorage.removeItem(THEME_STORAGE_KEY)
    window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: null }))
    await nextTick()
    expect(sidebar.get('button').attributes('data-mode')).toBe('system')
    expect(root().classList.contains('dark')).toBe(true)

    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: 'light' }))
    expect(root().classList.contains('dark')).toBe(false)
    localStorage.clear()
    window.dispatchEvent(new StorageEvent('storage', { key: null }))
    await nextTick()
    expect(sidebar.get('button').attributes('data-mode')).toBe('system')
    expect(root().classList.contains('dark')).toBe(true)

    sidebar.unmount()
    runtime.unmount()
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: 'light' }))
    expect(root().classList.contains('dark')).toBe(true)
  })

  it('honors an explicit cross-tab preference after subsequent OS changes', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'system')
    mount(RuntimeOnly)
    mount(SidebarThemeToggle)
    localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: 'dark' }))
    systemDark = false
    mediaListeners.forEach(listener => listener({ matches: false } as MediaQueryListEvent))
    expect(root().classList.contains('dark')).toBe(true)
  })

  it('closes the menu on outside clicks and Escape', async () => {
    const wrapper = mount(ThemeSwitcher, { global: { plugins: [i18n()] }, attachTo: document.body })
    await wrapper.get('[data-test="console-theme-trigger"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(true)
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    await wrapper.get('[data-test="console-theme-trigger"]').trigger('click')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('mounts the header switcher only with real managed enablement', async () => {
    const store = useExtensionStore()
    const wrapper = mount(ExtensionSlot, { props: { name: 'header-actions' }, global: { plugins: [i18n()] } })
    await flushPromises()
    expect(wrapper.find('[data-test="console-theme-trigger"]').exists()).toBe(false)
    store.flags = { [CONSOLE_THEME_ID]: true }
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()
    expect(wrapper.find('[data-test="console-theme-trigger"]').exists()).toBe(true)
    store.flags = { [CONSOLE_THEME_ID]: false }
    await flushPromises()
    expect(wrapper.find('[data-test="console-theme-trigger"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
