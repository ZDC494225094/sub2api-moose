import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OperationsFinanceView from '../OperationsFinanceView.vue'

const { getReport, push, replace } = vi.hoisted(() => ({ getReport: vi.fn(), push: vi.fn(), replace: vi.fn() }))
vi.mock('@/api/admin/operationsFinance', () => ({ getOperationsFinance: getReport }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push, replace }), useRoute: () => ({ query: {} }) }))
vi.mock('vue-chartjs', () => ({ Line: { template: '<canvas />' }, Bar: { template: '<canvas />' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('../components/OperationsCustomers.vue', () => ({ default: { template: '<section />', methods: { open() {} } } }))
const row = { key: '7', label: 'Account A', upstream: 'Vendor A', requests: 3, consumption: 10, cost: 12, list_cost: 20, recharge: 30, subscription: 0, profit: -2, margin: -20 }
const data = { start_date: '2026-09-01', end_date: '2026-09-14', generated_at: '2026-09-14T10:00:00Z', current_balance: 50, summary: row, rows: [{ ...row, dimension: 'day', key: '2026-09-01' }, { ...row, dimension: 'upstream', key: 'Vendor A', label: 'Vendor A' }, { ...row, dimension: 'account' }] }
const create = () => mount(OperationsFinanceView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
describe('Operations finance', () => {
  beforeEach(() => { vi.clearAllMocks(); getReport.mockResolvedValue(structuredClone(data)) })
  it('uses the server date despite a changed local clock', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2040-01-01T00:00:00Z'))
    try {
      const wrapper = create(); await flushPromises()
      expect(getReport.mock.calls[0][0]).toEqual({ preset: 'today' })
      expect(wrapper.findAll('input[type="date"]')[0].element.value).toBe(data.start_date)
      await wrapper.findAll('button').find(button => button.text() === '昨天')!.trigger('click')
      await flushPromises()
      expect(getReport.mock.lastCall?.[0]).toEqual({ preset: 'yesterday' })
      wrapper.unmount()
    } finally { vi.useRealTimers() }
  })
  it('drills from upstream to account to date-filtered requests', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    await wrapper.find('tbody button').trigger('click')
    expect(wrapper.text()).toContain('上游：Vendor A')
    await wrapper.find('tbody button').trigger('click')
    expect(push).toHaveBeenCalledWith({ path: '/admin/usage', query: { start_date: data.start_date, end_date: data.end_date, timezone: 'Asia/Shanghai', account_id: '7' } })
    wrapper.unmount()
  })
  it('opens all recharge orders for the selected created-at range', async () => {
    const wrapper = create(); await flushPromises()
    const rechargeMetric = wrapper.findAll('button').find(button => button.text().includes('期间充值额度'))
    expect(rechargeMetric).toBeDefined()
    await rechargeMetric!.trigger('click')
    expect(push).toHaveBeenCalledWith({ path: '/admin/orders', query: { date_field: 'created_at', start_date: data.start_date, end_date: data.end_date } })
    wrapper.unmount()
  })
  it('rejects ranges beyond 90 days and clears stale data', async () => {
    const wrapper = create(); await flushPromises()
    const dates = wrapper.findAll('input[type="date"]')
    await dates[0].setValue('2025-01-01'); await dates[1].setValue('2026-09-14')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(getReport).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[role="alert"]').text()).toContain('90')
    expect(wrapper.find('table').exists()).toBe(false)
    wrapper.unmount()
  })
  it('shows failures instead of fabricated zero metrics', async () => {
    getReport.mockRejectedValue(new Error('unavailable'))
    const wrapper = create(); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.find('table').exists()).toBe(false)
    wrapper.unmount()
  })
})
