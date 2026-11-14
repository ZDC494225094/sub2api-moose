import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AddAccessDialog from '../AddAccessDialog.vue'
import AccessPanel from '../AccessPanel.vue'
import AdminApplicationsPanel from '../AdminApplicationsPanel.vue'
import usersAPI from '@/api/admin/users'
import { invoiceAdminAPI, type AccessEntry } from '../api'
import type { AdminUser } from '@/types'

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/api/admin/users', () => ({ default: { list: vi.fn() } }))
vi.mock('../api', () => ({ invoiceAdminAPI: { access: vi.fn(), addAccess: vi.fn(), updateAccess: vi.fn(), applications: vi.fn() } }))
vi.mock('../makeout', () => ({ TAX_MAKEOUT_URL: 'https://dppt.jiangsu.chinatax.gov.cn:8443/blue-invoice-makeout', goMakeout: vi.fn(), downloadMakeoutAssistant: vi.fn() }))
const stubs = {
  Icon: true,
  ReviewDialog: true,
  ConfirmDialog: true,
  BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" data-dialog><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>' },
}
const user = (id: number) => ({ id, email: `user${id}@example.com`, username: `user${id}`, status: 'active' }) as AdminUser
const entry: AccessEntry = { user_id: 1, email: 'user1@example.com', username: 'user1', note: '客户', min_amount: null, created_at: '' }

beforeEach(() => { vi.clearAllMocks() })
afterEach(() => { vi.useRealTimers() })

describe('invoice allowlist', () => {
  it('starts as a list and loads selectable users only after clicking add', async () => {
    vi.mocked(invoiceAdminAPI.access).mockResolvedValue([entry])
    vi.mocked(usersAPI.list).mockResolvedValue({ items: [user(1), user(2)], total: 2, page: 1, page_size: 20, pages: 1 })
    const wrapper = mount(AccessPanel, { props: { mode: 'allowlist' }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('user1@example.com')
    expect(wrapper.text()).toContain('跟随全局设置')
    expect(usersAPI.list).not.toHaveBeenCalled()
    await wrapper.findAll('button').find((button) => button.text() === '添加用户')!.trigger('click')
    await flushPromises()
    expect(usersAPI.list).toHaveBeenCalledTimes(1)
    const existing = wrapper.findAll('[data-dialog] tbody input')[0]
    expect(existing.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('retains bulk selection across search and page changes and submits IDs', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    vi.mocked(usersAPI.list)
      .mockResolvedValueOnce({ items: [user(1), user(2)], total: 41, page: 1, page_size: 20, pages: 3 })
      .mockResolvedValueOnce({ items: [user(3)], total: 41, page: 2, page_size: 20, pages: 3 })
      .mockResolvedValueOnce({ items: [user(4)], total: 1, page: 1, page_size: 20, pages: 1 })
    vi.mocked(invoiceAdminAPI.addAccess).mockResolvedValue({ added: 3, missing: [] })
    const wrapper = mount(AddAccessDialog, { props: { show: true, existingIds: [1] }, global: { stubs } })
    await flushPromises()
    await wrapper.find('thead input').setValue(true)
    await wrapper.findAll('button').find((button) => button.text() === '下一页')!.trigger('click')
    await flushPromises()
    await wrapper.find('thead input').setValue(true)
    await wrapper.find('[aria-label="搜索用户"]').setValue('user4')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(usersAPI.list).toHaveBeenLastCalledWith(1, 20, { search: 'user4', include_subscriptions: false }, { signal: expect.any(AbortSignal) })
    await wrapper.find('thead input').setValue(true)
    expect(wrapper.text()).toContain('已选 3 人')
    await wrapper.findAll('button').find((button) => button.text() === '添加所选用户')!.trigger('click')
    await flushPromises()
    expect(invoiceAdminAPI.addAccess).toHaveBeenCalledWith(['2', '3', '4'], '')
    expect(wrapper.emitted('added')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('saves explicit zero separately from global inheritance', async () => {
    vi.mocked(invoiceAdminAPI.access).mockResolvedValue([entry])
    vi.mocked(invoiceAdminAPI.updateAccess).mockResolvedValue()
    const wrapper = mount(AccessPanel, { props: { mode: 'allowlist' }, global: { stubs: { ...stubs, AddAccessDialog: true } } })
    await flushPromises()
    const edit = () => wrapper.findAll('button').find((button) => button.text() === '设置门槛')!
    await edit().trigger('click')
    await wrapper.find('form input[type="checkbox"]').setValue(false)
    await wrapper.find('input[type="number"]').setValue('0')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(invoiceAdminAPI.updateAccess).toHaveBeenCalledWith(1, { min_amount: '0', note: '客户' })
    await edit().trigger('click')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(invoiceAdminAPI.updateAccess).toHaveBeenLastCalledWith(1, { min_amount: null, note: '客户' })
    wrapper.unmount()
  })
})

it('offers makeout only for pending/processing and blocks refunded applications', async () => {
  vi.mocked(invoiceAdminAPI.applications).mockResolvedValue({
    items: [
      { ...entry, id: 1, status: 'pending', amount: '100', title_type: 'personal', invoice_type: 'normal' },
      { ...entry, id: 2, status: 'processing', amount: '100', title_type: 'personal', invoice_type: 'normal', refund_alert: true },
      { ...entry, id: 3, status: 'issued', amount: '100', title_type: 'personal', invoice_type: 'normal' },
    ], total: 3, page: 1, page_size: 20, pages: 1,
  } as Awaited<ReturnType<typeof invoiceAdminAPI.applications>>)
  const wrapper = mount(AdminApplicationsPanel, { global: { stubs } })
  await flushPromises()
  const buttons = wrapper.findAll('button').filter((button) => button.text() === '去开票')
  expect(buttons).toHaveLength(2)
  expect(buttons[0].attributes('disabled')).toBeUndefined()
  expect(buttons[1].attributes('disabled')).toBeDefined()
  wrapper.unmount()
})
