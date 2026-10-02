import { beforeEach, describe, expect, it, vi } from 'vitest'
const { get, patch, put } = vi.hoisted(() => ({ get: vi.fn(), patch: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, patch, put } }))
import * as api from './upstreamGroupsApi'
import * as host from '@/api/admin/accounts'

describe('upstream group API contract and host compatibility', () => {
  beforeEach(() => vi.resetAllMocks())
  it('keeps host exports identical to the module functions', () => {
    expect(host.listUpstreamGroups).toBe(api.listUpstreamGroups)
    expect(host.renameUpstreamGroup).toBe(api.renameUpstreamGroup)
    expect(host.updateUpstreamGroupSortOrders).toBe(api.updateUpstreamGroupSortOrders)
  })
  it('retains empty persistent groups in the response', async () => {
    const groups = [{ id: 9, key: 'edge', name: 'Edge', account_count: 0, sort_order: 0 }]
    get.mockResolvedValueOnce({ data: { groups } })
    await expect(api.listUpstreamGroups()).resolves.toEqual(groups)
    expect(get).toHaveBeenCalledWith('/admin/accounts/upstream-groups')
  })
  it('preserves rename and sort request/response envelopes', async () => {
    const group = { id: 9, name: 'Edge' }
    patch.mockResolvedValueOnce({ data: group })
    await expect(api.renameUpstreamGroup(9, 'Edge')).resolves.toEqual(group)
    expect(patch).toHaveBeenCalledWith('/admin/accounts/upstream-groups/9', { name: 'Edge' })
    const updates = [{ id: 9, sort_order: 0 }]
    put.mockResolvedValueOnce({ data: { message: 'ok' } })
    await expect(api.updateUpstreamGroupSortOrders(updates)).resolves.toEqual({ message: 'ok' })
    expect(put).toHaveBeenCalledWith('/admin/accounts/upstream-groups/sort-order', { updates })
  })
  it('propagates host authorization and transaction errors without fallback', async () => {
    const error = new Error('forbidden')
    get.mockRejectedValueOnce(error)
    patch.mockRejectedValueOnce(error)
    put.mockRejectedValueOnce(error)
    await expect(api.listUpstreamGroups()).rejects.toBe(error)
    await expect(api.renameUpstreamGroup(9, 'Edge')).rejects.toBe(error)
    await expect(api.updateUpstreamGroupSortOrders([{ id: 9, sort_order: 0 }])).rejects.toBe(error)
  })
})
