import { apiClient } from './client'

export interface RechargeCampaign {
  revision?: string
  id: number
  name: string
  description: string
  enabled: boolean
  starts_at: string
  ends_at: string
  kind: 'bonus' | 'discount'
  percent: number
  min_amount: number
  reward_percent: number
  reward_cap: number
  freeze_hours: number
  new_invitees_only: boolean
}
export const campaignAPI = {
  list: () => apiClient.get<RechargeCampaign[]>('/admin/payment/campaigns'),
  publicList: () => apiClient.get<RechargeCampaign[]>('/payment/public/campaigns'),
  save: (data: RechargeCampaign) => data.id
    ? apiClient.put<RechargeCampaign>(`/admin/payment/campaigns/${data.id}`, data)
    : apiClient.post<RechargeCampaign>('/admin/payment/campaigns', data),
}
export function campaignHeadline(a: RechargeCampaign): string {
  return a.kind === 'bonus' ? `充值加赠 ${a.percent}%` : `充值享 ${Number((a.percent / 10).toFixed(2))} 折`
}
export function campaignStatus(a: RechargeCampaign, now = Date.now()): string {
  if (!a.enabled) return '已停用'
  if (now < Date.parse(a.starts_at)) return '未开始'
  if (now >= Date.parse(a.ends_at)) return '已结束'
  return '进行中'
}
export function campaignAmounts(a: RechargeCampaign | null, amount: number, multiplier: number) {
  const round = (v: number) => Math.round((v + Number.EPSILON) * 100) / 100
  return {
    principal: round(amount * (a?.kind === 'discount' ? a.percent / 100 : 1)),
    credited: round(amount * multiplier * (a?.kind === 'bonus' ? 1 + a.percent / 100 : 1)),
  }
}

export function automaticCampaign(items: RechargeCampaign[], amount: number, now = Date.now()): RechargeCampaign | null {
  if (amount <= 0) return null
  const benefit = (a: RechargeCampaign) => a.kind === 'bonus' ? 1 + a.percent / 100 : 100 / a.percent
  return items.filter(a => campaignStatus(a, now) === '进行中' && amount >= a.min_amount)
    .sort((a, b) => Math.abs(benefit(b) - benefit(a)) < 1e-12 ? b.id - a.id : benefit(b) - benefit(a))[0] ?? null
}
