import { apiClient } from '@/api/client'
import type { Account } from '@/types'

export async function updateSortOrder(
  updates: Array<{ id: number; sort_order: number }>
): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>('/admin/accounts/sort-order', {
    updates
  })
  return data
}

export async function getGroupAccounts(id: number): Promise<Account[]> {
  const { data } = await apiClient.get<Account[]>(`/admin/groups/${id}/accounts`)
  return data
}

export async function updateGroupAccounts(id: number, accountIds: number[]): Promise<Account[]> {
  const { data } = await apiClient.put<Account[]>(`/admin/groups/${id}/accounts`, {
    account_ids: accountIds
  })
  return data
}

export interface BatchUserActionSkipped {
  user_id: number
  reason: string
}

export interface BatchUserActionResponse {
  affected: number
  skipped: BatchUserActionSkipped[]
}

export async function batchDisable(userIds: number[]): Promise<BatchUserActionResponse> {
  const { data } = await apiClient.post<BatchUserActionResponse>('/admin/users/batch-disable', {
    user_ids: userIds
  })
  return data
}

export async function batchDelete(userIds: number[]): Promise<BatchUserActionResponse> {
  const { data } = await apiClient.post<BatchUserActionResponse>('/admin/users/batch-delete', {
    user_ids: userIds
  })
  return data
}

