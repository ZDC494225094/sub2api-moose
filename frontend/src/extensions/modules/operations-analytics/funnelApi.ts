// Historical funnel/marketing contracts are owned by this extension, not dashboardAPI.
import { apiClient } from '@/api/client'
import type { PaginatedResponse } from '@/types'

export interface OperationsFunnelParams {
  start_date?: string
  end_date?: string
  timezone?: string
}

export interface OperationsFunnelStep {
  key: string
  label: string
  count: number
  conversion_rate: number
  overall_conversion_rate: number
  dropoff_from_previous: number
}

export interface OperationsRevenueSummary {
  total_revenue: number
  paid_orders: number
  paying_users: number
  average_order_amount: number
  balance_revenue: number
  balance_orders: number
  subscription_revenue: number
  subscription_orders: number
}

export interface OperationsOrderSignals {
  pending_orders: number
  failed_orders: number
  refund_requested_orders: number
  expired_orders: number
  cancelled_orders: number
}

export interface OperationsUserSummary {
  total_users: number
  active_users: number
  inactive_users: number
  active_rate: number
}

export interface OperationsCreditSummary {
  total_recharge_amount: number
  balance_recharge_amount: number
  subscription_recharge_amount: number
  total_remaining_amount: number
  balance_recharge_remaining: number
  subscription_remaining: number
  gifted_remaining: number
  remaining_balance: number
  gifted_amount: number
}

export interface OperationsSubscriptionSummary {
  active_subscriptions: number
  active_subscription_users: number
  limited_subscriptions: number
  daily_remaining_usd: number
  weekly_remaining_usd: number
  monthly_remaining_usd: number
}

export interface OperationsBreakdownItem {
  key: string
  label: string
  count: number
  amount: number
  percent: number
}

export interface OperationsSubscriptionQuotaBreakdown {
  key: string
  label: string
  limit_usd: number
  used_usd: number
  remaining_usd: number
  utilization_rate: number
}

export interface OperationsBreakdownSummary {
  users: OperationsBreakdownItem[]
  credits: OperationsBreakdownItem[]
  revenue: OperationsBreakdownItem[]
  subscriptions: OperationsSubscriptionQuotaBreakdown[]
}

export interface OperationsFunnelResponse {
  start_date: string
  end_date: string
  range_days: number
  generated_at: string
  steps: OperationsFunnelStep[]
  revenue: OperationsRevenueSummary
  signals: OperationsOrderSignals
  users: OperationsUserSummary
  credits: OperationsCreditSummary
  subscriptions: OperationsSubscriptionSummary
  breakdown?: OperationsBreakdownSummary
}

export type OperationsUserSegment = 'all' | 'active' | 'inactive' | 'balance' | 'recharge' | 'subscription'

export interface OperationsUserDetailsParams extends OperationsFunnelParams {
  segment?: OperationsUserSegment
  page?: number
  page_size?: number
}

export interface OperationsUserDetail {
  user_id: number
  email: string
  user_name?: string
  status: string
  balance: number
  total_recharged: number
  gifted_amount_estimate: number
  last_active_at?: string
  created_at: string
  period_requests: number
  period_usage_cost: number
  paid_order_count: number
  paid_order_amount: number
  balance_order_amount: number
  subscription_order_amount: number
  active_subscription_count: number
  subscription_daily_remaining_usd: number
  subscription_weekly_remaining_usd: number
  subscription_monthly_remaining_usd: number
}

export interface OperationsMarketingEmailRequest {
  subject: string
  body: string
  body_format?: 'plain' | 'html'
  audience?: 'all' | 'active' | 'inactive'
  status?: 'all' | 'active' | 'disabled'
  active_days?: number
  min_balance?: number
  max_balance?: number
  min_total_recharged?: number
  max_total_recharged?: number
  user_ids?: number[]
  limit?: number
  dry_run?: boolean
  confirm?: boolean
}

export interface OperationsMarketingEmailRecipient {
  user_id: number
  email: string
  user_name?: string
  status: string
  balance: number
  total_recharged: number
  last_active_at?: string
  created_at: string
}

export interface OperationsMarketingEmailResult {
  dry_run: boolean
  total_matched: number
  targeted: number
  sent: number
  failed: number
  skipped_invalid: number
  limit: number
  sample?: OperationsMarketingEmailRecipient[]
  errors?: string[]
}

export interface OperationsMarketingRecipientsParams {
  keyword?: string
  audience?: 'all' | 'active' | 'inactive'
  status?: 'all' | 'active' | 'disabled'
  active_days?: number
  min_balance?: number
  max_balance?: number
  min_total_recharged?: number
  max_total_recharged?: number
  page?: number
  page_size?: number
}

export interface OperationsMarketingEmailRecord {
  id: number
  subject: string
  body_format: 'plain' | 'html'
  body_preview: string
  audience: 'all' | 'active' | 'inactive'
  status: 'all' | 'active' | 'disabled'
  active_days: number
  min_balance?: number
  max_balance?: number
  min_total_recharged?: number
  max_total_recharged?: number
  selected_user_count: number
  total_matched: number
  targeted: number
  sent: number
  failed: number
  skipped_invalid: number
  errors?: string[]
  sample?: OperationsMarketingEmailRecipient[]
  created_at: string
}

export async function getOperationsFunnel(
  params?: OperationsFunnelParams
): Promise<OperationsFunnelResponse> {
  const { data } = await apiClient.get<OperationsFunnelResponse>('/admin/dashboard/operations-funnel', {
    params
  })
  return data
}

export async function getOperationsUserDetails(
  params?: OperationsUserDetailsParams
): Promise<PaginatedResponse<OperationsUserDetail>> {
  const { data } = await apiClient.get<PaginatedResponse<OperationsUserDetail>>('/admin/dashboard/operations-users', {
    params
  })
  return data
}

export async function getOperationsMarketingRecipients(
  params?: OperationsMarketingRecipientsParams
): Promise<PaginatedResponse<OperationsMarketingEmailRecipient>> {
  const { data } = await apiClient.get<PaginatedResponse<OperationsMarketingEmailRecipient>>(
    '/admin/dashboard/operations-marketing-recipients',
    { params }
  )
  return data
}

export async function getOperationsMarketingEmailRecords(params?: {
  page?: number
  page_size?: number
}): Promise<PaginatedResponse<OperationsMarketingEmailRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<OperationsMarketingEmailRecord>>(
    '/admin/dashboard/operations-marketing-email-records',
    { params }
  )
  return data
}

export async function sendOperationsMarketingEmail(
  payload: OperationsMarketingEmailRequest
): Promise<OperationsMarketingEmailResult> {
  const { data } = await apiClient.post<OperationsMarketingEmailResult>(
    '/admin/dashboard/operations-marketing-email',
    payload
  )
  return data
}

