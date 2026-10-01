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
  { id: 'multi-group-billing', name: '计费', description: '核心逻辑', managed: false, enabled: null, disable_behavior: '待解耦', paths: [] },
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
