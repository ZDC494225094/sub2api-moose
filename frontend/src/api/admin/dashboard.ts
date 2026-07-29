/**
 * Admin Dashboard API endpoints
 * Provides system-wide statistics and metrics
 */

import { apiClient } from '../client'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  GroupStat,
  ApiKeyUsageTrendPoint,
  UserUsageTrendPoint,
  UserSpendingRankingResponse,
  UserBreakdownItem,
  UsageRequestType,
  PaginatedResponse
} from '@/types'

/**
 * Get dashboard statistics
 * @returns Dashboard statistics including users, keys, accounts, and token usage
 */
export async function getStats(): Promise<DashboardStats> {
  const { data } = await apiClient.get<DashboardStats>('/admin/dashboard/stats')
  return data
}

/**
 * Get real-time metrics
 * @returns Real-time system metrics
 */
export async function getRealtimeMetrics(): Promise<{
  active_requests: number
  requests_per_minute: number
  average_response_time: number
  error_rate: number
}> {
  const { data } = await apiClient.get<{
    active_requests: number
    requests_per_minute: number
    average_response_time: number
    error_rate: number
  }>('/admin/dashboard/realtime')
  return data
}

export interface TrendParams {
  start_date?: string
  end_date?: string
  granularity?: 'day' | 'hour'
  user_id?: number
  api_key_id?: number
  model?: string
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface TrendResponse {
  trend: TrendDataPoint[]
  start_date: string
  end_date: string
  granularity: string
}

/**
 * Get usage trend data
 * @param params - Query parameters for filtering
 * @returns Usage trend data
 */
export async function getUsageTrend(params?: TrendParams): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>('/admin/dashboard/trend', { params })
  return data
}

export interface ModelStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface ModelStatsResponse {
  models: ModelStat[]
  start_date: string
  end_date: string
}

/**
 * Get model usage statistics
 * @param params - Query parameters for filtering
 * @returns Model usage statistics
 */
export async function getModelStats(params?: ModelStatsParams): Promise<ModelStatsResponse> {
  const { data } = await apiClient.get<ModelStatsResponse>('/admin/dashboard/models', { params })
  return data
}

