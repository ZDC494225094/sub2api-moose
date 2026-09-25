import { apiClient } from '../client'

export interface FinanceRow {
  dimension: 'day' | 'upstream' | 'account' | 'model'
  key: string
  label: string
  upstream: string
  total_orders: number
  paid_orders: number
  excluded_recharge: number
  requests: number
  consumption: number
  list_cost: number
  cost: number
  recharge: number
  subscription: number
  profit: number
  margin: number | null
}

export interface FinanceReport {
  start_date: string
  end_date: string
  generated_at: string
  timezone: string
  current_balance: number
  summary: FinanceRow
  rows: FinanceRow[]
}

export async function getOperationsFinance(params: { start_date?: string; end_date?: string; preset?: string }, signal?: AbortSignal) {
  const { data } = await apiClient.get<FinanceReport>('/admin/dashboard/operations-finance', { params, signal })
  return data
}

export type CustomerSegment = 'all' | 'balance' | 'paying' | 'repeat' | 'new_paying' | 'active' | 'churned'
export interface CustomerSummary {
  total: number; paying: number; repeat: number; new_paying: number; active: number
  churned: number; previous_active: number; balance_users: number
  repeat_rate: number | null; churn_rate: number | null
}
export interface OperationsCustomer {
  id: number; email: string; username: string; balance: number
  period_orders: number; total_orders: number; period_amount: number
  consumption: number; cost: number; profit: number; margin: number | null
  last_used_at: string | null; last_paid_at: string | null; repeat: boolean; churned: boolean
}
export interface CustomersReport {
  summary: CustomerSummary; items: OperationsCustomer[]; total: number; as_of: string; churn_days: number
}
export async function getOperationsCustomers(params: {
  start_date: string; end_date: string; timezone: string; segment: CustomerSegment
  churn_days: number; page: number; search: string
}, signal?: AbortSignal) {
  const { data } = await apiClient.get<CustomersReport>('/admin/dashboard/operations-customers', { params, signal })
  return data
}
