import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import RechargeCampaignTicker from '../RechargeCampaignTicker.vue'
import { campaignAPI, type RechargeCampaign } from '@/api/rechargeCampaigns'

vi.mock('@/api/rechargeCampaigns', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/rechargeCampaigns')>(),
  campaignAPI: { publicList: vi.fn() },
}))
const active: RechargeCampaign = { id: 1, name: '充值好礼', description: '', enabled: true, starts_at: '2026-09-20T00:00:00Z', ends_at: '2026-09-22T00:00:00Z', kind: 'bonus', percent: 10, min_amount: 0, reward_percent: 0, reward_cap: 0, freeze_hours: 0, new_invitees_only: false }
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })
describe('RechargeCampaignTicker', () => {
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
