import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import GroupBillingRateSyncFields from '../GroupBillingRateSyncFields.vue'
import type { Account } from '@/types'

const listAccountsMock = vi.fn()
const getAccountMock = vi.fn()

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: (...args: unknown[]) => listAccountsMock(...args),
      getById: (...args: unknown[]) => getAccountMock(...args)
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (key === 'admin.groups.billingRateSync.preview') {
        return `最近探测 ${params?.detected} + 加价 ${params?.markup} = 最终倍率 ${params?.final}`
      }
      return key
    }
  })
}))

const account = {
  id: 17,
  name: 'reference-account',
  platform: 'openai',
  type: 'apikey',
  status: 'active',
  extra: {
    upstream_billing_probe: {
      status: 'ok',
      data: { effective_rate_multiplier: 0.2 },
      last_attempt_at: '2026-07-21T00:00:00Z',
      next_probe_at: '2026-07-21T00:05:00Z'
    }
  }
} as Account

describe('GroupBillingRateSyncFields', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listAccountsMock.mockResolvedValue({ items: [account], total: 1, page: 1, page_size: 100 })
    getAccountMock.mockResolvedValue(account)
  })

  it('loads only OpenAI API key accounts and previews additive markup', async () => {
    const wrapper = mount(GroupBillingRateSyncFields, {
      props: { accountId: 17, markup: 0.1 }
    })
    await flushPromises()

    expect(listAccountsMock).toHaveBeenCalledWith(1, 100, expect.objectContaining({
      platform: 'openai',
      type: 'apikey'
    }))
    expect(wrapper.get('[data-testid="group-billing-rate-preview"]').text()).toContain(
      '0.2 + 加价 0.1 = 最终倍率 0.3'
    )

    await wrapper.get('[data-testid="group-billing-rate-markup"]').setValue('0.15')
    expect(wrapper.emitted('update:markup')?.at(-1)).toEqual([0.15])
  })
})
