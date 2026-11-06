import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import {
  CONSOLE_THEME_ATTRIBUTE,
  CONSOLE_THEME_CACHE_KEY,
  CONSOLE_THEME_ID,
  CONSOLE_THEME_NAME,
  restoreCachedConsoleTheme,
  syncConsoleTheme,
} from '..'
import ExtensionSlot from '../../../components/ExtensionSlot.vue'
import { extensionAPI } from '../../../api'
import { extensionIds, type ExtensionId } from '../../../catalog'
import { useExtensionStore } from '../../../store'

vi.mock('vue-router', () => ({ useRoute: () => reactive({ name: 'Home' }) }))
vi.mock('../../../api', () => ({ extensionAPI: { publicState: vi.fn() } }))
vi.mock('../ThemeSwitcher.vue', () => ({ __esModule: true, default: { template: '<button data-theme-switcher />' } }))
vi.mock('../../customer-support/SupportWidget.vue', () => ({ __esModule: true, default: { template: '<aside data-support />' } }))
vi.mock('../../marketing/CouponSelection.vue', () => ({ __esModule: true, default: { template: '<aside data-coupon-selection />' } }))

function flags(enabled: ExtensionId[] = []): Record<ExtensionId, boolean> {
  return Object.fromEntries(extensionIds.map(id => [id, enabled.includes(id)])) as Record<ExtensionId, boolean>
}

async function settleSlots(): Promise<void> {
  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()
}

function mountSlots() {
  return mount(defineComponent({
    components: { ExtensionSlot },
    template: '<main><ExtensionSlot name="header-actions" /><ExtensionSlot name="application-overlay" /><ExtensionSlot name="checkout-discount-selection" /></main>',
  }))
}

describe('console theme slot presentation fallback', () => {
  let stopTheme: (() => void) | undefined

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(extensionAPI.publicState).mockReset()
    localStorage.clear()
    restoreCachedConsoleTheme()
  })

  afterEach(() => {
    stopTheme?.()
    stopTheme = undefined
    localStorage.clear()
    restoreCachedConsoleTheme()
  })

  it('keeps the cached theme control through unknown and failed reads, then removes it on confirmed disablement', async () => {
    localStorage.setItem(CONSOLE_THEME_CACHE_KEY, 'on')
    restoreCachedConsoleTheme()
    const store = useExtensionStore()
    stopTheme = syncConsoleTheme(store)
    const wrapper = mountSlots()
    await settleSlots()

    expect(store.loaded).toBe(false)
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(true)
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(false)

    vi.mocked(extensionAPI.publicState).mockRejectedValueOnce(new Error('unavailable'))
    await store.refresh(true)
    await settleSlots()
    expect(store.flags).toEqual({})
    expect(store.error).not.toBe('')
    expect(document.documentElement.getAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(CONSOLE_THEME_NAME)
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(true)
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(false)

    vi.mocked(extensionAPI.publicState).mockResolvedValueOnce({ data: { enabled: flags() } } as Awaited<ReturnType<typeof extensionAPI.publicState>>)
    await store.refresh(true)
    await settleSlots()
    expect(store.error).toBe('')
    expect(document.documentElement.hasAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(false)
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(false)
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('retains only the theme control when a refresh clears previously enabled business flags', async () => {
    const store = useExtensionStore()
    store.flags = flags([CONSOLE_THEME_ID, 'customer-support', 'marketing-tools'])
    store.loaded = true
    stopTheme = syncConsoleTheme(store)
    const wrapper = mountSlots()
    await settleSlots()
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(true)
    expect(wrapper.find('[data-support]').exists()).toBe(true)
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(true)

    vi.mocked(extensionAPI.publicState).mockRejectedValueOnce(new Error('unavailable'))
    await store.refresh(true)
    await settleSlots()
    expect(store.flags).toEqual({})
    expect(document.documentElement.getAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(CONSOLE_THEME_NAME)
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(true)
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(false)

    vi.mocked(extensionAPI.publicState).mockResolvedValueOnce({ data: { enabled: flags() } } as Awaited<ReturnType<typeof extensionAPI.publicState>>)
    await store.refresh(true)
    await settleSlots()
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(false)
    expect(document.documentElement.hasAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(false)
    wrapper.unmount()
  })

  it('does not activate the theme control after a failed read without an applied cached theme', async () => {
    const store = useExtensionStore()
    stopTheme = syncConsoleTheme(store)
    const wrapper = mountSlots()
    await settleSlots()
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(false)

    vi.mocked(extensionAPI.publicState).mockRejectedValueOnce(new Error('unavailable'))
    await store.refresh(true)
    await settleSlots()
    expect(document.documentElement.hasAttribute(CONSOLE_THEME_ATTRIBUTE)).toBe(false)
    expect(wrapper.find('[data-theme-switcher]').exists()).toBe(false)
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(false)
    wrapper.unmount()
  })
})
