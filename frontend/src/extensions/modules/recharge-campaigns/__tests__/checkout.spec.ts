import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { computed, defineComponent, h, ref } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { getAffiliateDetail } from '@/api/user'
import type { CreateOrderRequest } from '@/types/payment'
import { useExtensionStore } from '../../../store'
import { composeCheckoutContributions, type CheckoutContribution, type CheckoutExtensionContext } from '../../../checkout'
import { createCampaignCheckout, useCampaignCheckout } from '../checkout'
import { campaignAPI, type RechargeCampaign } from '../api'

vi.mock('@/api/user', () => ({ getAffiliateDetail: vi.fn() }))
vi.mock('../api', async original => ({ ...await original<typeof import('../api')>(), campaignAPI: { publicList: vi.fn() } }))
const campaign: RechargeCampaign = { id: 7, revision: 'v1', name: 'gift', description: '', enabled: true, starts_at: '2020-01-01T00:00:00Z', ends_at: '2099-01-01T00:00:00Z', kind: 'bonus', percent: 10, min_amount: 0, reward_percent: 5, reward_cap: 10, freeze_hours: 72, new_invitees_only: true }
const wrappers: VueWrapper[] = []
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
function context(amount = ref(100)): CheckoutExtensionContext {
  return { amount: computed(() => amount.value), baseQuote: computed(() => ({ principal: amount.value, credited: Math.round(amount.value * 100) / 100 })), balanceMultiplier: () => 1, feeRate: () => 0, mayRefresh: () => true, formatPaymentAmount: value => `$${value.toFixed(2)}`, sharer: () => ({ name: 'tester' }) }
}
function harness(enabled = true, amount = ref(100)) {
  const store = useExtensionStore(); store.flags = { 'recharge-campaigns': enabled }
  let contribution!: CheckoutContribution
  let controller!: ReturnType<typeof useCampaignCheckout>
  let host!: ReturnType<typeof composeCheckoutContributions>
  const ctx = context(amount)
  const Probe = defineComponent({ setup() { controller = useCampaignCheckout(); return () => h('span') } })
  const wrapper = mount(defineComponent({ setup() { contribution = createCampaignCheckout(ctx); host = composeCheckoutContributions(ctx, [contribution]); return () => h(Probe) } }))
  wrappers.push(wrapper)
  return { wrapper, store, host, controller, contribution }
}
function order(extra: Partial<CreateOrderRequest> = {}): CreateOrderRequest { return { amount: 100, payment_type: 'alipay', order_type: 'balance', ...extra } }

beforeEach(() => {
  setActivePinia(createPinia()); vi.useFakeTimers(); vi.setSystemTime(new Date('2026-09-26T00:00:00Z'))
  vi.mocked(campaignAPI.publicList).mockReset().mockResolvedValue({ data: [campaign] } as never)
  vi.mocked(getAffiliateDetail).mockReset().mockResolvedValue({ aff_code: 'friend' } as never)
})
afterEach(() => { for (const wrapper of wrappers.splice(0)) wrapper.unmount(); vi.useRealTimers() })

