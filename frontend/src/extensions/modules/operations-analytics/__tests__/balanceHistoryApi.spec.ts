import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getOperationsBalanceHistory, type BalanceMonthlyStats } from '../api'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

describe('balance history API', () => {
  beforeEach(() => { get.mockReset() })

  it.each([3, 6, 12] as const)('requests %i months and forwards cancellation', async months => {
    const points: BalanceMonthlyStats[] = [{ period: '2026-09', total_balance: 50, total_subscription_remaining: 75, realtime: true }]
    const controller = new AbortController()
    get.mockResolvedValue({ data: points })

    expect(await getOperationsBalanceHistory({ months }, controller.signal)).toEqual(points)
    expect(get).toHaveBeenCalledWith('/admin/operations/balance/history', { params: { months }, signal: controller.signal })
  })

  it('preserves empty results', async () => {
    get.mockResolvedValue({ data: [] })
    expect(await getOperationsBalanceHistory({ months: 3 })).toEqual([])
  })

  it('propagates request failures for the view to handle', async () => {
    const error = new Error('unavailable')
    get.mockRejectedValue(error)
    await expect(getOperationsBalanceHistory({ months: 6 })).rejects.toBe(error)
  })
})
