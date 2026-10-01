import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { extensionHistoricalPath } from '../route-access'
import { useExtensionStore } from '../store'
import { installCustomExtensionGuard, useCustomExtensionRuntime } from '../runtime'
import { extensionAPI } from '../api'
import { marketingRoutes } from '../modules/marketing/routes'
import { useMarketingAdmission } from '../modules/marketing/admission'
import { extensionIds } from '../catalog'

// Use the real managed catalog; no simulated promotion.
vi.mock('../api', () => ({ extensionAPI: { publicState: vi.fn() } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: true }) }))

let wrapper: VueWrapper | undefined
beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks() })
afterEach(() => { wrapper?.unmount(); wrapper = undefined })
function router() {
  const r = createRouter({ history: createMemoryHistory(), routes: [
    ...marketingRoutes.map(route => ({ ...route, component: { template: '<div />' } })),
    ...['/lottery/new', '/playground', '/admin/dashboard'].map(path => ({ path, component: { template: '<div />' } })),
  ] })
  installCustomExtensionGuard(r)
  return r
}

describe('disabled extension historical access', () => {
  it('only allows exact owner-bound historical pages', () => {
    for (const route of marketingRoutes) {
      expect(extensionHistoricalPath(route.path)).toBe(true)
      expect(extensionHistoricalPath(`${route.path}/?page=2#history`)).toBe(true)
      expect(extensionHistoricalPath(`${route.path}/new`)).toBe(false)
      expect(extensionHistoricalPath(`${route.path}-other`)).toBe(false)
      expect(route.meta).toMatchObject({ requiresAuth: true, requiresPayment: true })
      expect(route.meta?.requiresAdmin).toBe(route.path.startsWith('/admin/'))
    }
    expect(extensionHistoricalPath('/playground')).toBe(false)
  })
  it('preserves history navigation and sidebar visibility on missing/error state, not write admission', async () => {
    vi.mocked(extensionAPI.publicState).mockRejectedValue(new Error('offline'))
    const r = router(), store = useExtensionStore()
    for (const route of marketingRoutes) {
      await r.push(`${route.path}?page=2`)
      await flushPromises()
      expect(r.currentRoute.value.path).toBe(route.path)
      expect(r.currentRoute.value.query).toEqual({ page: '2' })
      expect(store.pathEnabled(route.path)).toBe(true)
    }
    expect(store.flags).toEqual({})
    expect(useMarketingAdmission().value).toBe(false)
    expect(store.error).not.toBe('')
    expect(store.pathEnabled('/lottery/new')).toBe(false)
    await r.push('/lottery/new')
    expect(r.currentRoute.value.path).toBe('/admin/dashboard')
  })
  it('does not wait on an unavailable state request to read history', async () => {
    vi.mocked(extensionAPI.publicState).mockImplementation(() => new Promise(() => {}))
    const r = router()
    for (const route of marketingRoutes) {
      await r.push(route.path)
      expect(r.currentRoute.value.path).toBe(route.path)
    }
  })
  it.each(['disabled', 'offline'])('does not evict the open historical page when admission becomes %s', async state => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, true])) } } as never)
    const r = router(), store = useExtensionStore()
    await r.push('/admin/orders/lottery?page=2')
    wrapper = mount(defineComponent({ setup() { useCustomExtensionRuntime(r) }, template: '<div />' }))
    await flushPromises()
    expect(useMarketingAdmission().value).toBe(true)
    if (state === 'offline') {
      vi.mocked(extensionAPI.publicState).mockRejectedValue(new Error('offline'))
    } else {
      vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    }
    await store.refresh(true)
    expect(useMarketingAdmission().value).toBe(false)
    await flushPromises()
    expect(r.currentRoute.value.fullPath).toBe('/admin/orders/lottery?page=2')
    expect(store.pathEnabled('/admin/orders/lottery')).toBe(true)
    expect(store.pathEnabled('/playground')).toBe(false)
  })
})


