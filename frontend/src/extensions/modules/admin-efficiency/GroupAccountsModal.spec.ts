import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { Account, AdminGroup } from '@/types'
import GroupAccountsModal from './GroupAccountsModal.vue'
const { getGroupAccounts, updateGroupAccounts } = vi.hoisted(() => ({ getGroupAccounts: vi.fn(), updateGroupAccounts: vi.fn() }))
vi.mock('./managementApi', () => ({ getGroupAccounts, updateGroupAccounts }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('group membership extension ports', () => {
  it('loads through narrow ports and blocks saved callbacks after disable', async () => {
    const account = { id: 1, name: 'historical', platform: 'openai', type: 'oauth', priority: 1 } as Account
    getGroupAccounts.mockResolvedValue([account])
    updateGroupAccounts.mockResolvedValue([account])
    const listAccounts = vi.fn().mockResolvedValue({ items: [account], total: 1 })
    const updateAccount = vi.fn()
    const wrapper = mount(GroupAccountsModal, {
      props: { show: false, enabled: true, group: { id: 7, platform: 'openai' } as AdminGroup, listAccounts, updateAccount },
      global: { stubs: {
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
        VueDraggable: { template: '<div><slot /></div>' }, Icon: true, PlatformIcon: true
      } }
    })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(getGroupAccounts).toHaveBeenCalledWith(7)
    expect(listAccounts).toHaveBeenCalledWith(1, 200, { platform: 'openai', sort_by: 'priority', sort_order: 'asc' })
    expect(wrapper.text()).toContain('historical')
    // Invoke the real setup callbacks, as if a click was queued before the switch changed.
    const vm = wrapper.vm as unknown as { handleSave: () => Promise<void>; saveAccountSettings: (account: Account) => Promise<void> }
    await vm.handleSave()
    expect(updateGroupAccounts).toHaveBeenCalledWith(7, [1])
    updateGroupAccounts.mockClear()
    await wrapper.setProps({ enabled: false })
    expect(wrapper.text()).toBe('')
    await vm.handleSave()
    await vm.saveAccountSettings(account)
    expect(updateGroupAccounts).not.toHaveBeenCalled()
    expect(updateAccount).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
