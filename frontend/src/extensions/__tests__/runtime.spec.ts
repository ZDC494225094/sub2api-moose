import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { getPublicSettings } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'
import { extensionAPI } from '../api'
import { extensionIds } from '../catalog'
import { installCustomExtensionGuard, useCustomExtensionRuntime } from '../runtime'
import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { EXTENSION_REFRESH_MS, EXTENSION_STORAGE_EVENT, useExtensionStore } from '../store'

vi.mock('../api', () => ({ extensionAPI: { publicState: vi.fn(), update: vi.fn() } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: true }) }))
vi.mock('@/api/auth', () => ({ getPublicSettings: vi.fn() }))
vi.mock('@/api/admin/system', () => ({ checkUpdates: vi.fn() }))

const settings = (customized: boolean) => ({
  site_name: 'Test site',
  custom_menu_items: customized ? [{ id: 'guide', label: 'Guide', url: 'https://example.com/guide', visibility: 'user', sort_order: 0 }] : [],
  footer_friend_links: customized ? [{ label: 'Friend', url: 'https://example.com/friend' }] : [],
  custom_endpoints: customized ? [{ name: 'Custom', endpoint: 'https://example.com/api', description: '' }] : [],
} as PublicSettings)

describe('extension navigation gate', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks() })
  const router = () => {
    const r = createRouter({ history: createMemoryHistory(), routes: [
      '/', '/home', '/lottery', '/admin/orders/coupons', '/admin/orders/lottery', '/playground', '/admin/operations', '/admin/dashboard', '/admin/custom-extensions', '/setup',
    ].map(path => ({ path, component: { template: '<div />' } })) })
    installCustomExtensionGuard(r); return r
  }
  it('returns to upstream home when the brand home is disabled', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    const r = router(); await r.push('/?aff=ref123'); expect(r.currentRoute.value.path).toBe('/home')
    expect(r.currentRoute.value.query).toEqual({ aff: 'ref123' })
  })
  it('blocks direct navigation, not only sidebar entries', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    const r = router(); await r.push('/admin/operations'); expect(r.currentRoute.value.path).toBe('/admin/dashboard')
    await r.push('/admin/custom-extensions'); expect(r.currentRoute.value.path).toBe('/admin/custom-extensions')
  })
  it('retains managed marketing historical routes while new business is disabled', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    const r = router()
    for (const path of ['/lottery', '/admin/orders/coupons', '/admin/orders/lottery']) {
      await r.push(path)
      expect(r.currentRoute.value.path).toBe(path)
    }
  })
  it('allows enabled routes', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, true])) } } as never)
    const r = router(); await r.push('/playground'); expect(r.currentRoute.value.path).toBe('/playground')
  })
  it('does not block host navigation on an unavailable extension API', async () => {
    vi.mocked(extensionAPI.publicState).mockImplementation(() => new Promise(() => {}))
    const r = router()
    await r.push('/admin/custom-extensions')
    expect(r.currentRoute.value.path).toBe('/admin/custom-extensions')
    await r.push('/admin/dashboard')
    expect(r.currentRoute.value.path).toBe('/admin/dashboard')
  })
  it('never depends on database setup to enter the setup wizard', async () => {
    const r = router(); await r.push('/setup'); expect(extensionAPI.publicState).not.toHaveBeenCalled()
  })
})