export interface GroupStatsParams {
  start_date?: string
  end_date?: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface GroupStatsResponse {
  groups: GroupStat[]
  start_date: string
  end_date: string
}

export interface DashboardSnapshotV2Params extends TrendParams {
  include_stats?: boolean
  include_trend?: boolean
  include_model_stats?: boolean
  include_group_stats?: boolean
  include_users_trend?: boolean
  users_trend_limit?: number
}

export interface DashboardSnapshotV2Stats extends DashboardStats {
  uptime: number
}

export interface DashboardSnapshotV2Response {
  generated_at: string
  start_date: string
  end_date: string
  granularity: string
  stats?: DashboardSnapshotV2Stats
  trend?: TrendDataPoint[]
  models?: ModelStat[]
  groups?: GroupStat[]
  users_trend?: UserUsageTrendPoint[]
}

/**
 * Get group usage statistics
 * @param params - Query parameters for filtering
 * @returns Group usage statistics
 */
export async function getGroupStats(params?: GroupStatsParams): Promise<GroupStatsResponse> {
  const { data } = await apiClient.get<GroupStatsResponse>('/admin/dashboard/groups', { params })
  return data
}

export interface UserBreakdownParams {
  start_date?: string
  end_date?: string
  group_id?: number
  model?: string
  model_source?: 'requested' | 'upstream' | 'mapping'
  endpoint?: string
  endpoint_type?: 'inbound' | 'upstream' | 'path'
  limit?: number
  // Sort column for the ranking (allowlisted server-side; falls back to actual_cost)
  sort_by?: 'total_tokens' | 'input_tokens' | 'output_tokens' | 'cache_tokens' | 'requests' | 'cost' | 'actual_cost'
  // Additional filter conditions
  user_id?: number
  api_key_id?: number
  account_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
}

export interface UserBreakdownResponse {
  users: UserBreakdownItem[]
  start_date: string
  end_date: string
}

export async function getUserBreakdown(params: UserBreakdownParams): Promise<UserBreakdownResponse> {
  const { data } = await apiClient.get<UserBreakdownResponse>('/admin/dashboard/user-breakdown', {
    params
  })
  return data
}

/**
 * Get dashboard snapshot v2 (aggregated response for heavy admin pages).
 */
export async function getSnapshotV2(params?: DashboardSnapshotV2Params): Promise<DashboardSnapshotV2Response> {
  const { data } = await apiClient.get<DashboardSnapshotV2Response>('/admin/dashboard/snapshot-v2', {
    params
  })
  return data
}

export interface ApiKeyTrendParams extends TrendParams {
  limit?: number
}

export interface ApiKeyTrendResponse {
  trend: ApiKeyUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

/**
 * Get API key usage trend data
 * @param params - Query parameters for filtering
 * @returns API key usage trend data
 */
export async function getApiKeyUsageTrend(
  params?: ApiKeyTrendParams
): Promise<ApiKeyTrendResponse> {
  const { data } = await apiClient.get<ApiKeyTrendResponse>('/admin/dashboard/api-keys-trend', {
    params
  })
  return data
}

export interface UserTrendParams extends TrendParams {
  limit?: number
}

export interface UserTrendResponse {
  trend: UserUsageTrendPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface UserSpendingRankingParams
  extends Pick<TrendParams, 'start_date' | 'end_date'> {
  limit?: number
}

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

/**
 * Get user usage trend data
 * @param params - Query parameters for filtering
 * @returns User usage trend data
 */
export async function getUserUsageTrend(params?: UserTrendParams): Promise<UserTrendResponse> {
  const { data } = await apiClient.get<UserTrendResponse>('/admin/dashboard/users-trend', {
    params
  })
  return data
}

/**
 * Get user spending ranking data
 * @param params - Query parameters for filtering
 * @returns User spending ranking data
 */
export async function getUserSpendingRanking(
  params?: UserSpendingRankingParams
): Promise<UserSpendingRankingResponse> {
  const { data } = await apiClient.get<UserSpendingRankingResponse>('/admin/dashboard/users-ranking', {
    params
  })
  return data
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

export interface PlatformUsage {
  platform: string
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchUserUsageStats {
  user_id: number
  today_actual_cost: number
  total_actual_cost: number
  by_platform?: PlatformUsage[]
}

export interface BatchUsersUsageResponse {
  stats: Record<string, BatchUserUsageStats>
}

/**
 * Get batch usage stats for multiple users
 * @param userIds - Array of user IDs
 * @returns Usage stats map keyed by user ID
 */
export async function getBatchUsersUsage(userIds: number[]): Promise<BatchUsersUsageResponse> {
  const { data } = await apiClient.post<BatchUsersUsageResponse>('/admin/dashboard/users-usage', {
    user_ids: userIds
  })
  return data
}

export interface BatchApiKeyUsageStats {
  api_key_id: number
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchApiKeysUsageResponse {
  stats: Record<string, BatchApiKeyUsageStats>
}

/**
 * Get batch usage stats for multiple API keys
 * @param apiKeyIds - Array of API key IDs
 * @returns Usage stats map keyed by API key ID
 */
export async function getBatchApiKeysUsage(
  apiKeyIds: number[]
): Promise<BatchApiKeysUsageResponse> {
  const { data } = await apiClient.post<BatchApiKeysUsageResponse>(
    '/admin/dashboard/api-keys-usage',
    {
      api_key_ids: apiKeyIds
    }
  )
  return data
}

export const dashboardAPI = {
  getStats,
  getRealtimeMetrics,
  getUsageTrend,
  getModelStats,
  getGroupStats,
  getSnapshotV2,
  getApiKeyUsageTrend,
  getUserUsageTrend,
  getUserSpendingRanking,
  getOperationsFunnel,
  getOperationsUserDetails,
  getOperationsMarketingRecipients,
  getOperationsMarketingEmailRecords,
  sendOperationsMarketingEmail,
  getBatchUsersUsage,
  getBatchApiKeysUsage
}

export default dashboardAPI
