import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useMarketingGeneration } from './generation'
import { useExtensionStore } from '../../store'
import { generate } from '@/api/admin/redeem'
import { redeemMarketingAPI } from './api'

vi.mock('@/api/admin/redeem', () => ({ generate: vi.fn() }))
vi.mock('./api', () => ({ redeemMarketingAPI: { generate: vi.fn() } }))

describe('marketing generation boundary', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks() })
  it('leaves native generation unchanged when unrestricted or non-marketing', async () => {
    const plugin = useMarketingGeneration()
    await plugin.generate(3, 'marketing', 10, undefined, undefined, 7)
    expect(generate).toHaveBeenCalledWith(3, 'marketing', 10, undefined, undefined, 7)
    plugin.options.newUsersOnly = true
    await plugin.generate(1, 'balance', 20)
    expect(generate).toHaveBeenLastCalledWith(1, 'balance', 20, undefined, undefined, undefined)
    expect(redeemMarketingAPI.generate).not.toHaveBeenCalled()
  })
  it('uses the atomic plugin endpoint for restricted batches and resets after success', async () => {
    useExtensionStore().flags = { 'redeem-marketing': true }
    vi.mocked(redeemMarketingAPI.generate).mockResolvedValue([])
    const plugin = useMarketingGeneration()
    plugin.options.newUsersOnly = true
    await plugin.generate(5, 'marketing', 2, null, undefined, 30)
    expect(redeemMarketingAPI.generate).toHaveBeenCalledWith(5, 2, 30, true)
    expect(generate).not.toHaveBeenCalled()
    expect(plugin.options.newUsersOnly).toBe(false)
  })
  it('never silently drops the restriction when disabled or on failure', async () => {
    const plugin = useMarketingGeneration()
    plugin.options.newUsersOnly = true
    await expect(plugin.generate(1, 'marketing', 10)).rejects.toThrow('插件已关闭')
    expect(generate).not.toHaveBeenCalled()
    useExtensionStore().flags = { 'redeem-marketing': true }
    vi.mocked(redeemMarketingAPI.generate).mockRejectedValue(new Error('network'))
    await expect(plugin.generate(1, 'marketing', 10)).rejects.toThrow('network')
    expect(plugin.options.newUsersOnly).toBe(true)
  })
  it('isolates state per page instance', () => {
    const a = useMarketingGeneration()
    const b = useMarketingGeneration()
    a.options.newUsersOnly = true
    expect(b.options.newUsersOnly).toBe(false)
  })
})
