export interface AccountUpstreamGroup {
  id: number
  key: string
  name: string
  account_count: number
  sort_order: number
}

// The host retains generic account updates, authorization and per-account results.
export type BulkUpstreamGroupUpdater = (
 accountIds: number[], update: { upstream_group: string }
) => Promise<{ success?: number; failed?: number }>