describe('recharge checkout plugin isolation', () => {
  it('disabled uses the unmodified native quote, no API, no timers, no order fields', async () => {
    const { host, contribution } = harness(false, ref(10.075))
    await contribution.initialize()
    expect(host.quote.value).toEqual({ principal: 10.075, credited: Math.round(10.075 * 100) / 100 })
    expect(host.adjusted.value).toBe(false); expect(host.excludesCoupon.value).toBe(false)
    const payload = order(); host.prepareOrder(payload); expect(payload).toEqual(order())
    expect(campaignAPI.publicList).not.toHaveBeenCalled(); expect(vi.getTimerCount()).toBe(0)
  })
  it('preserves automatic bonus, discount, quick-amount and versioned request behavior', async () => {
    const { host, contribution, controller } = harness()
    await contribution.initialize()
    expect(host.quote.value).toEqual({ principal: 100, credited: 110 })
    expect(host.excludesCoupon.value).toBe(true); expect(host.adjustsAmountLimits.value).toBe(true)
    expect(controller.quickCampaignLabel(50)).toBe('到账 $55.00')
    const payload = order(); host.prepareOrder(payload)
    expect(payload).toMatchObject({ campaign_id: 7, campaign_revision: 'v1' })
    vi.mocked(campaignAPI.publicList).mockResolvedValue({ data: [{ ...campaign, kind: 'discount', percent: 90 }] } as never)
    await contribution.initialize()
    expect(host.quote.value).toEqual({ principal: 90, credited: 100 })
    expect(controller.quickCampaignLabel(100)).toBe('实付 $90.00')
  })
  it('does not contribute when a campaign is expired or its threshold is unmet', async () => {
    vi.mocked(campaignAPI.publicList).mockResolvedValue({ data: [{ ...campaign, min_amount: 200 }, { ...campaign, ends_at: '2025-01-01T00:00:00Z' }] } as never)
    const { host, contribution } = harness(); await contribution.initialize()
    expect(host.quote.value).toEqual({ principal: 100, credited: 100 }); expect(host.adjusted.value).toBe(false)
    const payload = order(); host.prepareOrder(payload); expect(payload).toEqual(order())
  })
  it('blocks new checkout until the first enabled quote has been resolved', async () => {
    const request = deferred<{ data: RechargeCampaign[] }>(); vi.mocked(campaignAPI.publicList).mockReturnValue(request.promise as never)
    const { contribution, host, controller } = harness(); const pending = contribution.initialize()
    expect(host.canSubmit.value).toBe(false); expect(controller.pendingInitial.value).toBe(true)
    request.resolve({ data: [] }); await pending
    expect(host.canSubmit.value).toBe(true); expect(controller.pendingInitial.value).toBe(false)
  })
  it('selects request fields for the submitted amount, not a changed UI amount', async () => {
    vi.mocked(campaignAPI.publicList).mockResolvedValue({ data: [{ ...campaign, min_amount: 75 }] } as never)
    const { contribution, host } = harness(); await contribution.initialize()
    expect(host.adjusted.value).toBe(true)
    const payload = order({ amount: 50 }); host.prepareOrder(payload)
    expect(payload.campaign_id).toBeUndefined()
  })
  it('deduplicates in-flight loading', async () => {
    const request = deferred<{ data: RechargeCampaign[] }>(); vi.mocked(campaignAPI.publicList).mockReturnValue(request.promise as never)
    const { contribution } = harness(); const first = contribution.initialize(); const second = contribution.initialize()
    expect(first).toBe(second); expect(campaignAPI.publicList).toHaveBeenCalledTimes(1)
    request.resolve({ data: [] }); await first
  })
  it('invalidates a late result on disable and restores native pricing and timers immediately', async () => {
    const request = deferred<{ data: RechargeCampaign[] }>(); vi.mocked(campaignAPI.publicList).mockReturnValue(request.promise as never)
    const { contribution, store, host } = harness(); const pending = contribution.initialize()
    store.flags['recharge-campaigns'] = false
    request.resolve({ data: [campaign] }); await pending
    expect(host.quote.value).toEqual({ principal: 100, credited: 100 }); expect(host.adjusted.value).toBe(false)
    expect(vi.getTimerCount()).toBe(0)
  })
  it('does not let an old request overwrite a newer off/on generation', async () => {
    const old = deferred<{ data: RechargeCampaign[] }>(); const fresh = deferred<{ data: RechargeCampaign[] }>()
    vi.mocked(campaignAPI.publicList).mockReturnValueOnce(old.promise as never).mockReturnValueOnce(fresh.promise as never)
    const { contribution, store, host } = harness(); const pending = contribution.initialize()
    store.flags['recharge-campaigns'] = false; store.flags['recharge-campaigns'] = true
    fresh.resolve({ data: [{ ...campaign, percent: 20, revision: 'v2' }] }); await flushPromises()
    old.resolve({ data: [campaign] }); await pending
    expect(host.quote.value.credited).toBe(120)
    const payload = order(); host.prepareOrder(payload); expect(payload.campaign_revision).toBe('v2')
  })
  it('blocks uncertain new prices on failure but does not interrupt signed resumes or subscriptions', async () => {
    vi.mocked(campaignAPI.publicList).mockRejectedValue(new Error('unavailable'))
    const { contribution, host, store, controller } = harness(); await contribution.initialize()
    expect(host.canSubmit.value).toBe(false); expect(controller.campaignError.value).toContain('活动加载失败')
    expect(() => host.prepareOrder(order())).toThrow('报价')
    expect(() => host.prepareOrder(order({ order_type: 'subscription', plan_id: 1 }))).not.toThrow()
    const resumed = order({ wechat_resume_token: 'signed-historical-snapshot' })
    host.prepareOrder(resumed); expect(resumed.campaign_id).toBeUndefined()
    store.flags['recharge-campaigns'] = false
    expect(host.canSubmit.value).toBe(true); expect(controller.campaignError.value).toBe('')
  })
  it('does not reopen sharing from an affiliate request that finished after disable', async () => {
    const affiliate = deferred<Awaited<ReturnType<typeof getAffiliateDetail>>>()
    vi.mocked(getAffiliateDetail).mockReturnValue(affiliate.promise)
    const { contribution, controller, store } = harness(); await contribution.initialize()
    const pending = controller.shareCampaign(); store.flags['recharge-campaigns'] = false
    affiliate.resolve({ aff_code: 'late' } as Awaited<ReturnType<typeof getAffiliateDetail>>); await pending
    expect(controller.sharingCampaign.value).toBeNull(); expect(controller.affiliateCode.value).toBe('')
  })
  it('keeps share selection bound to the amount that requested it', async () => {
    const affiliate = deferred<Awaited<ReturnType<typeof getAffiliateDetail>>>()
    vi.mocked(getAffiliateDetail).mockReturnValue(affiliate.promise)
    const amount = ref(100); const { contribution, controller } = harness(true, amount); await contribution.initialize()
    const pending = controller.shareCampaign(); amount.value = 0
    affiliate.resolve({ aff_code: 'late' } as Awaited<ReturnType<typeof getAffiliateDetail>>); await pending
    expect(controller.sharingCampaign.value).toBeNull()
  })
  it('disposes pending results and all clocks on unmount', async () => {
    const request = deferred<{ data: RechargeCampaign[] }>(); vi.mocked(campaignAPI.publicList).mockReturnValue(request.promise as never)
    const { contribution, wrapper, controller } = harness(); const pending = contribution.initialize()
    wrapper.unmount(); request.resolve({ data: [campaign] }); await pending
    expect(controller.selectedCampaign.value).toBeNull(); expect(vi.getTimerCount()).toBe(0)
  })
})

describe('checkout contribution contract', () => {
  it('uses native behavior without any contributions', () => {
    const host = composeCheckoutContributions(context(), [])
    expect(host.quote.value).toEqual({ principal: 100, credited: 100 }); expect(host.canSubmit.value).toBe(true)
  })
  it('rejects competing replacement prices rather than choosing by registration order', () => {
    const prepare = vi.fn()
    const contribution: CheckoutContribution = { id: 'recharge-campaigns', quote: computed(() => ({ principal: 90, credited: 100 })), adjustsAmountLimits: computed(() => true), excludesCoupon: computed(() => true), canSubmit: computed(() => true), initialize: async () => {}, prepareOrder: prepare }
    const host = composeCheckoutContributions(context(), [contribution, { ...contribution }])
    expect(host.canSubmit.value).toBe(false); expect(() => host.prepareOrder(order())).toThrow('冲突')
    expect(prepare).not.toHaveBeenCalled()
  })
})