describe('extension runtime synchronization', () => {
  let wrapper: VueWrapper | undefined
  let visibility: ReturnType<typeof vi.spyOn>
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'Date'] })
    visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, true])) } } as never)
    vi.mocked(getPublicSettings).mockResolvedValue(settings(true))
    delete window.__APP_CONFIG__
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; delete window.__APP_CONFIG__; vi.restoreAllMocks(); vi.useRealTimers() })
  async function start(path: string) {
    const r = createRouter({ history: createMemoryHistory(), routes: [
      '/playground', '/admin/dashboard', '/admin/custom-extensions', '/setup',
    ].map(path => ({ path, component: { template: '<div />' } })) })
    await r.push(path)
    wrapper = mount(defineComponent({ setup() { useCustomExtensionRuntime(r); return {} }, template: '<div />' }))
    await flushPromises()
    return r
  }
  it('refreshes an open page and redirects after disablement without dropping its query', async () => {
    const r = await start('/playground?prompt=draft')
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS); await flushPromises()
    expect(r.currentRoute.value.path).toBe('/admin/dashboard')
    expect(r.currentRoute.value.query).toEqual({ prompt: 'draft' })
  })
  it('refreshes for visibility and relevant cross-tab events and removes listeners and timers', async () => {
    await start('/admin/custom-extensions')
    vi.mocked(extensionAPI.publicState).mockClear()
    vi.mocked(getPublicSettings).mockClear()
    visibility.mockReturnValue('hidden')
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS)
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT }))
    expect(extensionAPI.publicState).not.toHaveBeenCalled()
    expect(getPublicSettings).not.toHaveBeenCalled()
    visibility.mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
    expect(getPublicSettings).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated' })); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT })); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(2)
    expect(getPublicSettings).toHaveBeenCalledTimes(2)
    wrapper?.unmount(); wrapper = undefined
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS)
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT })); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(2)
    expect(getPublicSettings).toHaveBeenCalledTimes(2)
  })
  it('refreshes cached customization settings on enable and disable without reloading the page', async () => {
    const app = useAppStore()
    window.__APP_CONFIG__ = settings(false)
    app.initFromInjectedConfig()
    await start('/admin/custom-extensions')
    expect(app.cachedPublicSettings).toMatchObject(settings(true))
    expect(window.__APP_CONFIG__).toMatchObject(settings(true))

    vi.mocked(getPublicSettings).mockResolvedValue(settings(false))
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS); await flushPromises()
    expect(app.cachedPublicSettings).toMatchObject(settings(false))
    expect(window.__APP_CONFIG__).toMatchObject(settings(false))

    vi.mocked(getPublicSettings).mockResolvedValue(settings(true))
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, true])) } } as never)
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT })); await flushPromises()
    expect(app.cachedPublicSettings).toMatchObject(settings(true))
  })
  it('refreshes settings immediately after a same-tab customization save', async () => {
    await start('/admin/custom-extensions')
    vi.mocked(getPublicSettings).mockClear().mockResolvedValue(settings(false))
    vi.mocked(extensionAPI.update).mockResolvedValue({ data: { id: 'site-customization', enabled: false } } as never)
    await useExtensionStore().setEnabled('site-customization', false); await flushPromises()
    expect(getPublicSettings).toHaveBeenCalledTimes(1)
    expect(useAppStore().cachedPublicSettings).toMatchObject(settings(false))
  })
  it('follows an older in-flight settings read with a fresh read after a saved toggle', async () => {
    await start('/admin/custom-extensions')
    let resolve!: (value: PublicSettings) => void
    vi.mocked(getPublicSettings).mockClear().mockReturnValueOnce(new Promise(done => { resolve = done }))
      .mockResolvedValue(settings(false))
    const pending = useAppStore().fetchPublicSettings(true)
    expect(useAppStore().publicSettingsLoading).toBe(true)
    expect(useExtensionStore().flags['site-customization']).toBe(true)
    vi.mocked(extensionAPI.update).mockResolvedValue({ data: { id: 'site-customization', enabled: false } } as never)
    await useExtensionStore().setEnabled('site-customization', false)
    expect(useExtensionStore().flags['site-customization']).toBe(false)
    expect(getPublicSettings).toHaveBeenCalledTimes(1)
    resolve(settings(true)); await pending; await flushPromises()
    expect(getPublicSettings).toHaveBeenCalledTimes(2)
    expect(useAppStore().cachedPublicSettings).toMatchObject(settings(false))
  })
  it('does not start a queued settings read after unmount', async () => {
    await start('/admin/custom-extensions')
    let resolve!: (value: PublicSettings) => void
    vi.mocked(getPublicSettings).mockClear().mockReturnValueOnce(new Promise(done => { resolve = done }))
    const pending = useAppStore().fetchPublicSettings(true)
    useExtensionStore().flags['site-customization'] = false
    wrapper?.unmount(); wrapper = undefined
    resolve(settings(true)); await pending; await flushPromises()
    expect(getPublicSettings).toHaveBeenCalledTimes(1)
  })
  it('does not poll extension storage while database setup is incomplete', async () => {
    await start('/setup')
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS)
    expect(extensionAPI.publicState).not.toHaveBeenCalled()
    expect(getPublicSettings).not.toHaveBeenCalled()
  })
})
