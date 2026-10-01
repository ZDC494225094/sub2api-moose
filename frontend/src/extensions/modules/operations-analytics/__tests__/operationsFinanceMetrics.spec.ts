import { describe, expect, it } from 'vitest'
import { cashMoney, paymentTotals } from '../operationsFinanceMetrics'
const payments = [
  { day: '2026-09-01', currency: 'CNY', recharge_paid: 70, subscription_paid: 30, credited: 100, pending_credit: 50 },
  { day: '2026-09-02', currency: 'USD', recharge_paid: 5, subscription_paid: 0, credited: 8, pending_credit: 0 },
  { day: '2026-09-03', currency: 'CNY', recharge_paid: 20, subscription_paid: 10, credited: 30, pending_credit: 0 }
]
describe('finance cash and credit units', () => {
  it('never combines different payment currencies, but adds USD credits', () => {
    expect(paymentTotals(payments)).toEqual({ currencies: [
      { currency: 'CNY', recharge_paid: 90, subscription_paid: 40 },
      { currency: 'USD', recharge_paid: 5, subscription_paid: 0 }
    ], credited: 138, pending: 50 })
  })
  it('uses inclusive day bounds for a clicked period rather than all-range totals', () => {
    expect(paymentTotals(payments, '2026-09-02', '2026-09-02')).toEqual({
      currencies: [{ currency: 'USD', recharge_paid: 5, subscription_paid: 0 }], credited: 8, pending: 0
    })
    expect(paymentTotals(payments, '2026-09-04')).toEqual({ currencies: [], credited: 0, pending: 0 })
  })
  it('identifies payment currency explicitly and distinguishes it from credits', () => {
    expect(cashMoney(70, 'CNY')).toBe('¥70.00 元')
    expect(cashMoney(5, 'USD')).toBe('USD 5.00')
    expect(cashMoney(8, 'EUR')).toBe('EUR 8.00')
  })
})
