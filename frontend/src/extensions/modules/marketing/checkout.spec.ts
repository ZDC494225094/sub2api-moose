import { createPinia, setActivePinia } from 'pinia'
import { useExtensionStore } from '../../store'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { computed, defineComponent, h, nextTick, ref } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { CreateOrderRequest, OrderType, UserCoupon } from '@/types/payment'
import { composeCheckoutAdjustments, type CheckoutAdjustmentContext } from '../../checkout'
import { createCouponCheckout, useCouponCheckout } from './checkout'
import { marketingAPI } from './api'
import CouponSelection from './CouponSelection.vue'
import CouponSummary from './CouponSummary.vue'
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('./api', () => ({ marketingAPI: { getCoupons: vi.fn() } }))
const wrappers: VueWrapper[] = []
function order(extra: Partial<CreateOrderRequest> = {}): CreateOrderRequest {
  return { amount: 100, payment_type: 'alipay', order_type: 'balance', ...extra }
}
function harness(renderUI = false, sharedAdmission = false) {
  const admission = ref(true)
  const amount = ref(100), type = ref<OrderType>('balance'), excluded = ref(false)
  const ctx: CheckoutAdjustmentContext = {
    orderType: computed(() => type.value), orderAmount: () => amount.value,
    baseAmount: t => t === 'balance' ? 110 : 770,
    projectPayable: (t, value) => Math.round(value * (t === 'balance' ? 1.1 : 7.7) * 100) / 100,
    excludesAdjustments: t => t === 'balance' && excluded.value, formatPaymentAmount: String,
  }
  let controller!: ReturnType<typeof useCouponCheckout>
  let host!: ReturnType<typeof composeCheckoutAdjustments>
  const Probe = defineComponent({ setup() { controller = useCouponCheckout(); return () => renderUI ? h('div', [h(CouponSelection, { orderType: type.value }), h(CouponSummary, { orderType: type.value })]) : h('span') } })
  const wrapper = mount(defineComponent({ setup() {
    host = composeCheckoutAdjustments(ctx, [createCouponCheckout(ctx, sharedAdmission ? undefined : admission)]); return () => h(Probe)
  } }), { global: { stubs: { Select: true } } })
  wrappers.push(wrapper)
  vi.mocked(marketingAPI.getCoupons).mockResolvedValue({ data: { items: [
    { id: 7, status: 'unused', scope: 'universal', threshold_amount: 100, discount_amount: 20, coupon_code: 'HISTORY' } as UserCoupon,
  ] } } as never)
  return { wrapper, admission, controller, host, amount, type, excluded, ctx }
}
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.clearAllMocks() })
describe('coupon checkout contribution', () => {
  it('preserves native quote and payload without selection', async () => {
    const { host } = harness(); await host.initialize(); expect(host.payableFor('balance')).toBe(110)
    const payload = order(); host.prepareOrder(payload); expect(payload).toEqual(order())
  })
  it('discounts principal before fee and subscription currency projection', async () => {
    const { host, controller } = harness(); await host.initialize(); controller.selectedId.value = 7
    expect(host.payableFor('balance')).toBe(88); expect(host.payableFor('subscription')).toBe(616)
    const payload = order(); host.prepareOrder(payload); expect(payload.user_coupon_id).toBe(7); expect(payload.amount).toBe(100)
  })
  it('never qualifies using fees or converted totals', async () => {
    const { host, controller, amount } = harness(); await host.initialize(); controller.selectedId.value = 7
    amount.value = 99; expect(controller.selectedId.value).toBeNull(); expect(host.payableFor('balance')).toBe(110)
  })
  it('clears incompatible scope and status but excludes campaigns only for balance', async () => {
    const { host, controller, type, excluded } = harness(); await host.initialize()
    controller.coupons.value[0]!.scope = 'balance'; controller.selectedId.value = 7
    type.value = 'subscription'; expect(controller.selectedId.value).toBeNull()
    controller.coupons.value[0]!.scope = 'universal'; controller.selectedId.value = 7
    excluded.value = true; expect(controller.selectedId.value).toBe(7)
    type.value = 'balance'; expect(controller.selectedId.value).toBeNull()
    excluded.value = false; controller.selectedId.value = 7
    controller.coupons.value[0]!.status = 'used'; expect(controller.selectedId.value).toBeNull()
  })
  it('floors principal before projection and preserves historical resume fields', async () => {
    const { host, controller } = harness(); await host.initialize()
    controller.coupons.value[0]!.discount_amount = 200; controller.selectedId.value = 7
    expect(host.payableFor('subscription')).toBe(0.08)
    const payload = order({ wechat_resume_token: 'history', user_coupon_id: 42 }); host.prepareOrder(payload)
    expect(payload.user_coupon_id).toBe(42)
  })
  it('returns to native pricing on disable without erasing historical coupons or resume fields', async () => {
    const { host, controller, admission } = harness(); await host.initialize()
    controller.selectedId.value = 7
    expect(host.payableFor('balance')).toBe(88)
    admission.value = false
    expect(controller.selectedId.value).toBeNull()
    expect(host.payableFor('balance')).toBe(110)
    expect(host.payableFor('subscription')).toBe(770)
    expect(controller.coupons.value).toHaveLength(1)
    const payload = order({ user_coupon_id: 7 }); host.prepareOrder(payload)
    expect(payload).toEqual(order())
    const historical = order({ wechat_resume_token: 'history', user_coupon_id: 42 })
    host.prepareOrder(historical); expect(historical.user_coupon_id).toBe(42)
    admission.value = true
    expect(controller.selectedId.value).toBeNull()
    expect(host.payableFor('balance')).toBe(110)
    controller.selectedId.value = 7; expect(host.payableFor('balance')).toBe(88)
  })
  it('unmounts coupon selection and summary on disable while retaining native checkout', async () => {
    const { host, controller, admission, wrapper } = harness(true)
    await host.initialize(); controller.selectedId.value = 7; await nextTick()
    expect(wrapper.text()).toContain('userLottery.availableCoupons')
    expect(wrapper.text()).toContain('payment.discountCoupon')
    admission.value = false; await nextTick()
    expect(wrapper.text()).not.toContain('userLottery.availableCoupons')
    expect(wrapper.text()).not.toContain('payment.discountCoupon')
    expect(host.canSubmit.value).toBe(true)
    expect(host.payableFor('balance')).toBe(110)
    admission.value = true; await nextTick()
    expect(wrapper.text()).toContain('userLottery.availableCoupons')
    expect(wrapper.text()).not.toContain('payment.discountCoupon')
  })
  it('does not request coupons while disabled or accept late loads after disable', async () => {
    const { host, controller, admission } = harness()
    admission.value = false; await host.initialize()
    expect(marketingAPI.getCoupons).not.toHaveBeenCalled()
    let resolve!: (value: never) => void
    vi.mocked(marketingAPI.getCoupons).mockReturnValueOnce(new Promise(done => { resolve = done }))
    admission.value = true
    const pending = host.initialize(); admission.value = false
    resolve({ data: { items: [{ id: 7 }] } } as never); await pending
    expect(controller.coupons.value).toEqual([])
    expect(host.payableFor('balance')).toBe(110)
  })
  it('waits for checkout initialization and reloads automatically after an initially disabled state', async () => {
    const { host, controller, admission } = harness()
    admission.value = false; admission.value = true
    expect(marketingAPI.getCoupons).not.toHaveBeenCalled()
    admission.value = false; await host.initialize()
    expect(marketingAPI.getCoupons).not.toHaveBeenCalled()
    admission.value = true; await flushPromises()
    expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(1)
    expect(controller.coupons.value.map(coupon => coupon.id)).toEqual([7])
    expect(controller.selectedId.value).toBeNull()
  })
  it('deduplicates initialization and an automatic recovery load', async () => {
    const { host, admission } = harness()
    admission.value = false; await host.initialize()
    let resolve!: (value: never) => void
    vi.mocked(marketingAPI.getCoupons).mockReturnValueOnce(new Promise(done => { resolve = done }))
    admission.value = true
    const first = host.initialize(), second = host.initialize()
    expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(1)
    resolve({ data: { items: [] } } as never); await Promise.all([first, second])
  })
  it.each(['older-first', 'newer-first'])('ignores a pre-disable response after rapid re-enable (%s)', async completionOrder => {
    const { host, controller, admission } = harness()
    let resolveOld!: (value: never) => void
    let resolveNew!: (value: never) => void
    vi.mocked(marketingAPI.getCoupons)
      .mockReturnValueOnce(new Promise(done => { resolveOld = done }))
      .mockReturnValueOnce(new Promise(done => { resolveNew = done }))
    const oldRequest = host.initialize()
    admission.value = false; admission.value = true
    const newRequest = host.initialize()
    expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(2)
    const oldResponse = { data: { items: [{ id: 1 }] } } as never
    const newResponse = { data: { items: [{ id: 2 }] } } as never
    if (completionOrder === 'older-first') {
      resolveOld(oldResponse); await oldRequest
      expect(controller.coupons.value).toEqual([])
      // Completing the stale request must not clear the newer in-flight read.
      const joined = host.initialize()
      expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(2)
      resolveNew(newResponse); await Promise.all([joined, newRequest])
    } else {
      resolveNew(newResponse); await newRequest
      resolveOld(oldResponse); await oldRequest
    }
    expect(controller.coupons.value.map(coupon => coupon.id)).toEqual([2])
    expect(controller.selectedId.value).toBeNull()
  })
  it('contains a failed background reload and retries on the next recovery', async () => {
    const { host, controller, admission } = harness()
    admission.value = false; await host.initialize()
    vi.mocked(marketingAPI.getCoupons).mockRejectedValueOnce(new Error('offline'))
    admission.value = true; await flushPromises()
    expect(controller.coupons.value).toEqual([])
    expect(host.payableFor('balance')).toBe(110)
    admission.value = false; admission.value = true; await flushPromises()
    expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(2)
    expect(controller.coupons.value.map(coupon => coupon.id)).toEqual([7])
  })
  it('ignores a late response and stops reloading after checkout unmounts', async () => {
    const { host, controller, admission, wrapper } = harness()
    let resolve!: (value: never) => void
    vi.mocked(marketingAPI.getCoupons).mockReturnValueOnce(new Promise(done => { resolve = done }))
    const pending = host.initialize()
    wrapper.unmount()
    resolve({ data: { items: [{ id: 7 }] } } as never); await pending
    admission.value = false; admission.value = true; await host.initialize()
    expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(1)
    expect(controller.coupons.value).toEqual([])
  })
  it('rejects conflicting replacements without mutating requests', () => {
    const { ctx } = harness(); const prepareOrder = vi.fn()
    const host = composeCheckoutAdjustments(ctx, [1, 2].map(value => ({ payableFor: () => value, initialize: async () => {}, prepareOrder })))
    expect(host.canSubmit.value).toBe(false); expect(host.payableFor('balance')).toBe(110)
    expect(() => host.prepareOrder(order())).toThrow('冲突'); expect(prepareOrder).not.toHaveBeenCalled()
  })
})


