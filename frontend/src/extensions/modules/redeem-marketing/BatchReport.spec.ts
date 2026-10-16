import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BatchReport from './BatchReport.vue'
import { redeemMarketingAPI, type MarketingBatch } from './api'

vi.mock('./api', () => ({ redeemMarketingAPI: { batches: vi.fn() } }))

const batch: MarketingBatch = {
  batch_id: 'batch-one', created_at: '2026-10-16T00:00:00Z', registered_after: null,
  code_count: 10, redeemed_users: 4, redeemed_value: 8,
  paying_users: 2, order_count: 3, pay_amounts: { CNY: 30, USD: 5 }
}

describe('marketing batch report', () => {
  beforeEach(() => { vi.resetAllMocks() })
  const render = () => mount(BatchReport, { global: { stubs: { Teleport: true } } })
  it('loads only when opened, shows actual cohort conversion and separates currencies', async () => {
    vi.mocked(redeemMarketingAPI.batches).mockResolvedValue([batch])
    const wrapper = render()
    expect(redeemMarketingAPI.batches).not.toHaveBeenCalled()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(redeemMarketingAPI.batches).toHaveBeenCalledWith('', 1)
    expect(wrapper.text()).toContain('50.0%')
    expect(wrapper.text()).toContain('40.0%')
    expect(wrapper.text()).toContain('CNY 30.00 / USD 5.00')
    expect(wrapper.text()).toContain('不限注册时间')
    wrapper.unmount()
  })
  it('queries an exact batch and renders zero conversion safely', async () => {
    vi.mocked(redeemMarketingAPI.batches).mockResolvedValue([{ ...batch, redeemed_users: 0, paying_users: 0, pay_amounts: {}, registered_after: batch.created_at }])
    const wrapper = render()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    await wrapper.get('input').setValue(' batch-one ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(redeemMarketingAPI.batches).toHaveBeenLastCalledWith('batch-one', 1)
    expect(wrapper.text()).toContain('0.0%')
    expect(wrapper.text()).not.toContain('NaN')
    expect(wrapper.text()).toContain('仅新注册用户')
    wrapper.unmount()
  })
  it('clears stale results on a failed refresh and exposes the error', async () => {
    vi.mocked(redeemMarketingAPI.batches).mockResolvedValueOnce([batch]).mockRejectedValueOnce({ response: { data: { detail: '插件已停用' } } })
    const wrapper = render()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('batch-one')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('插件已停用')
    expect(wrapper.text()).not.toContain('batch-one')
    wrapper.unmount()
  })
})
