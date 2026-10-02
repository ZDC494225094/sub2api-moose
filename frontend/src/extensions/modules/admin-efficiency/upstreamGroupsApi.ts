import { apiClient } from '@/api/client'
import type { AccountUpstreamGroup } from './types'

/** List all distinct explicit upstream groups. */
export async function listUpstreamGroups(): Promise<AccountUpstreamGroup[]> {
  const { data } = await apiClient.get<{ groups: AccountUpstreamGroup[] }>('/admin/accounts/upstream-groups')
  return data.groups
}

/** Rename an explicit upstream group. */
export async function renameUpstreamGroup(id: number, name: string): Promise<AccountUpstreamGroup> {
  const { data } = await apiClient.patch<AccountUpstreamGroup>(`/admin/accounts/upstream-groups/${id}`, { name })
  return data
}

/** Persist the custom display order for upstream groups. */
export async function updateUpstreamGroupSortOrders(
  updates: Array<{ id: number; sort_order: number }>
): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>('/admin/accounts/upstream-groups/sort-order', {
    updates
  })
  return data
}