describe('managed marketing checkout integration', () => {
  it('uses the real shared flag and restores upstream pricing without altering resumed orders', async () => {
    setActivePinia(createPinia())
    const store = useExtensionStore()
    const { host, controller } = harness(false, true)
    await host.initialize()
    expect(marketingAPI.getCoupons).not.toHaveBeenCalled()
    store.flags = { 'marketing-tools': true }
    await flushPromises()
    expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(1)
    controller.selectedId.value = 7
    expect(host.payableFor('balance')).toBe(88)
    const enabled = order(); host.prepareOrder(enabled)
    expect(enabled.user_coupon_id).toBe(7)
    store.flags['marketing-tools'] = false
    expect(controller.selectedId.value).toBeNull()
    expect(host.payableFor('balance')).toBe(110)
    const disabled = order({ user_coupon_id: 7 }); host.prepareOrder(disabled)
    expect(disabled).toEqual(order())
    const historical = order({ wechat_resume_token: 'signed-history', user_coupon_id: 7 })
    const before = { ...historical }; host.prepareOrder(historical)
    expect(historical).toEqual(before)
    store.flags['marketing-tools'] = true
    expect(controller.selectedId.value).toBeNull()
    expect(host.payableFor('balance')).toBe(110)
    store.flags = {}
    expect(controller.admission.value).toBe(false)
  })
  it('reloads options when unavailable shared extension state recovers', async () => {
    setActivePinia(createPinia())
    const store = useExtensionStore()
    const { host, controller } = harness(false, true)
    await host.initialize()
    store.flags = { 'marketing-tools': true }; await flushPromises()
    controller.selectedId.value = 7
    store.flags = {}
    expect(controller.selectedId.value).toBeNull()
    vi.mocked(marketingAPI.getCoupons).mockResolvedValueOnce({ data: { items: [] } } as never)
    store.flags = { 'marketing-tools': true }; await flushPromises()
    expect(marketingAPI.getCoupons).toHaveBeenCalledTimes(2)
    expect(controller.coupons.value).toEqual([])
    expect(host.payableFor('balance')).toBe(110)
  })
})
