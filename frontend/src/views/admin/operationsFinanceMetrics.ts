import type { PaymentDay } from '@/api/admin/operationsFinance'

export function cashMoney(value: number, currency = 'CNY') {
  const formatted = value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  return currency === 'CNY' ? `¥${formatted} 元` : `${currency} ${formatted}`
}

export function paymentTotals(rows: PaymentDay[], start?: string, end?: string) {
  const buckets = new Map<string, { currency: string; recharge_paid: number; subscription_paid: number }>()
  let credited = 0, pending = 0
  for (const row of rows) {
    if ((start && row.day < start) || (end && row.day > end)) continue
    const bucket = buckets.get(row.currency) ?? { currency: row.currency, recharge_paid: 0, subscription_paid: 0 }
    bucket.recharge_paid += row.recharge_paid
    bucket.subscription_paid += row.subscription_paid
    buckets.set(row.currency, bucket)
    credited += row.credited
    pending += row.pending_credit
  }
  return { currencies: [...buckets.values()].sort((a, b) => a.currency.localeCompare(b.currency)), credited, pending }
}
