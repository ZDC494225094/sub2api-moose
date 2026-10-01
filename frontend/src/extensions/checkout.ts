import { computed, type ComputedRef } from 'vue'
import type { CreateOrderRequest, OrderType } from '@/types/payment'
import type { ExtensionId } from './catalog'
import { useExtensionStore } from './store'
import { createCampaignCheckout } from './modules/recharge-campaigns/checkout'
import { createCouponCheckout } from './modules/marketing/checkout'

export interface CheckoutQuote {
  principal: number
  credited: number
}

// Upstream owns its base quote; extensions contribute only through this port.
export interface CheckoutExtensionContext {
  amount: ComputedRef<number>
  baseQuote: ComputedRef<CheckoutQuote>
  balanceMultiplier: () => number
  feeRate: () => number
  mayRefresh: () => boolean
  formatPaymentAmount: (amount: number) => string
  sharer: () => { name: string; avatarUrl?: string } | undefined
}

export interface CheckoutContribution {
  id: ExtensionId
  quote: ComputedRef<CheckoutQuote | null>
  adjustsAmountLimits: ComputedRef<boolean>
  excludesCoupon: ComputedRef<boolean>
  canSubmit: ComputedRef<boolean>
  initialize: () => Promise<void>
  prepareOrder: (payload: CreateOrderRequest) => void
}

// Never choose a price by registration order. Multiple pricing replacements
// need an explicit composition policy; until then fail closed before checkout.
export function composeCheckoutContributions(context: CheckoutExtensionContext, contributions: readonly CheckoutContribution[]) {
  const replacements = computed(() => contributions.filter(item => item.quote.value !== null))
  const conflict = computed(() => replacements.value.length > 1)
  const quote = computed(() => replacements.value.length === 1
    ? replacements.value[0]!.quote.value! : context.baseQuote.value)
  const adjusted = computed(() => replacements.value.length > 0)
  const adjustsAmountLimits = computed(() => contributions.some(item => item.adjustsAmountLimits.value))
  const excludesCoupon = computed(() => contributions.some(item => item.excludesCoupon.value))
  const canSubmit = computed(() => !conflict.value && contributions.every(item => item.canSubmit.value))
  function prepareOrder(payload: CreateOrderRequest): void {
    // Subscription and resumed orders keep their native/historical contracts.
    if (payload.order_type !== 'balance' || payload.wechat_resume_token) return
    if (!canSubmit.value) throw new Error('扩展报价尚未就绪或存在冲突，请刷新后重试。')
    for (const item of contributions) item.prepareOrder(payload)
  }
  return { quote, adjusted, adjustsAmountLimits, excludesCoupon, canSubmit, prepareOrder }
}

export function useCheckoutExtensions(context: CheckoutExtensionContext) {
  const store = useExtensionStore()
  const contributions = [createCampaignCheckout(context)]
  async function initialize(): Promise<void> {
    await store.refresh()
    await Promise.all(contributions.map(item => item.initialize()))
  }
  return { ...composeCheckoutContributions(context, contributions), initialize }
}


export interface CheckoutAdjustmentContext {
  orderType: ComputedRef<OrderType>
  baseAmount: (orderType: OrderType, currency?: string) => number
  // Discounts apply to the order principal before gateway conversion and fees.
  orderAmount: (orderType: OrderType) => number
  projectPayable: (orderType: OrderType, adjustedAmount: number, currency?: string) => number
  excludesAdjustments: (orderType: OrderType) => boolean
  formatPaymentAmount: (amount: number) => string
}

export interface CheckoutAdjustmentContribution {
  payableFor: (orderType: OrderType, currency?: string) => number | null
  initialize: () => Promise<void>
  prepareOrder: (payload: CreateOrderRequest) => void
}

export function composeCheckoutAdjustments(context: CheckoutAdjustmentContext, contributions: readonly CheckoutAdjustmentContribution[]) {
  const replacements = (orderType: OrderType, currency?: string) => contributions.map(item => item.payableFor(orderType, currency)).filter(value => value !== null)
  const canSubmit = computed(() => replacements(context.orderType.value).length <= 1)
  function payableFor(orderType: OrderType, currency?: string): number {
    const values = replacements(orderType, currency)
    // Never pick a winning price by plugin registration order.
    return values.length === 1 ? values[0]! : context.baseAmount(orderType, currency)
  }
  async function initialize() { await Promise.all(contributions.map(item => item.initialize())) }
  function prepareOrder(payload: CreateOrderRequest) {
    // Resume tokens carry historical pricing; do not attach today's selection.
    if (payload.wechat_resume_token) return
    if (payload.order_type !== 'balance' && payload.order_type !== 'subscription') return
    if (replacements(payload.order_type).length > 1) throw new Error('结算扩展报价冲突，请刷新后重试。')
    for (const item of contributions) item.prepareOrder(payload)
  }
  return { payableFor, canSubmit, initialize, prepareOrder }
}

export function useCheckoutAdjustments(context: CheckoutAdjustmentContext) {
  return composeCheckoutAdjustments(context, [createCouponCheckout(context)])
}
