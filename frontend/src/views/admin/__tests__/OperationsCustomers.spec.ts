import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OperationsCustomers from '../components/OperationsCustomers.vue'

const { getCustomers } = vi.hoisted(() => ({ getCustomers: vi.fn() }))
vi.mock('@/api/admin/operationsFinance', () => ({ getOperationsCustomers: getCustomers }))
const summary = { total: 24, paying: 10, repeat: 4, new_paying: 6, active: 12, churned: 3, previous_active: 6, balance_users: 10, repeat_rate: 40, churn_rate: 50 }
const data = { summary, total: 24, items: [], as_of: '2026-09-14T00:00:00Z', churn_days: 30 }
const create = () => mount(OperationsCustomers, { props: { startDate: '2026-09-01', endDate: '2026-09-13', timezone: 'Asia/Shanghai' }, global: { stubs: { RouterLink: true } } })
describe('Operations customer drilldown', () => {
  beforeEach(() => { vi.clearAllMocks(); Element.prototype.scrollIntoView = vi.fn(); getCustomers.mockResolvedValue(structuredClone(data)) })
  it('opens repeat and balance segments in-place with the selected reporting period', async () => {
    const wrapper = create(); await flushPromises()
    expect(wrapper.text()).toContain('40.00%')
    const repeat = wrapper.findAll('button').find(b => b.text() === '回购用户 4')!
    await repeat.trigger('click'); await flushPromises()
    expect(getCustomers.mock.lastCall?.[0]).toMatchObject({ segment: 'repeat', start_date: '2026-09-01', end_date: '2026-09-13', page: 1 })
    wrapper.vm.open('balance'); await flushPromises()
    expect(getCustomers.mock.lastCall?.[0].segment).toBe('balance')
    wrapper.unmount()
  })
  it('resets pagination when changing churn cycle and preserves the churn segment', async () => {
    const wrapper = create(); await flushPromises()
    wrapper.vm.open('churned'); await flushPromises()
    await wrapper.get('[aria-label="下一页用户"]').trigger('click'); await flushPromises()
    expect(getCustomers.mock.lastCall?.[0].page).toBe(2)
    await wrapper.findAll('select')[0].setValue('7'); await flushPromises()
    expect(getCustomers.mock.lastCall?.[0]).toMatchObject({ segment: 'churned', churn_days: 7, page: 1 })
    wrapper.unmount()
  })
  it('shows a failure without stale customer metrics', async () => {
    const wrapper = create(); await flushPromises()
    getCustomers.mockRejectedValueOnce(new Error('unavailable'))
    await wrapper.get('[aria-label="刷新用户数据"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('加载失败')
    expect(wrapper.find('table').exists()).toBe(false)
    wrapper.unmount()
  })
})
