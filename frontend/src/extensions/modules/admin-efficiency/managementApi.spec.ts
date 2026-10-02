import { beforeEach, describe, expect, it, vi } from 'vitest'
const { get, put, post } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, put, post } }))
import * as api from './managementApi'
import * as accounts from '@/api/admin/accounts'
import * as groups from '@/api/admin/groups'
import * as users from '@/api/admin/users'

describe('admin management API contracts', () => {
  beforeEach(() => vi.resetAllMocks())
  it('retains host facade identity', () => {
    expect(accounts.updateSortOrder).toBe(api.updateSortOrder)
    expect(groups.getGroupAccounts).toBe(api.getGroupAccounts)
    expect(groups.updateGroupAccounts).toBe(api.updateGroupAccounts)
    expect(users.batchDisable).toBe(api.batchDisable)
    expect(users.batchDelete).toBe(api.batchDelete)
  })
  it('retains sort and membership URLs and envelopes', async () => {
    put.mockResolvedValue({ data: [] })
    get.mockResolvedValue({ data: [{ id: 2 }] })
    const updates = [{ id: 2, sort_order: 10 }]
    await api.updateSortOrder(updates)
    expect(put).toHaveBeenLastCalledWith('/admin/accounts/sort-order', { updates })
    await expect(api.getGroupAccounts(7)).resolves.toEqual([{ id: 2 }])
    expect(get).toHaveBeenCalledWith('/admin/groups/7/accounts')
    await api.updateGroupAccounts(7, [2, 1])
    expect(put).toHaveBeenLastCalledWith('/admin/groups/7/accounts', { account_ids: [2, 1] })
  })
  it.each(['batchDisable', 'batchDelete'] as const)('uses the gated batch endpoint for %s and preserves failures', async action => {
    const result = { affected: 1, skipped: [{ user_id: 2, reason: 'protected' }] }
    post.mockResolvedValueOnce({ data: result })
    await expect(api[action]([1, 2])).resolves.toEqual(result)
    expect(post).toHaveBeenCalledWith(`/admin/users/${action === 'batchDelete' ? 'batch-delete' : 'batch-disable'}`, { user_ids: [1, 2] })
    const denied = new Error('CUSTOM_EXTENSION_DISABLED')
    post.mockRejectedValueOnce(denied)
    await expect(api[action]([1])).rejects.toBe(denied)
    expect(post).toHaveBeenCalledTimes(2)
  })
})
