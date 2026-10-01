import { computed } from 'vue'
import { useExtensionStore } from '../../store'
import type { BillingPriority } from '@/types'

export const nativeRouting = 'upstream-v1'

// multi: full multi-group UI. retain: an existing multi-group key while new
// configuration is switched off (keep, reduce, or return to balance-first).
// single: new key while off, or an upstream-owned key (ownership is immutable).
export type KeyRoutingMode = 'multi' | 'retain' | 'single'

interface RoutedKey {
  routing_policy?: string
  group_ids?: number[]
  group_id?: number | null
  billing_priority?: string
}

// Missing or failed switch state reads as "off": the UI never offers
// configuration that the server would reject. The server stays authoritative.
export function useMultiGroupAdmission() {
  const extensions = useExtensionStore()
  return computed(() => extensions.enabled('multi-group-billing'))
}

export function keyRoutingMode(enabled: boolean, key?: RoutedKey | null): KeyRoutingMode {
  if (key?.routing_policy === nativeRouting) return 'single'
  if (enabled) return 'multi'
  return key ? 'retain' : 'single'
}

export function originalGroupIds(key?: RoutedKey | null): number[] {
  if (!key) return []
  // Primary first, matching the server's NormalizeGroupIDs.
  const ids = key.group_id ? [key.group_id, ...(key.group_ids ?? [])] : [...(key.group_ids ?? [])]
  return [...new Set(ids.filter(id => Number.isFinite(id) && id > 0))]
}

// Whether another group may join `current` (which excludes groupId).
export function canAddGroup(mode: KeyRoutingMode, current: readonly number[], groupId: number, original: readonly number[]): boolean {
  if (mode === 'multi') return true
  if (current.length === 0) return true
  // Re-adding a group the key already had keeps its existing configuration.
  return mode === 'retain' && original.includes(groupId)
}

export function allowedBillingPriorities(mode: KeyRoutingMode, original?: string): BillingPriority[] {
  if (mode === 'multi' || (mode === 'retain' && original === 'subscription_first')) {
    return ['balance_first', 'subscription_first']
  }
  return ['balance_first']
}

export function keyRoutingNotice(mode: KeyRoutingMode, creating: boolean, locale: string): string {
  if (mode === 'multi') return ''
  const zh = locale.startsWith('zh')
  if (mode === 'retain') {
    return zh
      ? '多分组功能已关闭：此密钥现有的分组与计费优先级继续生效，可保留或缩减为单分组，但不能新增分组或改为订阅优先。'
      : 'Multi-group configuration is off: this key keeps its existing groups and billing priority. You may keep them or reduce to one group, but cannot add groups or switch to subscription-first.'
  }
  if (creating) {
    return zh
      ? '多分组功能已关闭：新建密钥使用单个分组并按余额优先计费。'
      : 'Multi-group configuration is off: new keys use a single group with balance-first billing.'
  }
  return zh
    ? '此密钥使用上游单分组路由，只能绑定一个分组；如需多分组，请新建密钥。'
    : 'This key uses upstream single-group routing. Create a new key for multi-group routing.'
}
