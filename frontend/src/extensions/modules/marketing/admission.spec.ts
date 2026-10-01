import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useExtensionStore } from '../../store'
import { useMarketingAdmission } from './admission'
import { extensionAPI } from '../../api'

vi.mock('../../api', () => ({ extensionAPI: { publicState: vi.fn() } }))

beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks() })
describe('marketing admission readiness boundary', () => {
  it('defaults managed marketing to closed without an explicit flag', () => {
    expect(useMarketingAdmission().value).toBe(false)
  })
  it('uses real managed shared flags and fails closed on missing state', () => {
    const store = useExtensionStore(), allowed = useMarketingAdmission()
    expect(allowed.value).toBe(false)
    store.flags = { 'marketing-tools': true } as typeof store.flags
    expect(allowed.value).toBe(true)
    store.flags = { 'marketing-tools': false } as typeof store.flags
    expect(allowed.value).toBe(false)
    store.flags = {}
    expect(allowed.value).toBe(false)
  })
  it('drops managed admission when the existing shared refresh fails', async () => {
    const store = useExtensionStore(), allowed = useMarketingAdmission()
    store.flags = { 'marketing-tools': true } as typeof store.flags
    vi.mocked(extensionAPI.publicState).mockRejectedValue(new Error('offline'))
    await store.refresh(true)
    expect(allowed.value).toBe(false)
  })
})
