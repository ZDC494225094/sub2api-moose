import { apiClient } from '@/api/client'
import type { RedeemCode } from '@/types'

export interface MarketingBatch {
  batch_id: string
  created_at: string
  registered_after: string | null
  code_count: number
  redeemed_users: number
  redeemed_value: number
  paying_users: number
  order_count: number
  pay_amounts: Record<string, number>
}

export const redeemMarketingAPI = {
  async generate(count: number, value: number, expiresInDays: number | null | undefined, newUsersOnly: boolean) {
    const { data } = await apiClient.post<RedeemCode[]>('/admin/redeem-codes/marketing-batches/generate', {
      type: 'marketing', count, value, expires_in_days: expiresInDays || undefined, new_users_only: newUsersOnly
    })
    return data
  },
  async batches(batchId: string, page: number) {
    const { data } = await apiClient.get<{ items: MarketingBatch[] }>('/admin/redeem-codes/marketing-batches', {
      params: { batch_id: batchId, page, page_size: 20 }
    })
    return data.items
  }
}
