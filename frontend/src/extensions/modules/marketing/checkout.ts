import { computed, inject, onScopeDispose, provide, ref, watch, type InjectionKey } from 'vue'
import type { CreateOrderRequest, OrderType, UserCoupon } from '@/types/payment'
import type { CheckoutAdjustmentContext, CheckoutAdjustmentContribution } from '../../checkout'
import { marketingAPI } from './api'
import { useMarketingAdmission } from './admission'
import type { Ref } from 'vue'

export async function loadCheckoutCoupons() {
  const response = await marketingAPI.getCoupons({ page: 1, page_size: 100, status: 'unused' })
  return response.data.items
}

function createController(context: CheckoutAdjustmentContext, admission: Readonly<Ref<boolean>>) {
  const coupons = ref<UserCoupon[]>([])
  const selectedId = ref<number | null>(null)
  const selected = computed(() => coupons.value.find(item => item.id === selectedId.value) ?? null)
  let initialized = false
  let disposed = false
  let revision = 0
  let inFlight: Promise<void> | undefined
  function eligible(coupon: UserCoupon, orderType: OrderType): boolean {
    return admission.value && !context.excludesAdjustments(orderType) && coupon.status === 'unused'
      && (coupon.scope === 'universal' || coupon.scope === orderType)
      && (coupon.threshold_amount <= 0 || context.orderAmount(orderType) >= coupon.threshold_amount)
  }
  function couponFor(orderType: OrderType) {
    const coupon = selected.value
    return coupon && eligible(coupon, orderType) ? coupon : null
  }
  function payableFor(orderType: OrderType, currency?: string): number | null {
    const coupon = couponFor(orderType)
    if (!coupon) return null
    const amount = Math.max(0.01, context.orderAmount(orderType) - coupon.discount_amount)
    return context.projectPayable(orderType, amount, currency)
  }
  function discountFor(orderType: OrderType): number {
    const payable = payableFor(orderType)
    return payable === null ? 0 : Math.round((context.baseAmount(orderType) - payable) * 100) / 100
  }
  // Clear incompatible selection synchronously on tab, threshold, status or
  // campaign exclusion changes. Scope is validated again at payload creation.
  watch(() => selected.value && eligible(selected.value, context.orderType.value), valid => {
    if (!valid) selectedId.value = null
  }, { flush: 'sync' })
  function loadCoupons(): Promise<void> {
    if (disposed || !admission.value) return Promise.resolve()
    if (inFlight) return inFlight
    const requestRevision = revision
    const request = loadCheckoutCoupons().then(loaded => {
      if (!disposed && admission.value && requestRevision === revision) coupons.value = loaded
    }).finally(() => {
      if (inFlight === request) inFlight = undefined
    })
    inFlight = request
    return request
  }
  function initialize(): Promise<void> {
    initialized = true
    return loadCoupons()
  }
  // A checkout mounted while disabled/unknown must recover without remounting.
  // Invalidate synchronously so an off -> on cycle cannot revive an older load.
  watch(admission, enabled => {
    revision++
    inFlight = undefined
    if (!enabled) selectedId.value = null
    else if (initialized) {
      // Background refresh failures leave native checkout available; a later
      // enable/recovery or explicit initialization can retry the coupon read.
      void loadCoupons().catch(() => {})
    }
  }, { flush: 'sync' })
  onScopeDispose(() => { disposed = true; revision++; inFlight = undefined })
  function prepareOrder(payload: CreateOrderRequest) {
    if (payload.wechat_resume_token) return
    if (!admission.value) {
      delete payload.user_coupon_id
      return
    }
    if (selectedId.value === null) return
    if (payload.order_type !== 'balance' && payload.order_type !== 'subscription') return
    const coupon = couponFor(payload.order_type)
    if (!coupon) throw new Error('优惠券已不适用于当前订单，请重新选择。')
    payload.user_coupon_id = coupon.id
  }
  return { admission, coupons, selectedId, couponFor, payableFor, discountFor, initialize, prepareOrder, context }
}

type CouponCheckoutController = ReturnType<typeof createController>
const couponCheckoutKey: InjectionKey<CouponCheckoutController> = Symbol('marketing-coupon-checkout')
export function createCouponCheckout(context: CheckoutAdjustmentContext, admission: Readonly<Ref<boolean>> = useMarketingAdmission()): CheckoutAdjustmentContribution {
  const controller = createController(context, admission)
  provide(couponCheckoutKey, controller)
  return controller
}
export function useCouponCheckout(): CouponCheckoutController {
  const controller = inject(couponCheckoutKey)
  if (!controller) throw new Error('Coupon checkout must be mounted inside its checkout extension host')
  return controller
}
