import type { ApplicationStatus, InvoiceType, TitleType } from './api'

export const statusLabels: Record<ApplicationStatus, string> = {
  pending: '待处理',
  processing: '开票中',
  issued: '已开具',
  rejected: '已驳回',
  cancelled: '已撤销',
  voided: '已作废',
}

/** Badge classes per status, from the shared badge styles. */
export const statusBadge: Record<ApplicationStatus, string> = {
  pending: 'badge badge-warning',
  processing: 'badge badge-primary',
  issued: 'badge badge-success',
  rejected: 'badge badge-danger',
  cancelled: 'badge badge-gray',
  voided: 'badge badge-gray',
}

export const invoiceTypeLabels: Record<InvoiceType, string> = {
  normal: '电子普通发票',
  special: '增值税专用发票',
}

export const titleTypeLabels: Record<TitleType, string> = {
  personal: '个人',
  company: '企业',
}

export const orderTypeLabels: Record<string, string> = {
  balance: '余额充值',
  subscription: '套餐购买',
}

export const paymentTypeLabels: Record<string, string> = {
  alipay: '支付宝',
  alipay_direct: '支付宝',
  wxpay: '微信支付',
  wxpay_direct: '微信支付',
  easypay: '聚合支付',
  stripe: 'Stripe',
  card: '银行卡',
  link: 'Link',
  airwallex: 'Airwallex',
  bepusdt: 'USDT',
}

/** Formats a decimal string or number as CNY with two places. */
export function formatMoney(value: string | number | null | undefined): string {
  const n = typeof value === 'number' ? value : Number(value ?? 0)
  if (!Number.isFinite(n)) return '¥0.00'
  return `¥${n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

/**
 * Sums decimal strings in cents so the selection total never shows float noise
 * such as 110.49999.
 */
export function sumCents(values: Array<string | number>): number {
  return values.reduce<number>((total, value) => total + Math.round(Number(value) * 100), 0)
}

export function centsToAmount(cents: number): string {
  return (cents / 100).toFixed(2)
}

export function formatTime(value: string | null | undefined): string {
  if (!value) return '—'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString('zh-CN', { hour12: false })
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return '—'
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? value : d.toLocaleDateString('zh-CN')
}

export function errorMessage(e: unknown, fallback: string): string {
  const m = (e as { message?: string })?.message
  return m && typeof m === 'string' ? m : fallback
}

/** Hides most of a bank account or taxpayer number in lists. */
export function maskTail(value: string, keep = 4): string {
  const v = value.replace(/\s+/g, '')
  if (v.length <= keep + 2) return v
  return `${v.slice(0, 2)}${'*'.repeat(Math.min(8, v.length - keep - 2))}${v.slice(-keep)}`
}
