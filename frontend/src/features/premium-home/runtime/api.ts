import { apiClient } from '@/api/client'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserAnnouncement } from '@/types'

export async function getPublicPlans(): Promise<SubscriptionPlan[]> {
  const response = await apiClient.get<SubscriptionPlan[]>('/payment/public/plans')
  return response.data
}

export async function getPublicAnnouncements(): Promise<UserAnnouncement[]> {
  const response = await apiClient.get<UserAnnouncement[]>('/announcements/public')
  return response.data
}
