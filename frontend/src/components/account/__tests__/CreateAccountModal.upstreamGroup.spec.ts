import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CreateAccountModal from '../CreateAccountModal.vue'

const { createAccount } = vi.hoisted(() => ({
  createAccount: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: true })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccount,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false })
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({ account_quota_notify_enabled: false })
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn().mockResolvedValue({ mappings: {} })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const upstreamGroups = [
  { key: 'hi-code', name: 'hi-code', account_count: 4 },
  { key: 'other', name: 'Other upstream', account_count: 2 }
]

function mountModal() {
  return mount(CreateAccountModal, {
    props: {
      show: true,
      proxies: [],
      groups: [],
      upstreamGroups
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        Select: true,
        PlatformIcon: true,
        Icon: true,
        ProxySelector: true,
        ProxyAdBanner: true,
        GroupSelector: true,
        ModelWhitelistSelector: true,
        QuotaLimitCard: true,
        OAuthAuthorizationFlow: true
      }
    }
  })
}

describe('CreateAccountModal upstream group', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createAccount.mockResolvedValue({ id: 1 })
  })

  it('offers existing groups while allowing a new name', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const input = wrapper.get('[data-testid="upstream-group-input"]')
    const listId = input.attributes('list')
    expect(wrapper.findAll(`#${listId} option`).map(option => option.attributes('value'))).toEqual([
      'hi-code',
      'Other upstream'
    ])

    await input.setValue('My new upstream')
    expect((input.element as HTMLInputElement).value).toBe('My new upstream')
  })

  it('submits the group independently from the account Base URL', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const apiKeyType = wrapper.findAll('button')
      .find(button => button.text().includes('admin.accounts.claudeConsole'))
    await apiKeyType!.trigger('click')
    await flushPromises()

    await wrapper.get('input[placeholder="admin.accounts.enterAccountName"]').setValue('hi-code key 4')
    await wrapper.get('[data-testid="upstream-group-input"]').setValue('hi-code')
    await wrapper.get('input[placeholder="https://api.anthropic.com"]').setValue('https://edge-4.example/v1')
    await wrapper.get('input[placeholder="sk-ant-..."]').setValue('secret-key')
    await wrapper.get('#create-account-form').trigger('submit')
    await flushPromises()

    expect(createAccount).toHaveBeenCalledWith(expect.objectContaining({
      upstream_group: 'hi-code',
      credentials: expect.objectContaining({
        base_url: 'https://edge-4.example/v1',
        api_key: 'secret-key'
      })
    }))
  })
})
