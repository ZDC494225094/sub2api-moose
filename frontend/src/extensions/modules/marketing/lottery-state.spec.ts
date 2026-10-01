import { describe, expect, it } from 'vitest'
import type { LotteryOverview } from '@/types/payment'
import { availableLotteryChances } from './lottery-state'

describe('lottery read-only grant preview', () => {
  const overview = (granted: boolean, remaining: number, pending: number): LotteryOverview => ({
    user_state: { default_granted: granted, available_draw_times: remaining, total_granted_times: 0, total_drawn_times: 0, total_wallet_paid_amount: 0 },
    draw_eligibility: { consume_threshold_met: true, qualified_amount: 0, required_threshold_amount: 0, wallet_draw_enabled: false, can_draw_with_wallet: false, pending_default_draw_times: pending },
  })
  it('previews first-draw entitlement without modifying stored state', () => {
    const value = overview(false, 0, 3)
    expect(availableLotteryChances(value)).toBe(3)
    expect(value.user_state?.available_draw_times).toBe(0)
    expect(value.user_state?.default_granted).toBe(false)
  })
  it('never double counts a pending preview after Draw returns the granted state', () => {
    expect(availableLotteryChances(overview(true, 2, 3))).toBe(2)
    expect(availableLotteryChances(overview(true, 0, 3))).toBe(0)
  })
  it('preserves previously granted rights and tolerates absent legacy preview fields', () => {
    const value = overview(false, 2, 3)
    expect(availableLotteryChances(value)).toBe(5)
    delete value.draw_eligibility?.pending_default_draw_times
    expect(availableLotteryChances(value)).toBe(2)
    expect(availableLotteryChances(null)).toBe(0)
  })
})
