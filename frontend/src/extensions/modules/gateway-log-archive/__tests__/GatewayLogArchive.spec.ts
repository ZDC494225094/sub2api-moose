import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BucketsPanel from '../BucketsPanel.vue'
import BucketDialog from '../BucketDialog.vue'
import ModelStatsPanel from '../ModelStatsPanel.vue'
import { archiveAPI, type ArchiveBucket } from '../api'

vi.mock('../api', () => ({
  archiveAPI: {
    buckets: vi.fn(),
    createBucket: vi.fn(),
    updateBucket: vi.fn(),
    deleteBucket: vi.fn(),
    testBucket: vi.fn(),
    modelStats: vi.fn(),
  },
}))

const stubs = {
  Icon: true,
  BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" data-dialog><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>' },
  ConfirmDialog: {
    props: ['show', 'title', 'message'],
    emits: ['confirm', 'cancel'],
    template: '<div v-if="show" data-confirm-dialog><p>{{ message }}</p><button data-confirm @click="$emit(\'confirm\')">ok</button></div>',
  },
}

function bucket(overrides: Partial<ArchiveBucket> = {}): ArchiveBucket {
  return {
    id: 1, name: '主存储', provider: 'aws', bucket: 'incoming-04', region: 'us-east-1', endpoint: '', prefix: 'logs',
    force_path_style: false, enabled: true, priority: 10, access_key_id: 'AKIA****1234', secret_access_key_set: true,
    session_token_set: false, usage: { bundle_count: 3, size_bytes: 3 * 2 ** 30 }, created_at: '', updated_at: '',
    ...overrides,
  }
}

describe('gateway log archive buckets', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(archiveAPI.buckets).mockResolvedValue({
      items: [bucket({ id: 2, name: '停用', enabled: false, priority: 1 }), bucket(), bucket({ id: 3, name: '备用', priority: 20, usage: { bundle_count: 0, size_bytes: 0 } })],
      unattributed: { bundle_count: 1, size_bytes: 1024 },
    })
  })

  it('lists buckets as a table with masked keys, usage and the active upload target', async () => {
    const wrapper = mount(BucketsPanel, { props: { today: '2026-11-05' }, global: { stubs } })
    await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(3)
    expect(rows[1].text()).toContain('当前')
    expect(rows[0].text()).not.toContain('当前')
    expect(rows[1].text()).toContain('AKIA****1234')
    expect(rows[1].text()).toContain('3.00 GB')
    expect(wrapper.text()).toContain('上传到已删除的存储桶')
    expect(wrapper.html()).not.toContain('secret')
  })

  it('switches a bucket off through the API and reports the change', async () => {
    vi.mocked(archiveAPI.updateBucket).mockResolvedValue(bucket({ enabled: false }))
    const wrapper = mount(BucketsPanel, { props: { today: '2026-11-05' }, global: { stubs } })
    await flushPromises()
    await wrapper.findAll('tbody tr')[1].get('[role="switch"]').trigger('click')
    await flushPromises()
    expect(archiveAPI.updateBucket).toHaveBeenCalledWith(1, expect.objectContaining({ enabled: false, bucket: 'incoming-04' }))
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('asks before deleting and explains what happens to uploaded bundles', async () => {
    const wrapper = mount(BucketsPanel, { props: { today: '2026-11-05' }, global: { stubs } })
    await flushPromises()
    const del = wrapper.findAll('tbody tr')[1].findAll('button').find(b => b.text() === '删除')!
    await del.trigger('click')
    expect(archiveAPI.deleteBucket).not.toHaveBeenCalled()
    expect(wrapper.get('[data-confirm-dialog]').text()).toContain('已上传 3 个压缩包')
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(archiveAPI.deleteBucket).toHaveBeenCalledWith(1)
  })
})

describe('gateway log archive bucket dialog', () => {
  beforeEach(() => vi.resetAllMocks())

  it('creates a bucket from the modal form including new credentials', async () => {
    vi.mocked(archiveAPI.createBucket).mockResolvedValue(bucket({ id: 9 }))
    const wrapper = mount(BucketDialog, { props: { show: true, bucket: null, today: '2026-11-05' }, global: { stubs } })
    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('R2 备份')
    await wrapper.get('input[placeholder="incoming-04-20260624"]').setValue('moose-archive')
    await wrapper.get('input[placeholder="未设置"]').setValue('AK-NEW')
    await wrapper.findAll('footer button').find(b => b.text().includes('添加存储桶'))!.trigger('click')
    await flushPromises()
    expect(archiveAPI.createBucket).toHaveBeenCalledWith(expect.objectContaining({ name: 'R2 备份', bucket: 'moose-archive', access_key_id: 'AK-NEW', enabled: true }))
    expect(wrapper.emitted('saved')?.[0]?.[0]).toMatchObject({ id: 9 })
  })

  it('edits without resending stored secrets and tests with the bucket id', async () => {
    vi.mocked(archiveAPI.testBucket).mockResolvedValue({ ok: true, steps: [{ name: '访问存储桶', ok: true }] })
    vi.mocked(archiveAPI.updateBucket).mockResolvedValue(bucket())
    const wrapper = mount(BucketDialog, { props: { show: true, bucket: bucket(), today: '2026-11-05' }, global: { stubs } })
    expect(wrapper.text()).toContain('清除已保存的全部凭证')
    await wrapper.findAll('footer button').find(b => b.text().includes('测试连接'))!.trigger('click')
    await flushPromises()
    expect(archiveAPI.testBucket).toHaveBeenCalledWith(expect.objectContaining({ id: 1, secret_access_key: '' }))
    expect(wrapper.text()).toContain('连接测试通过')
    await wrapper.findAll('footer button').find(b => b.text().includes('保存修改'))!.trigger('click')
    await flushPromises()
    expect(archiveAPI.updateBucket).toHaveBeenCalledWith(1, expect.objectContaining({ secret_access_key: '', access_key_id: '' }))
  })
})

describe('gateway log archive model stats', () => {
  it('drills the uploaded total down by model and links to its bundles', async () => {
    vi.mocked(archiveAPI.modelStats).mockResolvedValue({
      items: [
        { model_key: 'Opus5', bundle_count: 2, entry_count: 40, size_bytes: 3 * 2 ** 30, last_uploaded_at: null },
        { model_key: 'gpt-6', bundle_count: 1, entry_count: 10, size_bytes: 2 ** 30, last_uploaded_at: null },
      ],
      total_bundles: 3, total_entries: 50, total_bytes: 4 * 2 ** 30,
    })
    const wrapper = mount(ModelStatsPanel, { props: { buckets: [bucket()] }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('4.00 GB')
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('Opus5')
    expect(rows[0].text()).toContain('75.0%')
    await wrapper.get('select').setValue('1')
    await flushPromises()
    expect(archiveAPI.modelStats).toHaveBeenLastCalledWith(1)
    await rows[1].get('button').trigger('click')
    expect(wrapper.emitted('viewBundles')?.[0]).toEqual(['gpt-6'])
  })
})
