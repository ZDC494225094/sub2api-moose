import { describe, expect, it, vi } from 'vitest'

import { formatLocalDate } from '../localDate'

describe('formatLocalDate', () => {
  it('uses local calendar fields instead of the UTC date', () => {
    const date = {
      getFullYear: vi.fn(() => 2026),
      getMonth: vi.fn(() => 6),
      getDate: vi.fn(() => 29),
      toISOString: vi.fn(() => '2026-07-28T16:30:00.000Z')
    } as unknown as Date

    expect(formatLocalDate(date)).toBe('2026-07-29')
    expect(date.toISOString).not.toHaveBeenCalled()
  })
})
