import { describe, expect, it } from 'vitest'
import { marketingRoutes } from './routes'
import { customExtensionRoutes } from '../../routes'

describe('marketing route isolation', () => {
  it.each([
    ['/lottery', 'Lottery', false, 'nav.lottery'],
    ['/admin/orders/coupons', 'AdminPaymentCoupons', true, 'nav.couponTemplates'],
    ['/admin/orders/lottery', 'AdminPaymentLottery', true, 'nav.marketingLottery'],
  ] as const)('preserves %s route identity and security metadata', (path, name, admin, titleKey) => {
    const route = marketingRoutes.find(item => item.path === path)
    expect(route).toMatchObject({ name, meta: { requiresAuth: true, requiresAdmin: admin, requiresPayment: true, titleKey } })
    expect(typeof route?.component).toBe('function')
    expect(customExtensionRoutes.filter(item => item.path === path)).toEqual([route])
  })
  it('has exactly the original three routes, not new public marketing entry points', () => {
    expect(marketingRoutes).toHaveLength(3)
    expect(marketingRoutes.every(item => item.meta?.requiresAuth === true)).toBe(true)
  })
})
