import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('AccountBulkActionsBar', () => {
  it('emits set-upstream-group for selected accounts', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1]
      }
    })

    const groupButton = wrapper.get('[data-testid="set-upstream-group"]')
    await groupButton.trigger('click')
    expect(wrapper.emitted('set-upstream-group')).toHaveLength(1)
  })
})
