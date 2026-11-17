import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from '@/views/auth/LoginView.vue'

const { getPublicSettingsMock, pushMock, loginMock, showWarningMock, buildOAuthLoginStartURLMock } = vi.hoisted(() => ({
  getPublicSettingsMock: vi.fn(),
  pushMock: vi.fn(),
  loginMock: vi.fn(),
  showWarningMock: vi.fn(),
  buildOAuthLoginStartURLMock: vi.fn()
}))

const publicSettings = {
  registration_enabled: true,
  turnstile_enabled: false,
  turnstile_site_key: '',
  tencent_captcha_enabled: false,
  tencent_captcha_app_id: '',
  aliyun_captcha_enabled: false,
  aliyun_captcha_scene_id: '',
  aliyun_captcha_prefix: '',
  linuxdo_oauth_enabled: false,
  dingtalk_oauth_enabled: false,
  wechat_oauth_enabled: false,
  backend_mode_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: 'OIDC',
  github_oauth_enabled: false,
  google_oauth_enabled: false,
  password_reset_enabled: false,
  passkey_enabled: false,
  login_agreement_enabled: false,
  login_agreement_documents: []
}

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
    currentRoute: { value: { query: {} } }
  })
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key
    }
  }),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    login: loginMock,
    loginWithPasskey: vi.fn(),
    login2FA: vi.fn()
  }),
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: showWarningMock
  })
}))

vi.mock('@/api/auth', () => ({
  buildOAuthLoginStartURL: buildOAuthLoginStartURLMock,
  getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
  isTotp2FARequired: vi.fn(() => false),
  isWeChatWebOAuthEnabled: vi.fn(() => false),
  startOAuthLogin: vi.fn()
}))

function mountLogin() {
  return mount(LoginView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        DingTalkOAuthSection: true,
        EmailOAuthButtons: true,
        Icon: true,
        LinuxDoOAuthSection: true,
        OidcOAuthSection: true,
        RouterLink: { template: '<a><slot /></a>' },
        TotpLoginModal: true,
        TurnstileWidget: true,
        WechatOAuthSection: true,
        transition: false
      }
    }
  })
}

describe('LoginView registration entry', () => {
  beforeEach(() => {
    localStorage.removeItem('sub2api_login_agreement_consent')
    loginMock.mockReset()
    loginMock.mockResolvedValue({})
    showWarningMock.mockReset()
    buildOAuthLoginStartURLMock.mockReset()
    getPublicSettingsMock.mockReset()
    pushMock.mockReset()
    getPublicSettingsMock.mockResolvedValue(publicSettings)
  })

  it('allows editing before consent, blocks submission, and submits the same values after consent', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      github_oauth_enabled: true,
      login_agreement_enabled: true,
      login_agreement_mode: 'checkbox',
      login_agreement_revision: 'login-test-v1',
      login_agreement_documents: [{ id: 'terms', title: 'Terms', content: 'Terms' }]
    })
    const wrapper = mountLogin()
    await flushPromises()

    for (const selector of ['#email', '#password', 'button[type="submit"]']) {
      expect(wrapper.get(selector).attributes('disabled')).toBeUndefined()
    }
    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('#password').setValue('secret-123')
    expect(showWarningMock).not.toHaveBeenCalled()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(showWarningMock).not.toHaveBeenCalled()
    expect(wrapper.find('.agreement-shake').exists()).toBe(true)
    expect(wrapper.get('#login-agreement-consent').attributes('aria-invalid')).toBe('true')
    expect(loginMock).not.toHaveBeenCalled()

    await wrapper.get('.agreement-shake').trigger('animationend')
    expect(wrapper.find('.agreement-shake').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('.agreement-shake').exists()).toBe(true)
    expect(loginMock).not.toHaveBeenCalled()

    wrapper.getComponent({ name: 'EmailOAuthButtons' }).vm.$emit('start', { provider: 'github' })
    await flushPromises()
    expect(buildOAuthLoginStartURLMock).not.toHaveBeenCalled()

    await wrapper.get('#login-agreement-consent').setValue(true)
    expect(wrapper.find('.agreement-shake').exists()).toBe(false)
    expect(wrapper.get('#login-agreement-consent').attributes('aria-invalid')).toBeUndefined()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(loginMock).toHaveBeenCalledOnce()
    expect(loginMock).toHaveBeenCalledWith(expect.objectContaining({ email: 'user@example.com', password: 'secret-123' }))

    await wrapper.get('#login-agreement-consent').setValue(false)
    expect(wrapper.get('#email').attributes('disabled')).toBeUndefined()
    expect((wrapper.get('#password').element as HTMLInputElement).value).toBe('secret-123')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(loginMock).toHaveBeenCalledOnce()
    expect(showWarningMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows the registration entry when registration is enabled', async () => {
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.signUp')
  })

  it('hides the registration entry when registration is disabled', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      registration_enabled: false
    })

    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('auth.signUp')
  })
})
