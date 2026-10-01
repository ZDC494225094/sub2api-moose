import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { extensionAPI } from '@/extensions/api'
import { extensionIds } from '@/extensions/catalog'
import { useExtensionStore } from '@/extensions/store'
import { flushPromises, mount } from '@vue/test-utils'
import RechargeCampaignTicker from '../RechargeCampaignTicker.vue'
import { campaignAPI, type RechargeCampaign } from '@/extensions/modules/recharge-campaigns/api'

vi.mock('@/extensions/api', () => ({ extensionAPI: { publicState: vi.fn(), update: vi.fn() } }))
vi.mock('@/extensions/modules/recharge-campaigns/api', async importOriginal => ({
  ...await importOriginal<typeof import('@/extensions/modules/recharge-campaigns/api')>(),
  campaignAPI: { publicList: vi.fn() },
}))
const active: RechargeCampaign = { id: 1, name: '充值好礼', description: '', enabled: true, starts_at: '2026-09-20T00:00:00Z', ends_at: '2026-09-22T00:00:00Z', kind: 'bonus', percent: 10, min_amount: 0, reward_percent: 0, reward_cap: 0, freeze_hours: 0, new_invitees_only: false }
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })
describe('RechargeCampaignTicker', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, true])) } } as never)
    vi.mocked(extensionAPI.update).mockImplementation(async (id, enabled) => ({ data: { id, enabled } }) as never)
  })
  it('does not request or advertise campaigns when the extension is disabled', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    const wrapper = mount(RechargeCampaignTicker, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    expect(campaignAPI.publicList).not.toHaveBeenCalled()
    expect(wrapper.text()).toBe('')
    wrapper.unmount()
  })
  it('hides existing promotions immediately and ignores a response arriving after disablement', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-21T00:00:00Z'))
    let resolveRequest!: (value: never) => void
    vi.mocked(campaignAPI.publicList).mockResolvedValueOnce({ data: [active] } as never)
      .mockImplementationOnce(() => new Promise(resolve => { resolveRequest = resolve }))
    const wrapper = mount(RechargeCampaignTicker, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('充值好礼')
    await vi.advanceTimersByTimeAsync(60000)
    await useExtensionStore().setEnabled('recharge-campaigns', false)
    await flushPromises()
    expect(wrapper.text()).toBe('')
    resolveRequest({ data: [active] } as never); await flushPromises()
    expect(wrapper.text()).toBe('')
    await vi.advanceTimersByTimeAsync(60000)
    expect(campaignAPI.publicList).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('rotates active campaigns, pauses on hover, and removes expired campaigns', async () => {
    vi.useFakeTimers()
    vi.spyOn(window, 'matchMedia').mockImplementation(query => ({ matches: false, media: query, onchange: null, addListener: vi.fn(), removeListener: vi.fn(), addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn() }))
    vi.setSystemTime(new Date('2026-09-21T00:00:00Z'))
    vi.mocked(campaignAPI.publicList).mockResolvedValue({ data: [active, { ...active, id: 2, name: '九折福利', kind: 'discount', percent: 90 }, { ...active, id: 3, name: '未来活动', starts_at: '2026-09-23T00:00:00Z' }] } as never)
    const wrapper = mount(RechargeCampaignTicker, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, Transition: false } } })
    await flushPromises()
    expect(wrapper.text()).toContain('充值好礼')
    expect(wrapper.text()).not.toContain('未来活动')
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.text()).toContain('九折福利')
    await wrapper.trigger('mouseenter')
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.text()).toContain('九折福利')
    vi.setSystemTime(new Date(active.ends_at))
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.text()).toBe('')
    wrapper.unmount()
  })
})
