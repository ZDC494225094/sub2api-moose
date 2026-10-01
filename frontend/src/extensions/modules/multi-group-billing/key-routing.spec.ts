import { describe, expect, it } from 'vitest'
import { allowedBillingPriorities, canAddGroup, keyRoutingMode, keyRoutingNotice, originalGroupIds } from './key-routing'

describe('multi-group key routing modes', () => {
  const legacy = { routing_policy: 'multigroup-v1', group_id: 1, group_ids: [1, 2], billing_priority: 'subscription_first' }
  const native = { routing_policy: 'upstream-v1', group_id: 1, group_ids: [1] }

  it('derives the mode from key ownership first, then the switch', () => {
    expect(keyRoutingMode(true, null)).toBe('multi')
    expect(keyRoutingMode(false, null)).toBe('single')
    expect(keyRoutingMode(true, legacy)).toBe('multi')
    expect(keyRoutingMode(false, legacy)).toBe('retain')
    expect(keyRoutingMode(false, { group_ids: [1, 2] })).toBe('retain') // pre-ownership rows are legacy
    expect(keyRoutingMode(true, native)).toBe('single')
    expect(keyRoutingMode(false, native)).toBe('single')
  })

  it('only allows re-adding original groups while retaining', () => {
    const original = originalGroupIds(legacy)
    expect(original).toEqual([1, 2])
    expect(canAddGroup('retain', [1], 2, original)).toBe(true)
    expect(canAddGroup('retain', [1], 3, original)).toBe(false)
    expect(canAddGroup('single', [1], 2, original)).toBe(false)
    expect(canAddGroup('single', [], 2, original)).toBe(true)
    expect(canAddGroup('multi', [1, 2], 3, original)).toBe(true)
  })

  it('offers subscription-first only where the server admits it', () => {
    expect(allowedBillingPriorities('multi')).toEqual(['balance_first', 'subscription_first'])
    expect(allowedBillingPriorities('retain', 'subscription_first')).toEqual(['balance_first', 'subscription_first'])
    expect(allowedBillingPriorities('retain', 'balance_first')).toEqual(['balance_first'])
    expect(allowedBillingPriorities('single', 'subscription_first')).toEqual(['balance_first'])
  })

  it('normalizes original groups and explains restricted modes', () => {
    expect(originalGroupIds({ group_id: 3, group_ids: [2, 3, 2, 0, -1] })).toEqual([3, 2])
    expect(originalGroupIds(null)).toEqual([])
    expect(keyRoutingNotice('multi', true, 'zh-CN')).toBe('')
    expect(keyRoutingNotice('single', true, 'zh-CN')).toContain('新建密钥')
    expect(keyRoutingNotice('single', false, 'en')).toContain('upstream single-group')
    expect(keyRoutingNotice('retain', false, 'en')).toContain('existing groups')
  })
})
