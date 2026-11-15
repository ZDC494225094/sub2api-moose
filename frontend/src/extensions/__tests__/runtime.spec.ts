import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent, reactive } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { extensionAPI } from '../api'
import { extensionIds } from '../catalog'
import { installCustomExtensionGuard, prefetchCustomExtensionState, useCustomExtensionRuntime } from '../runtime'
import { EXTENSION_STORAGE_EVENT, useExtensionStore } from '../store'

vi.mock('../api', () => ({ extensionAPI: { publicState: vi.fn(), list: vi.fn(), update: vi.fn() } }))
const session = vi.hoisted(() => ({ current: { isAdmin: false, isAuthenticated: false } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => session.current }))
const flags = (value: boolean) => Object.fromEntries(extensionIds.map(id => [id, value]))
let wrapper: VueWrapper | undefined
beforeEach(() => {
  setActivePinia(createPinia()); vi.resetAllMocks()
  session.current = reactive({ isAdmin: false, isAuthenticated: false })
  vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: flags(true) } } as never)
  vi.mocked(extensionAPI.list).mockResolvedValue({ data: { items: extensionIds.map(id => ({ id, enabled: true })) } } as never)
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks(); vi.useRealTimers() })
function router() {
  const r = createRouter({ history: createMemoryHistory(), routes: [
    '/', '/home', '/docs', '/login', '/playground', '/dashboard', '/admin/operations',
    '/admin/dashboard', '/admin/custom-extensions', '/admin/settings', '/setup',
    '/lottery', '/admin/orders/coupons', '/admin/orders/lottery',
  ].map(path => ({ path, component: { template: '<div />' }, meta: { requiresAuth: !['/', '/home', '/docs', '/login'].includes(path) } })) })
  installCustomExtensionGuard(r)
  return r
}
async function start(path: string) {
  const r = router(); await r.push(path)
  wrapper = mount(defineComponent({ setup() { useCustomExtensionRuntime(r) }, template: '<div />' }))
  await flushPromises()
  return r
}

describe('nonblocking extension navigation', () => {
  it.each([false, true])('keeps unknown public state on the page (admin=%s)', async admin => {
    session.current.isAdmin = admin
    vi.mocked(extensionAPI.publicState).mockImplementation(() => new Promise(() => {}))
    const r = router()
    await r.push('/?aff=keep')
    expect(r.currentRoute.value.path).toBe('/')
    await r.push('/docs')
    expect(r.currentRoute.value.path).toBe('/docs')
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
  })
  it('returns known disabled public pages to a public home, never a console', async () => {
    const store = useExtensionStore()
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: flags(false) } } as never)
    await store.refresh()
    const r = router(); await r.push('/docs?from=menu#help')
    expect(r.currentRoute.value.fullPath).toBe('/home?from=menu#help')
    await r.push('/?aff=keep')
    expect(r.currentRoute.value.path).toBe('/home')
  })
  it('reuses warmup without waiting or evicting an already opened page', async () => {
    let resolve!: (value: unknown) => void
    vi.mocked(extensionAPI.publicState).mockImplementation(() => new Promise(done => { resolve = done }) as never)
    prefetchCustomExtensionState()
    const r = await start('/?aff=keep')
    expect(r.currentRoute.value.path).toBe('/')
    resolve({ data: { enabled: flags(false) } }); await flushPromises()
    expect(r.currentRoute.value.path).toBe('/')
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
  })
  it('does not check or redirect same-page anchor changes', async () => {
    const r = await start('/')
    useExtensionStore().flags = { 'premium-home': false }
    for (const hash of ['#plans', '#model-plaza', '#top']) {
      await r.push('/' + hash)
      expect(r.currentRoute.value.fullPath).toBe('/' + hash)
    }
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
  })
  it('keeps core navigation independent of plugin discovery', async () => {
    vi.mocked(extensionAPI.publicState).mockImplementation(() => new Promise(() => {}))
    const r = router()
    await r.push('/admin/settings'); await r.push('/dashboard')
    expect(r.currentRoute.value.path).toBe('/dashboard')
    expect(extensionAPI.publicState).not.toHaveBeenCalled()
  })
  it('retains marketing history even with a confirmed disabled plugin', async () => {
    useExtensionStore().flags = { 'marketing-tools': false }
    const r = router()
    for (const path of ['/lottery', '/admin/orders/coupons', '/admin/orders/lottery']) {
      await r.push(path); expect(r.currentRoute.value.path).toBe(path)
    }
  })
})

describe('event driven extension state', () => {
  it('loads core-page menus once, never polls or refetches on visibility/navigation', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'Date'] })
    const r = await start('/dashboard')
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(120000)
    document.dispatchEvent(new Event('visibilitychange'))
    await r.push('/playground'); await r.push('/docs'); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
  })
  it('syncs an explicit save across tabs without moving the current page', async () => {
    const r = await start('/playground?draft=keep')
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: flags(false) } } as never)
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT })); await flushPromises()
    expect(useExtensionStore().enabled('playground')).toBe(false)
    expect(r.currentRoute.value.fullPath).toBe('/playground?draft=keep')
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(2)
    window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated' })); await flushPromises()
    wrapper?.unmount(); wrapper = undefined
    window.dispatchEvent(new StorageEvent('storage', { key: EXTENSION_STORAGE_EVENT })); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(2)
  })
  it('loads admin-only configuration after login and clears it after logout', async () => {
    await start('/login')
    expect(extensionAPI.list).not.toHaveBeenCalled()
    session.current.isAuthenticated = true; session.current.isAdmin = true
    await flushPromises()
    expect(useExtensionStore().enabled('access-policy')).toBe(true)
    session.current.isAuthenticated = false; session.current.isAdmin = false
    await flushPromises()
    expect(useExtensionStore().enabled('access-policy')).toBe(false)
  })
  it('does not discover plugins until setup is complete', async () => {
    const r = await start('/setup')
    expect(extensionAPI.publicState).not.toHaveBeenCalled()
    await r.push('/login'); await flushPromises()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
  })
})
