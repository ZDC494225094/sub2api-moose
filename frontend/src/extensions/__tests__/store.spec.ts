import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { extensionAPI } from '../api'
import { adminExtensionIds, extensionIds, publicExtensionIds, type ExtensionId } from '../catalog'
import { useExtensionStore } from '../store'
import { useSiteCustomizationAdmission } from '../modules/site-customization/admission'

vi.mock('../api', () => ({ extensionAPI: { publicState: vi.fn(), list: vi.fn(), update: vi.fn() } }))
const auth = { isAdmin: false }
const all = (enabled: boolean) => Object.fromEntries(extensionIds.map(id => [id, enabled])) as Record<ExtensionId, boolean>
const response = (enabled: boolean) => ({ data: { enabled: all(enabled) } })

describe('custom extension runtime store', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks(); auth.isAdmin = false })
  it('fails closed before initialization without hiding core menus', () => {
    const store = useExtensionStore()
    expect(store.pathEnabled('/playground')).toBe(false)
    expect(store.pathEnabled('/admin/custom-extensions')).toBe(true)
    expect(store.pathEnabled('/admin/orders')).toBe(true)
  })
  it('loads flags and deduplicates reads', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue(response(true) as never)
    const store = useExtensionStore()
    await Promise.all([store.refresh(), store.refresh()]); await store.refresh()
    expect(extensionAPI.publicState).toHaveBeenCalledTimes(1)
    expect(store.enabled('playground')).toBe(true)
  })
  it('registers site customization and reacts to its persisted switch without gating routes', async () => {
    expect(extensionIds).toContain('site-customization')
    vi.mocked(extensionAPI.publicState).mockResolvedValue(response(true) as never)
    vi.mocked(extensionAPI.update).mockResolvedValue({ data: { id: 'site-customization', enabled: false } } as never)
    const store = useExtensionStore()
    const admitted = useSiteCustomizationAdmission()
    expect(admitted.value).toBe(false)
    await store.refresh()
    expect(admitted.value).toBe(true)
    await store.setEnabled('site-customization', false)
    expect(extensionAPI.update).toHaveBeenCalledWith('site-customization', false)
    expect(admitted.value).toBe(false)
    expect(store.enabled('playground')).toBe(true)
    expect(store.pathEnabled('/admin/announcements')).toBe(true)
  })
  it('fails closed when the public state omits site customization', async () => {
    const enabled: Partial<Record<ExtensionId, boolean>> = all(true)
    delete enabled['site-customization']
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled } } as never)
    const store = useExtensionStore()
    await store.refresh()
    expect(store.enabled('site-customization')).toBe(false)
    expect(store.error).not.toBe('')
  })
  it('clears stale enabled state on a fetch error', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValueOnce(response(true) as never).mockRejectedValueOnce(new Error('offline'))
    const store = useExtensionStore(); await store.refresh(); await store.refresh(true)
    expect(store.enabled('playground')).toBe(false)
    expect(store.error).not.toBe('')
  })
  it('rejects incomplete or non-boolean state', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: { playground: 'true' } } } as never)
    const store = useExtensionStore(); await store.refresh()
    expect(store.enabled('playground')).toBe(false); expect(store.error).not.toBe('')
  })
  it('only applies a switch after successful persistence', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue(response(true) as never)
    vi.mocked(extensionAPI.update).mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ data: { id: 'playground', enabled: false } } as never)
    const store = useExtensionStore(); await store.refresh()
    await expect(store.setEnabled('playground', false)).rejects.toThrow()
    expect(store.enabled('playground')).toBe(true)
    await store.setEnabled('playground', false)
    expect(store.enabled('playground')).toBe(false); expect(store.enabled('premium-home')).toBe(true)
  })
  it('never needs admin-only switches for regular users', async () => {
    const enabled = Object.fromEntries(publicExtensionIds.map(id => [id, true]))
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled } } as never)
    const store = useExtensionStore(); await store.refresh()
    expect(store.error).toBe('')
    expect(store.enabled('playground')).toBe(true)
    expect(store.enabled('gateway-log-archive')).toBe(false)
    expect(extensionAPI.list).not.toHaveBeenCalled()
  })
  it('loads admin-only switches from the admin list once the user is an admin', async () => {
    const enabled = Object.fromEntries(publicExtensionIds.map(id => [id, true]))
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled } } as never)
    vi.mocked(extensionAPI.list).mockResolvedValue({ data: { items: extensionIds.map(id => ({ id, managed: true, enabled: true })) } } as never)
    const store = useExtensionStore(); store.setAdminSession(() => auth.isAdmin); await store.refresh()
    expect(store.enabled('gateway-log-archive')).toBe(false)
    auth.isAdmin = true
    await store.refresh() // a role change bypasses the refresh cache
    for (const id of adminExtensionIds) expect(store.enabled(id)).toBe(true)
    auth.isAdmin = false
    await store.refresh()
    expect(store.enabled('gateway-log-archive')).toBe(false)
  })
  it('keeps public switches when the admin list fails', async () => {
    auth.isAdmin = true
    vi.mocked(extensionAPI.publicState).mockResolvedValue(response(true) as never)
    vi.mocked(extensionAPI.list).mockRejectedValue(new Error('offline'))
    const store = useExtensionStore(); store.setAdminSession(() => auth.isAdmin); await store.refresh()
    expect(store.enabled('playground')).toBe(true)
    expect(store.enabled('gateway-log-archive')).toBe(false)
    expect(store.error).not.toBe('')
  })
  it('does not let an older public request overwrite a saved toggle', async () => {
    let resolve!: (value: unknown) => void
    vi.mocked(extensionAPI.publicState).mockImplementation(() => new Promise(done => { resolve = done }) as never)
    vi.mocked(extensionAPI.update).mockResolvedValue({ data: { id: 'playground', enabled: false } } as never)
    const store = useExtensionStore(); const fetch = store.refresh()
    await store.setEnabled('playground', false); resolve(response(true)); await fetch
    expect(store.enabled('playground')).toBe(false)
  })
})
