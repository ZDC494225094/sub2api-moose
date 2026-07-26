/**
 * Payment System Type Definitions
 */

// ==================== Enums / Union Types ====================

export type OrderStatus =
  | 'PENDING'
  | 'PAID'
  | 'RECHARGING'
  | 'COMPLETED'
  | 'EXPIRED'
  | 'CANCELLED'
  | 'FAILED'
  | 'REFUND_REQUESTED'
  | 'REFUNDING'
  | 'REFUND_PENDING'
  | 'PARTIALLY_REFUNDED'
  | 'REFUNDED'
  | 'REFUND_FAILED'

export type PaymentType = 'alipay' | 'wxpay' | 'alipay_direct' | 'wxpay_direct' | 'stripe' | 'easypay' | 'airwallex'

export type OrderType = 'balance' | 'subscription'

// ==================== Configuration ====================

export interface PaymentConfig {
  payment_enabled: boolean
  min_amount: number
  max_amount: number
  daily_limit: number
  max_pending_orders: number
  order_timeout_minutes: number
  balance_disabled: boolean
  balance_recharge_multiplier: number
  subscription_usd_to_cny_rate: number
  enabled_payment_types: PaymentType[]
  help_image_url: string
  help_text: string
  stripe_publishable_key: string
}

export interface MethodLimit {
  currency?: string
  display_name?: string
  daily_limit: number
  daily_used: number
  daily_remaining: number
  single_min: number
  single_max: number
  fee_rate: number
  available: boolean
}

/** Response from /payment/limits API */
export interface MethodLimitsResponse {
  methods: Record<string, MethodLimit>
  global_min: number  // widest min across all methods; 0 = no minimum
  global_max: number  // widest max across all methods; 0 = no maximum
}

/** Response from /payment/checkout-info API — single call for the payment page */
export interface CheckoutInfoResponse {
  methods: Record<string, MethodLimit>
  global_min: number
  global_max: number
  plans: SubscriptionPlan[]
  balance_disabled: boolean
  balance_recharge_multiplier: number
  /** Subscription CNY conversion rate (1 USD = X CNY); 0 = disabled, plan price is charged as-is */
  subscription_usd_to_cny_rate: number
  recharge_fee_rate: number
  help_text: string
  help_image_url: string
  stripe_publishable_key: string
  /** When true, Alipay payments on mobile always show the QR code instead of redirecting */
  alipay_force_qrcode?: boolean
  /** When true, official Alipay mobile orders use precreate plus an Alipay app deep link */
  alipay_mobile_precreate_deep_link?: boolean
}

// ==================== Orders ====================

export interface PaymentOrder {
  id: number
  user_id: number
  amount: number
  pay_amount: number
  currency?: string
  fee_rate: number
  payment_type: string
  out_trade_no: string
  status: OrderStatus
  order_type: OrderType
  order_type_name?: string
  plan_name?: string
  created_at: string
  expires_at: string
  paid_at?: string
  completed_at?: string
  refund_amount: number
  refund_reason?: string
  refund_requested_at?: string
  refund_requested_by?: number
  refund_request_reason?: string
  plan_id?: number
  provider_instance_id?: string
}

// ==================== Plans & Channels ====================

export interface SubscriptionPlan {
  id: number
  group_id: number
  group_platform?: string
  group_name?: string
  rate_multiplier?: number
  peak_rate_enabled?: boolean
  peak_start?: string
  peak_end?: string
  peak_rate_multiplier?: number
  daily_limit_usd?: number | null
  weekly_limit_usd?: number | null
  monthly_limit_usd?: number | null
  supported_model_scopes?: string[]
  name: string
  description: string
  price: number
  original_price?: number
  display_purchase_count?: number
  /** Display-only ISO 4217 currency label (e.g. "NZD"); empty means no label */
  currency?: string
  validity_days: number
  validity_unit: string
  /** Stored as JSON string in backend; API layer should parse before use */
  features: string[]
  for_sale: boolean
  sort_order: number
  discount_rate?: number
  purchase_count?: number
}

export interface PaymentChannel {
  id: number
  group_id?: number
  name: string
  platform: string
  rate_multiplier: number
  description: string
  models: string[]
  features: string[]
  enabled: boolean
}

// ==================== Providers ====================

export interface ProviderInstance {
  id: number
  provider_key: string
  name: string
  config: Record<string, string>
  supported_types: string[]
  enabled: boolean
  payment_mode: string
  refund_enabled: boolean
  allow_user_refund: boolean
  limits: string
  sort_order: number
}

// ==================== Request / Response ====================

export interface CreateOrderRequest {
  amount: number
  payment_type: string
  order_type: string
  plan_id?: number
  user_coupon_id?: number
  return_url?: string
  payment_source?: string
  openid?: string
  wechat_resume_token?: string
  is_mobile?: boolean
}

export type CreateOrderResultType = 'order_created' | 'oauth_required' | 'jsapi_ready'

export interface WechatOAuthInfo {
  authorize_url?: string
  appid?: string
  openid?: string
  scope?: string
  state?: string
  redirect_url?: string
}

