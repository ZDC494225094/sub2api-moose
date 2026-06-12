/**
 * User Payment API endpoints
 * Handles payment operations for regular users
 */

import { apiClient } from './client'
import type {
  PaymentConfig,
  SubscriptionPlan,
  PaymentChannel,
  MethodLimitsResponse,
  CheckoutInfoResponse,
  CreateOrderRequest,
  CreateOrderResult,
  PaymentOrder,
  UserCoupon,
  LotteryDrawResult,
  LotteryOverview
} from '@/types/payment'
import type { BasePaginationResponse } from '@/types'

export const paymentAPI = {
  /** Get payment configuration (enabled types, limits, etc.) */
  getConfig() {
    return apiClient.get<PaymentConfig>('/payment/config')
  },

  /** Get available subscription plans */
  getPlans() {
    return apiClient.get<SubscriptionPlan[]>('/payment/plans')
  },

  /** Get available payment channels */
  getChannels() {
    return apiClient.get<PaymentChannel[]>('/payment/channels')
  },

  /** Get all checkout page data in a single call */
  getCheckoutInfo() {
    return apiClient.get<CheckoutInfoResponse>('/payment/checkout-info')
  },

  /** Get payment method limits and fee rates */
  getLimits() {
    return apiClient.get<MethodLimitsResponse>('/payment/limits')
  },

  /** Get current user's usable coupons */
  getCoupons(params?: { page?: number; page_size?: number; status?: string; scope?: string }) {
    return apiClient.get<BasePaginationResponse<UserCoupon>>('/payment/coupons', { params })
  },

  /** Get active lottery activity */
  getActiveLottery() {
    return apiClient.get<LotteryOverview | Record<string, never>>('/payment/lottery/active')
  },

  /** Draw lottery */
  drawLottery(data: { activity_id: number; use_wallet?: boolean }) {
    return apiClient.post<LotteryDrawResult>('/payment/lottery/draw', data)
  },

  /** Get current user's draw history */
  getMyDrawRecords(params?: { activity_id?: number; page?: number; page_size?: number }) {
    return apiClient.get<{ items: import('@/types/payment').LotteryDrawRecord[]; total: number; page: number; page_size: number }>('/payment/lottery/my-records', { params })
  },

  /** Create a new payment order */
  createOrder(data: CreateOrderRequest) {
    return apiClient.post<CreateOrderResult>('/payment/orders', data)
  },

  /** Get current user's orders */
  getMyOrders(params?: { page?: number; page_size?: number; status?: string }) {
    return apiClient.get<BasePaginationResponse<PaymentOrder>>('/payment/orders/my', { params })
  },

  /** Get a specific order by ID */
  getOrder(id: number) {
    return apiClient.get<PaymentOrder>(`/payment/orders/${id}`)
  },

  /** Cancel a pending order */
  cancelOrder(id: number) {
    return apiClient.post(`/payment/orders/${id}/cancel`)
  },

  /** Verify order payment status with upstream provider */
  verifyOrder(outTradeNo: string) {
    return apiClient.post<PaymentOrder>('/payment/orders/verify', { out_trade_no: outTradeNo })
  },

  /** Legacy-compatible public order lookup by out_trade_no */
  verifyOrderPublic(outTradeNo: string) {
    return apiClient.post<PaymentOrder>('/payment/public/orders/verify', { out_trade_no: outTradeNo })
  },

  /** Resolve an order from a signed resume token without auth */
  resolveOrderPublicByResumeToken(resumeToken: string) {
    return apiClient.post<PaymentOrder>('/payment/public/orders/resolve', { resume_token: resumeToken })
  },

  /** Request a refund for a completed order */
  requestRefund(id: number, data: { reason: string }) {
    return apiClient.post(`/payment/orders/${id}/refund-request`, data)
  },

  /** Get provider instance IDs that allow user refund */
  getRefundEligibleProviders() {
    return apiClient.get<{ provider_instance_ids: string[] }>('/payment/orders/refund-eligible-providers')
  }
}
