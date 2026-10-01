import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '@/api/client'
import { marketingAPI, adminMarketingAPI } from './api'
import { loadCheckoutCoupons } from './checkout'

vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }))
beforeEach(() => vi.resetAllMocks())

describe('marketing HTTP contracts', () => {
  const params = { page: 2, page_size: 30 }
  const template = { name: 'coupon', discount_amount: 5 }
  const activity = { name: 'draw' }
  const prize = { name: 'prize' }
  const cases = [
    { name: 'user coupons', method: 'get', path: '/payment/coupons', args: [{ params }], call: () => marketingAPI.getCoupons(params) },
    { name: 'active lottery', method: 'get', path: '/payment/lottery/active', args: [], call: () => marketingAPI.getActiveLottery() },
    { name: 'draw', method: 'post', path: '/payment/lottery/draw', args: [{ activity_id: 7, use_wallet: true }], call: () => marketingAPI.drawLottery({ activity_id: 7, use_wallet: true }) },
    { name: 'own records', method: 'get', path: '/payment/lottery/my-records', args: [{ params }], call: () => marketingAPI.getMyDrawRecords(params) },
    { name: 'templates', method: 'get', path: '/admin/payment/coupon-templates', args: [{ params }], call: () => adminMarketingAPI.getCouponTemplates(params) },
    { name: 'create template', method: 'post', path: '/admin/payment/coupon-templates', args: [template], call: () => adminMarketingAPI.createCouponTemplate(template) },
    { name: 'update template', method: 'put', path: '/admin/payment/coupon-templates/7', args: [template], call: () => adminMarketingAPI.updateCouponTemplate(7, template) },
    { name: 'activities', method: 'get', path: '/admin/payment/lottery/activities', args: [{ params }], call: () => adminMarketingAPI.getLotteryActivities(params) },
    { name: 'create activity', method: 'post', path: '/admin/payment/lottery/activities', args: [activity], call: () => adminMarketingAPI.createLotteryActivity(activity) },
    { name: 'update activity', method: 'put', path: '/admin/payment/lottery/activities/7', args: [activity], call: () => adminMarketingAPI.updateLotteryActivity(7, activity) },
    { name: 'delete activity', method: 'delete', path: '/admin/payment/lottery/activities/7', args: [], call: () => adminMarketingAPI.deleteLotteryActivity(7) },
    { name: 'create prize', method: 'post', path: '/admin/payment/lottery/prizes', args: [prize], call: () => adminMarketingAPI.createLotteryPrize(prize) },
    { name: 'update prize', method: 'put', path: '/admin/payment/lottery/prizes/7', args: [prize], call: () => adminMarketingAPI.updateLotteryPrize(7, prize) },
    { name: 'admin records', method: 'get', path: '/admin/payment/lottery/activities/7/draw-records', args: [{ params }], call: () => adminMarketingAPI.getLotteryDrawRecords(7, params) },
  ] as const
  it.each(cases)('$name keeps verb, URL, payload and response envelope', async ({ method, path, args, call }) => {
    const response = { data: { items: [{ id: 7 }], total: 1 }, status: 200 }
    vi.mocked(apiClient[method]).mockResolvedValue(response)
    expect(await call()).toBe(response)
    expect(apiClient[method]).toHaveBeenCalledTimes(1)
    expect(apiClient[method]).toHaveBeenCalledWith(path, ...args)
  })
  it('checkout exposes coupon items without leaking transport details into the host', async () => {
    const items = [{ id: 7 }]
    vi.mocked(apiClient.get).mockResolvedValue({ data: { items } })
    expect(await loadCheckoutCoupons()).toBe(items)
    expect(apiClient.get).toHaveBeenCalledTimes(1)
    expect(apiClient.get).toHaveBeenCalledWith('/payment/coupons', { params: { page: 1, page_size: 100, status: 'unused' } })
  })
  it('preserves failed coupon loading instead of fabricating successful empty data', async () => {
    const error = new Error('unavailable')
    vi.mocked(apiClient.get).mockRejectedValue(error)
    await expect(loadCheckoutCoupons()).rejects.toBe(error)
  })
})
