import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BucketsPanel from '../BucketsPanel.vue'
import BucketDialog from '../BucketDialog.vue'
import ModelStatsPanel from '../ModelStatsPanel.vue'
import GatewayLogArchiveView from '../GatewayLogArchiveView.vue'
import { archiveAPI, type ArchiveBucket, type ArchiveConfig, type Entry } from '../api'

vi.mock('../api', () => ({
  archiveAPI: {
    buckets: vi.fn(),
    createBucket: vi.fn(),
    updateBucket: vi.fn(),
    deleteBucket: vi.fn(),
    testBucket: vi.fn(),
    modelStats: vi.fn(),
    getConfig: vi.fn(),
    saveConfig: vi.fn(),
    status: vi.fn(),
    entries: vi.fn(),
    exportSession: vi.fn(),
  },
}))

const stubs = {
  AppLayout: { template: '<main><slot /></main>' },
  Icon: true,
  BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" data-dialog><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>' },
  ConfirmDialog: {
    props: ['show', 'title', 'message'],
    emits: ['confirm', 'cancel'],
    template: '<div v-if="show" data-confirm-dialog><p>{{ message }}</p><button data-confirm @click="$emit(\'confirm\')">ok</button></div>',
  },
}

describe('gateway log archive JSONL and privacy settings', () => {
  it('loads and saves the file limit and upload-only privacy switch', async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    const config: ArchiveConfig = {
      capture_enabled: false, mode: 'all', rules: [], success_only: true, redact_pii: true,
      max_file_size_mb: 10240, session_idle_seconds: 1800, max_body_mb: 64,
      keep_local_hours: 0, entry_retention_days: 90, min_free_disk_gb: 5,
    }
    vi.mocked(archiveAPI.getConfig).mockResolvedValue(config)
    vi.mocked(archiveAPI.saveConfig).mockImplementation(async value => value)
    vi.mocked(archiveAPI.buckets).mockResolvedValue({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
    const wrapper = mount(GatewayLogArchiveView, { global: { stubs } })
    try {
      await flushPromises()
      expect(wrapper.text()).toContain('每行保存一个会话 JSON')
      expect(wrapper.text()).toContain('不修改正常请求与响应')
      const privacy = wrapper.get('[aria-label="上传时脱敏个人信息"]')
      expect(privacy.attributes('aria-checked')).toBe('true')
      await privacy.trigger('click')
      await wrapper.get('input[aria-label="单个 JSONL 上限（GB）"]').setValue(20)
      await wrapper.findAll('button').find(button => button.text() === '保存采集配置')!.trigger('click')
      await flushPromises()
      expect(archiveAPI.saveConfig).toHaveBeenCalledWith(expect.objectContaining({ redact_pii: false, max_file_size_mb: 20480 }))
      expect(wrapper.text()).toContain('采集配置已保存')
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('exports all indexed session fragments from an entry and surfaces pending-fragment errors', async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    vi.mocked(archiveAPI.getConfig).mockResolvedValue({ capture_enabled: false, mode: 'all', rules: [], success_only: true, redact_pii: true,
      max_file_size_mb: 10240, session_idle_seconds: 1800, max_body_mb: 64, keep_local_hours: 0, entry_retention_days: 90, min_free_disk_gb: 5 })
    vi.mocked(archiveAPI.buckets).mockResolvedValue({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
    const entry: Entry = { id: 42, bundle_id: 1, seq: 1, entry_name: '数据_000001.json', session_id: 'shared', model: 'm', platform: 'anthropic',
      user_id: 3, api_key_id: 9, exchange_count: 2, compressed_size: 100, uncompressed_size: 100, started_at: '', ended_at: '', day: '2026-11-17', file_name: 'm.jsonl', bundle_status: 'uploaded' }
    vi.mocked(archiveAPI.entries).mockResolvedValue({ items: [entry], total: 1, page: 1, page_size: 20 })
    vi.mocked(archiveAPI.exportSession).mockResolvedValue(new Blob(['{"summary":{}}'], { type: 'application/json' }))
    const createURL = vi.fn(() => 'blob:session')
    const oldCreate = URL.createObjectURL
    const oldRevoke = URL.revokeObjectURL
    URL.createObjectURL = createURL
    URL.revokeObjectURL = vi.fn()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const wrapper = mount(GatewayLogArchiveView, { global: { stubs } })
    try {
      await flushPromises()
      await wrapper.findAll('[role="tab"]').find(tab => tab.text() === '会话数据')!.trigger('click')
      await flushPromises()
      expect(wrapper.text()).toContain('每条记录是一个节点的会话片段')
      const exportButton = wrapper.findAll('tbody button').find(button => button.text() === '合并会话导出')!
      await exportButton.trigger('click')
      await flushPromises()
      expect(archiveAPI.exportSession).toHaveBeenCalledWith(42)
      expect(createURL).toHaveBeenCalledOnce()
      expect(click).toHaveBeenCalledOnce()
      const pending = new Blob()
      Object.defineProperty(pending, 'text', { value: async () => JSON.stringify({ message: '该会话有 1 个片段尚未上传' }) })
      vi.mocked(archiveAPI.exportSession).mockRejectedValue({ response: { data: pending } })
      await exportButton.trigger('click')
      await flushPromises()
      expect(wrapper.get('[role="alert"]').text()).toContain('片段尚未上传')
      expect(createURL).toHaveBeenCalledOnce()
    } finally {
      wrapper.unmount()
      click.mockRestore()
      URL.createObjectURL = oldCreate
      URL.revokeObjectURL = oldRevoke
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })
})

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
    expect(wrapper.get('[data-confirm-dialog]').text()).toContain('已上传 3 个归档文件')
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(archiveAPI.deleteBucket).toHaveBeenCalledWith(1)
  })
})

describe('gateway log archive bucket dialog', () => {
  beforeEach(() => vi.resetAllMocks())

  it('saves RustFS as an explicit primary with its own privacy policy', async () => {
    vi.mocked(archiveAPI.createBucket).mockResolvedValue(bucket({ id: 12, storage_role: 'rustfs', privacy_mode: 'original' }))
    const wrapper = mount(BucketDialog, { props: { show: true, bucket: null, today: '2026-11-19' }, global: { stubs } })
    await wrapper.get('[aria-label="存储桶用途"]').setValue('rustfs')
    await wrapper.get('[aria-label="存储桶脱敏策略"]').setValue('original')
    expect(wrapper.text()).toContain('写入失败时自动回退本地')
    await wrapper.findAll('footer button').find(button => button.text().includes('添加存储桶'))!.trigger('click')
    await flushPromises()
    expect(archiveAPI.createBucket).toHaveBeenCalledWith(expect.objectContaining({ storage_role: 'rustfs', privacy_mode: 'original', provider: 's3-compatible', force_path_style: true }))
  })

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
