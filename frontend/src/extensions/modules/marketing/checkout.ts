import { computed, inject, provide, ref, watch, type InjectionKey } from 'vue'
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
  async function initialize() {
    if (!admission.value) return
    const loaded = await loadCheckoutCoupons()
    if (admission.value) coupons.value = loaded
  }
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
