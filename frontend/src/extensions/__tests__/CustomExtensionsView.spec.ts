import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import CustomExtensionsView from '../views/CustomExtensionsView.vue'
import { extensionAPI } from '../api'
import { extensionIds } from '../catalog'

vi.mock('../api', () => ({ extensionAPI: { list: vi.fn(), publicState: vi.fn(), update: vi.fn() } }))
const stubs = {
  AppLayout: { template: '<main><slot /></main>' }, Icon: true,
  ConfirmDialog: { props: ['show', 'title', 'message'], emits: ['confirm', 'cancel'], template: '<div v-if="show" data-dialog><p>{{title}}</p><p>{{message}}</p><button data-confirm @click="$emit(\'confirm\')">confirm</button><button data-cancel @click="$emit(\'cancel\')">cancel</button></div>' },
}
const items = [
  { id: 'playground', name: '体验中心', description: '生成入口', managed: true, enabled: true, disable_behavior: '运行中任务继续执行', paths: ['/playground'] },
  { id: 'subscription-extensions', name: '订阅增强', description: '待权益关闭契约', managed: false, enabled: null, disable_behavior: '待解耦', paths: [] },
]

describe('custom extension management', () => {
  beforeEach(() => {
    setActivePinia(createPinia()); vi.resetAllMocks()
    vi.mocked(extensionAPI.list).mockResolvedValue({ data: { items } } as never)
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, true])) } } as never)
    vi.mocked(extensionAPI.update).mockResolvedValue({ data: { id: 'playground', enabled: false } } as never)
  })
  it('does not render fake toggles for pending core modules', async () => {
    const wrapper = mount(CustomExtensionsView, { global: { stubs } }); await flushPromises()
    expect(wrapper.findAll('[role="switch"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('尚不可切换')
    expect(wrapper.text()).toContain('不是全部二开的可卸载插件')
  })
  it('renders the actual 14-managed/0-pending catalog without a security bypass toggle', async () => {
    const catalog = JSON.parse(readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../../../backend/internal/customize/catalog.json'), 'utf8'))
    vi.mocked(extensionAPI.list).mockResolvedValue({ data: { items: catalog.map((item: { managed: boolean }) => ({ ...item, enabled: item.managed ? true : null })) } } as never)
    const wrapper = mount(CustomExtensionsView, { global: { stubs } }); await flushPromises()
    expect(wrapper.findAll('[role="switch"]')).toHaveLength(14)
    const security = wrapper.findAll('article').find(article => article.text().includes('访问与注册策略增强'))!
    expect(security.exists()).toBe(true)
    expect(security.find('[role="switch"]').exists()).toBe(true)
    expect(security.text()).toContain('禁止修改二开安全配置')
    expect(security.text()).toContain('安全规则仍按原安全设置执行')
    const admin = wrapper.findAll('article').find(article => article.text().includes('账号、分组与用户管理增强'))!
    expect(admin.exists()).toBe(true)
    expect(admin.find('[role="switch"]').exists()).toBe(true)
    expect(admin.text()).toContain('独立仓储端口')
    expect(admin.text()).toContain('保留可读')
    expect(wrapper.findAll('article').filter(article => !article.find('[role="switch"]').exists())).toHaveLength(0)
  })
  it('asks for confirmation and saves the requested state', async () => {
    const wrapper = mount(CustomExtensionsView, { global: { stubs } }); await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    expect(extensionAPI.update).not.toHaveBeenCalled()
    await wrapper.get('[data-confirm]').trigger('click'); await flushPromises()
    expect(extensionAPI.update).toHaveBeenCalledWith('playground', false)
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
  })
  it('explains restoration rather than disablement when enabling a module', async () => {
    vi.mocked(extensionAPI.publicState).mockResolvedValue({ data: { enabled: Object.fromEntries(extensionIds.map(id => [id, false])) } } as never)
    vi.mocked(extensionAPI.update).mockResolvedValue({ data: { id: 'playground', enabled: true } } as never)
    const wrapper = mount(CustomExtensionsView, { global: { stubs } }); await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    expect(wrapper.get('[data-dialog]').text()).toContain('开启后恢复')
    await wrapper.get('[data-confirm]').trigger('click'); await flushPromises()
    expect(extensionAPI.update).toHaveBeenCalledWith('playground', true)
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
  })
  it('does not mutate a cancelled toggle', async () => {
    const wrapper = mount(CustomExtensionsView, { global: { stubs } }); await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click'); await wrapper.get('[data-cancel]').trigger('click')
    expect(extensionAPI.update).not.toHaveBeenCalled()
  })
  it('reports unknown state rather than claiming the module is disabled during an outage', async () => {
    vi.mocked(extensionAPI.publicState).mockRejectedValue(new Error('offline'))
    const wrapper = mount(CustomExtensionsView, { global: { stubs } }); await flushPromises()
    expect(wrapper.text()).toContain('状态未知')
    expect(wrapper.text()).not.toContain('○ 已关闭')
    expect(wrapper.get('[role="switch"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('无法读取')
  })
  it('shows a save failure without claiming success', async () => {
    vi.mocked(extensionAPI.update).mockRejectedValue(new Error('offline'))
    const wrapper = mount(CustomExtensionsView, { global: { stubs } }); await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click'); await wrapper.get('[data-confirm]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('保存失败')
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
  })
})