export interface WechatJSAPIPayload {
  appId?: string
  timeStamp?: string
  nonceStr?: string
  package?: string
  signType?: string
  paySign?: string
}

export interface CreateOrderResult {
  order_id: number
  amount: number
  pay_url?: string
  qr_code?: string
  client_secret?: string
  intent_id?: string
  currency?: string
  country_code?: string
  payment_env?: string
  pay_amount: number
  fee_rate: number
  expires_at: string
  result_type?: CreateOrderResultType
  payment_type?: string
  out_trade_no?: string
  payment_mode?: string
  resume_token?: string
  alipay_mobile_precreate_deep_link?: boolean
  oauth?: WechatOAuthInfo
  jsapi?: WechatJSAPIPayload
  jsapi_payload?: WechatJSAPIPayload
}

export interface DailyStatPoint {
  date: string
  amount: number
  count: number
  balance_amount: number
  balance_count: number
  subscription_amount: number
  subscription_count: number
  new_user_count: number
  new_user_amount: number
  returning_user_count: number
  returning_user_amount: number
}

export interface DashboardStats {
  today_amount: number
  total_amount: number
  today_count: number
  total_count: number
  avg_amount: number
  pending_orders: number
  daily_series: DailyStatPoint[]
  payment_methods: { type: string; amount: number; count: number }[]
  top_users: { user_id: number; email: string; amount: number }[]
}

export type CouponScope = 'balance' | 'subscription' | 'universal'

export interface CouponTemplate {
  id: number
  name: string
  description: string
  scope: CouponScope
  discount_amount: number
  threshold_amount: number
  valid_days?: number | null
  valid_from?: string | null
  valid_until?: string | null
  status: 'active' | 'disabled'
  notes: string
  created_at: string
  updated_at: string
}

export interface UserCoupon {
  id: number
  template_id: number
  user_id: number
  coupon_code: string
  source_type: string
  scope: CouponScope
  discount_amount: number
  threshold_amount: number
  valid_from?: string | null
  valid_until?: string | null
  status: 'unused' | 'reserved' | 'used' | 'expired' | 'disabled'
  created_at: string
  updated_at: string
}

export interface LotteryActivity {
  id: number
  name: string
  description: string
  status: 'draft' | 'active' | 'inactive' | 'ended'
  default_draw_times: number
  consume_threshold_amount: number
  wallet_cost_per_draw: number
  starts_at?: string | null
  ends_at?: string | null
  sort_order: number
  created_at: string
  updated_at: string
  prizes?: LotteryPrize[]
}

export interface LotteryPrize {
  id: number
  activity_id: number
  name: string
  prize_type: 'balance_redeem' | 'coupon' | 'thanks'
  stock: number
  remaining_stock: number
  balance_amount?: number | null
  coupon_template_id?: number | null
  coupon_template?: CouponTemplate | null
  display_order: number
  status: 'active' | 'inactive'
  created_at: string
  updated_at: string
}

export interface LotteryDrawResult {
  activity?: LotteryActivity
  prize?: LotteryPrize
  user_state?: {
    activity_id: number
    user_id: number
    default_granted: boolean
    available_draw_times: number
    total_granted_times: number
    total_drawn_times: number
    total_wallet_paid_amount: number
  }
  record?: {
    id: number
    activity_id: number
    user_id: number
    prize_name: string
    prize_type: 'balance_redeem' | 'coupon' | 'thanks'
    result_code: 'win' | 'thanks'
    chance_source: 'default' | 'wallet'
    wallet_amount: number
    reward_reference: string
    created_at: string
  }
  user_coupon?: UserCoupon
  redeem_code?: {
    id: number
    code: string
    type: string
    value: number
    status: string
    created_at?: string
  }
}

export interface LotteryDrawRecord {
  id: number
  activity_id: number
  user_id: number
  prize_id?: number
  prize_name: string
  prize_type: 'balance_redeem' | 'coupon' | 'thanks'
  result_code: 'win' | 'thanks'
  chance_source: 'default' | 'wallet'
  wallet_amount: number
  reward_reference: string
  created_at: string
  user_name?: string
  user_email?: string
}

export interface LotteryOverview {
  activity?: LotteryActivity
  user_state?: {
    activity_id: number
    user_id: number
    default_granted: boolean
    available_draw_times: number
    total_granted_times: number
    total_drawn_times: number
    total_wallet_paid_amount: number
  }
  draw_eligibility?: {
    consume_threshold_met: boolean
    qualified_amount: number
    required_threshold_amount: number
    wallet_draw_enabled: boolean
    can_draw_with_wallet: boolean
    block_reason?: string
  }
  recent_winners?: Array<{
    id: number
    activity_id: number
    user_id: number
    prize_name: string
    prize_type: 'balance_redeem' | 'coupon' | 'thanks'
    result_code: 'win' | 'thanks'
    reward_reference: string
    created_at: string
    user_name?: string
    user_email?: string
  }>
}
