import { apiClient } from '@/api/client'
import type { BasePaginationResponse } from '@/types'
import type { UserCoupon, LotteryDrawResult, LotteryOverview, CouponTemplate, LotteryActivity, LotteryPrize } from '@/types/payment'

// Module-owned endpoints; retain the host client for auth and response handling.
export const marketingAPI = {
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

}

export const adminMarketingAPI = {
  // ==================== Coupon Templates ====================

  getCouponTemplates(params?: {
    page?: number
    page_size?: number
    status?: string
    scope?: string
    search?: string
  }) {
    return apiClient.get<BasePaginationResponse<CouponTemplate>>('/admin/payment/coupon-templates', { params })
  },

  createCouponTemplate(data: Partial<CouponTemplate>) {
    return apiClient.post<CouponTemplate>('/admin/payment/coupon-templates', data)
  },

  updateCouponTemplate(id: number, data: Partial<CouponTemplate>) {
    return apiClient.put<CouponTemplate>(`/admin/payment/coupon-templates/${id}`, data)
  },

  // ==================== Lottery ====================

  getLotteryActivities(params?: {
    page?: number
    page_size?: number
    status?: string
    search?: string
  }) {
    return apiClient.get<BasePaginationResponse<LotteryActivity>>('/admin/payment/lottery/activities', { params })
  },

  createLotteryActivity(data: Partial<LotteryActivity>) {
    return apiClient.post<LotteryActivity>('/admin/payment/lottery/activities', data)
  },

  updateLotteryActivity(id: number, data: Partial<LotteryActivity>) {
    return apiClient.put<LotteryActivity>(`/admin/payment/lottery/activities/${id}`, data)
  },

  deleteLotteryActivity(id: number) {
    return apiClient.delete(`/admin/payment/lottery/activities/${id}`)
  },

  createLotteryPrize(data: Partial<LotteryPrize>) {
    return apiClient.post<LotteryPrize>('/admin/payment/lottery/prizes', data)
  },

  updateLotteryPrize(id: number, data: Partial<LotteryPrize>) {
    return apiClient.put<LotteryPrize>(`/admin/payment/lottery/prizes/${id}`, data)
  },

  getLotteryDrawRecords(activityId: number, params?: { page?: number; page_size?: number }) {
    return apiClient.get<import('@/types').BasePaginationResponse<import('@/types/payment').LotteryDrawRecord>>(`/admin/payment/lottery/activities/${activityId}/draw-records`, { params })
  },

}
