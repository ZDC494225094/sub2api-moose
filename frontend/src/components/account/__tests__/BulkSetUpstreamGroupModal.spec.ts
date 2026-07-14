import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BulkSetUpstreamGroupModal from '../BulkSetUpstreamGroupModal.vue'
import { adminAPI } from '@/api/admin'

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      bulkUpdate: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const upstreamGroups = [
  { key: 'hi-code', name: 'hi-code', account_count: 4 },
  { key: 'other', name: 'Other upstream', account_count: 2 }
]

function mountModal(extraProps: Record<string, unknown> = {}) {
  return mount(BulkSetUpstreamGroupModal, {
    props: {
      show: true,
      accountIds: [1, 2],
      upstreamGroups,
      loading: false,
      ...extraProps
    },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>'
        }
      }
    }
  })
}

describe('BulkSetUpstreamGroupModal', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockReset()
    vi.mocked(adminAPI.accounts.bulkUpdate).mockResolvedValue({
      success: 2,
      failed: 0,
      results: []
    } as any)
    appStore.showError.mockReset()
    appStore.showSuccess.mockReset()
  })

  it('shows existing groups and applies a trimmed new group name', async () => {
    const wrapper = mountModal()
    const input = wrapper.get('[data-testid="upstream-group-input"]')
    const listId = input.attributes('list')

    expect(wrapper.findAll(`#${listId} option`).map((option) => option.attributes('value'))).toEqual([
      'hi-code',
      'Other upstream'
    ])

    await input.setValue('  new upstream  ')
    await wrapper.get('#bulk-set-upstream-group-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      upstream_group: 'new upstream'
    })
    expect(wrapper.emitted('updated')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('sends an explicit empty group when clearing the current grouping', async () => {
    const wrapper = mountModal()

    await wrapper.get('[data-testid="clear-upstream-group"]').setValue(true)
    expect(wrapper.find('[data-testid="upstream-group-input"]').exists()).toBe(false)

    await wrapper.get('#bulk-set-upstream-group-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      upstream_group: ''
    })
    expect(wrapper.emitted('updated')).toHaveLength(1)
  })

  it('does not submit an empty group without the clear switch', async () => {
    const wrapper = mountModal()

    await wrapper.get('#bulk-set-upstream-group-form').trigger('submit.prevent')

    expect(adminAPI.accounts.bulkUpdate).not.toHaveBeenCalled()
    expect(appStore.showError).toHaveBeenCalledWith('admin.accounts.bulkSetUpstreamGroup.required')
  })

  it('keeps the dialog open and reports request errors', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.mocked(adminAPI.accounts.bulkUpdate).mockRejectedValueOnce(new Error('request failed'))
    const wrapper = mountModal()

    await wrapper.get('[data-testid="upstream-group-input"]').setValue('hi-code')
    await wrapper.get('#bulk-set-upstream-group-form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.emitted('updated')).toBeUndefined()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(appStore.showError).toHaveBeenCalledWith('request failed')
    consoleError.mockRestore()
  })
})
