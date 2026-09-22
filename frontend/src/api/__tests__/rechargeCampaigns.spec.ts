import { describe, expect, it } from 'vitest'
import { automaticCampaign, campaignAmounts, campaignStatus, type RechargeCampaign } from '../rechargeCampaigns'

const campaign: RechargeCampaign = { id:1,name:'充值礼',description:'',enabled:true,starts_at:'2026-09-20T00:00:00Z',ends_at:'2026-09-21T00:00:00Z',kind:'bonus',percent:10,min_amount:0,reward_percent:5,reward_cap:10,freeze_hours:72,new_invitees_only:true }
describe('recharge campaigns', () => {
  it('automatically selects the best eligible return and uses newest id to break ties', () => {
    const now = Date.parse(campaign.starts_at)
    const discount: RechargeCampaign = { ...campaign, id: 2, kind: 'discount', percent: 90, min_amount: 100 }
    expect(automaticCampaign([discount, campaign], 50, now)?.id).toBe(1)
    expect(automaticCampaign([campaign, discount], 100, now)?.id).toBe(2)
    expect(automaticCampaign([campaign, { ...campaign, id: 3 }], 50, now)?.id).toBe(3)
    expect(automaticCampaign([campaign], 50, Date.parse(campaign.ends_at))).toBeNull()
    expect(automaticCampaign([campaign], 0, now)).toBeNull()
  })
  it('credits the requested bonus examples', () => {
    expect(campaignAmounts(campaign,50,1)).toEqual({principal:50,credited:55})
    expect(campaignAmounts(campaign,100,1)).toEqual({principal:100,credited:110})
  })
  it('discounts the payment without discounting the credited balance', () => {
    expect(campaignAmounts({...campaign,kind:'discount',percent:90},100,1)).toEqual({principal:90,credited:100})
  })
  it('preserves the configured currency conversion multiplier', () => {
    expect(campaignAmounts(campaign,50,2)).toEqual({principal:50,credited:110})
  })
  it('includes the start instant and excludes the end instant', () => {
    expect(campaignStatus(campaign,Date.parse(campaign.starts_at))).toBe('进行中')
    expect(campaignStatus(campaign,Date.parse(campaign.ends_at))).toBe('已结束')
    expect(campaignStatus({...campaign,enabled:false},Date.parse(campaign.starts_at))).toBe('已停用')
  })
})
