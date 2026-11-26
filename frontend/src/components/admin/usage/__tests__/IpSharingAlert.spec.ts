import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import IpSharingAlert from '../IpSharingAlert.vue'

const { ipSharing } = vi.hoisted(() => ({
  ipSharing: vi.fn()
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: { ipSharing }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: { count?: number }) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

const mountAlert = () =>
  mount(IpSharingAlert, {
    props: { startDate: '2026-11-20', endDate: '2026-11-25' }
  })

describe('IpSharingAlert', () => {
  beforeEach(() => {
    ipSharing.mockReset()
  })

  it('loads shared IPs and emits select-ip', async () => {
    ipSharing.mockResolvedValue({
      items: [
        {
          ip_address: '203.0.113.7',
          user_count: 2,
          request_count: 12,
          last_seen: '2026-11-24T08:30:00Z',
          users: [
            { user_id: 1, email: 'a@example.com', username: 'a', requests: 8 },
            { user_id: 2, email: 'b@example.com', username: 'b', requests: 4 }
          ]
        }
      ]
    })

    const wrapper = mountAlert()
    await flushPromises()

    expect(ipSharing).toHaveBeenCalledWith(
      expect.objectContaining({
        start_date: '2026-11-20',
        end_date: '2026-11-25',
        min_accounts: 2
      })
    )
    expect(wrapper.get('[data-test="ip-sharing-alert"]').text()).toContain('admin.usage.ipRisk.summary:1')
    expect(wrapper.get('[data-test="ip-sharing-list"]').text()).toContain('203.0.113.7')
    expect(wrapper.get('[data-test="ip-sharing-list"]').text()).toContain('a@example.com')

    await wrapper.get('[data-test="ip-sharing-view"]').trigger('click')
    expect(wrapper.emitted('select-ip')).toEqual([['203.0.113.7']])
  })

  it('shows the empty state when nothing is shared', async () => {
    ipSharing.mockResolvedValue({ items: [] })

    const wrapper = mountAlert()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.usage.ipRisk.empty')
    expect(wrapper.find('[data-test="ip-sharing-list"]').exists()).toBe(false)
  })

  it('rechecks when refreshed', async () => {
    ipSharing.mockResolvedValue({ items: [] })

    const wrapper = mountAlert()
    await flushPromises()
    expect(ipSharing).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-test="ip-sharing-refresh"]').trigger('click')
    await flushPromises()
    expect(ipSharing).toHaveBeenCalledTimes(2)
  })
})
