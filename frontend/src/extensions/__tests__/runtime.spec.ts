import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { extensionAPI } from '../api'
import { extensionIds } from '../catalog'
import { installCustomExtensionGuard, useCustomExtensionRuntime } from '../runtime'
import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { EXTENSION_REFRESH_MS, EXTENSION_STORAGE_EVENT } from '../store'

vi.mock('../api', () => ({ extensionAPI: { publicState: vi.fn(), update: vi.fn() } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: true }) }))

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
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks(); vi.useRealTimers() })
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
    visibility.mockReturnValue('hidden')
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS)
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT }))
    expect(extensionAPI.publicState).not.toHaveBeenCalled()
    visibility.mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated' })); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT })); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(2)
    wrapper?.unmount(); wrapper = undefined
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS)
    document.dispatchEvent(new Event('visibilitychange'))
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT })); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(2)
  })
  it('does not poll extension storage while database setup is incomplete', async () => {
    await start('/setup')
    await vi.advanceTimersByTimeAsync(EXTENSION_REFRESH_MS)
    expect(extensionAPI.publicState).not.toHaveBeenCalled()
  })
})
