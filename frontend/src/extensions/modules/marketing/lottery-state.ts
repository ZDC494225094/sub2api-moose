import type { LotteryOverview } from '@/types/payment'

/** Pending grants are previews, never persisted rights. Draw returns authoritative state. */
export function availableLotteryChances(overview: LotteryOverview | null): number {
  const state = overview?.user_state
  const granted = state?.available_draw_times ?? 0
  const pending = state?.default_granted ? 0 : (overview?.draw_eligibility?.pending_default_draw_times ?? 0)
  return granted + pending
}
