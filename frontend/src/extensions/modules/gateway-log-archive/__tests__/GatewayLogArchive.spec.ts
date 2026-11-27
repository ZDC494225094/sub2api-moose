import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { searchUsers } from '@/api/admin/usage'
import { getById } from '@/api/admin/users'
import type { AdminUser } from '@/types'
import BucketsPanel from '../BucketsPanel.vue'
import BucketDialog from '../BucketDialog.vue'
import ModelStatsPanel from '../ModelStatsPanel.vue'
import GatewayLogArchiveView from '../GatewayLogArchiveView.vue'
import DetectorSettings from '../DetectorSettings.vue'
import UploadStatsPanel from '../UploadStatsPanel.vue'
import { archiveAPI, type ArchiveBucket, type ArchiveConfig, type ArchiveStatus, type Bundle, type DetectorConfig, type Entry } from '../api'
import { shortError } from '../format'

vi.mock(import('@/api/admin/usage'), async importOriginal => ({ ...await importOriginal(), searchUsers: vi.fn() }))
vi.mock(import('@/api/admin/users'), async importOriginal => ({ ...await importOriginal(), getById: vi.fn() }))
vi.mock('../api', () => ({
  archiveAPI: {
    buckets: vi.fn(),
    createBucket: vi.fn(),
    updateBucket: vi.fn(),
    deleteBucket: vi.fn(),
    testBucket: vi.fn(),
    testDetector: vi.fn(),
    modelStats: vi.fn(),
    getConfig: vi.fn(),
    saveConfig: vi.fn(),
    status: vi.fn(),
    entries: vi.fn(),
    exportSession: vi.fn(),
    bundles: vi.fn(),
    deleteBundle: vi.fn(),
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
      expect(wrapper.text()).toContain('当前节点不支持按用户排除')
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

  it('keeps upload metrics and hides all redaction statistics when the detector is switched off', async () => {
    const counts = (detector_ms: number, replacements: Record<string, number>) => ({ records: 10, changed: 4, tool_skipped: 0, detector_ms, replacements })
    const GB = 1024 ** 3
    const status = {
      disk_free_bytes: 5 * GB,
      detector: { enabled: true, requests: 1, texts: 1, bytes: 1, cache_hits: 0, errors: 0, fail_open_skips: 0 },
      bundles: [{ status: 'sealed', count: 3, bytes: 6 * GB }, { status: 'uploaded', count: 9, bytes: 99 * GB }, { status: 'failed', count: 1, bytes: 2 * GB }],
      uploading: null,
      upload_stats: {
        totals: { uploads: 2, failed: 1, bytes: 2 * GB, duration_ms: 400_000, network_ms: 40_000, since: '2026-11-25T00:00:00Z', redaction: counts(300_000, { 'model:private_person': 18082, 'model:private_address': 59, 'rule:phone': 3 }) },
        recent: [
          { file_name: 'gpt-2026-11-19.jsonl', bytes: GB, started_at: '2026-11-25T01:00:00Z', duration_ms: 200_000, network_ms: 20_000, redacted: true, ok: true, redaction: counts(150_000, { 'model:private_person': 9000 }) },
          { file_name: 'gpt-2026-11-18.jsonl', bytes: GB, started_at: '2026-11-25T00:00:00Z', duration_ms: 200_000, network_ms: 20_000, redacted: true, ok: true, redaction: counts(150_000, {}) },
          { file_name: 'gpt-2026-11-17.jsonl', bytes: 0, started_at: '2026-11-24T23:00:00Z', duration_ms: 1000, network_ms: 0, redacted: true, ok: false, redaction: counts(0, {}) },
        ],
      },
    } as unknown as ArchiveStatus
    const wrapper = mount(UploadStatsPanel, { props: { status, detectorEnabled: true } })
    // Sealed and failed files wait for upload: 8 GB, more than the free disk.
    expect(wrapper.get('[data-testid="backlog"]').text()).toBe('4 个 · 8.00 GB')
    expect(wrapper.text()).toContain('超过剩余磁盘空间 5.00 GB')
    // 2 GB in 400 s is 5.12 MB/s; 8 GB then takes 1600 s.
    expect(wrapper.text()).toContain('5.12 MB/s')
    expect(wrapper.get('[data-testid="backlog-eta"]').text()).toBe('26 分 40 秒')
    expect(wrapper.text()).toContain('等待存储桶 10%')
    expect(wrapper.text()).toContain('模型脱敏服务 75%')
    expect(wrapper.get('[data-testid="bottleneck"]').text()).toContain('75% 的上传时间在等待模型脱敏服务')
    expect(wrapper.get('[data-testid="replacement-kinds"]').text()).toContain('人名 × 18,082')
    expect(wrapper.get('[data-testid="replacement-kinds"]').text()).toContain('地点 × 59')
    expect(wrapper.text()).toContain('失败')
    await wrapper.setProps({ detectorEnabled: false })
    expect(wrapper.get('[data-testid="backlog"]').text()).toContain('8.00 GB')
    expect(wrapper.text()).toContain('5.12 MB/s')
    expect(wrapper.text()).toContain('gpt-2026-11-19.jsonl')
    expect(wrapper.text()).not.toMatch(/脱敏|模型服务|替换|已修改会话/)
    expect(wrapper.find('[data-testid="bottleneck"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('saves the natural-language-only redaction scope and hides it on older nodes', async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    const config: ArchiveConfig = {
      capture_enabled: false, mode: 'all', rules: [], success_only: true, redact_pii: true, redact_scope: 'all',
      max_file_size_mb: 10240, session_idle_seconds: 1800, max_body_mb: 64,
      keep_local_hours: 0, entry_retention_days: 90, min_free_disk_gb: 5,
    }
    vi.mocked(archiveAPI.getConfig).mockResolvedValue(config)
    vi.mocked(archiveAPI.saveConfig).mockImplementation(async value => value)
    vi.mocked(archiveAPI.buckets).mockResolvedValue({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
    const wrapper = mount(GatewayLogArchiveView, { global: { stubs } })
    try {
      await flushPromises()
      const natural = wrapper.findAll('[role="radio"]').find(button => button.text() === '仅自然语言')!
      expect(natural.attributes('aria-checked')).toBe('false')
      await natural.trigger('click')
      expect(natural.attributes('aria-checked')).toBe('true')
      await wrapper.findAll('button').find(button => button.text() === '保存采集配置')!.trigger('click')
      await flushPromises()
      expect(archiveAPI.saveConfig).toHaveBeenCalledWith(expect.objectContaining({ redact_scope: 'natural_language' }))
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }

    const { redact_scope: _omitted, ...older } = config
    vi.mocked(archiveAPI.getConfig).mockResolvedValue(older)
    const legacy = mount(GatewayLogArchiveView, { global: { stubs } })
    try {
      await flushPromises()
      expect(legacy.text()).not.toContain('仅自然语言')
    } finally {
      legacy.unmount()
    }
  })

  it('loads, searches, edits and explicitly clears excluded users without resending an untouched list', async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    let config: ArchiveConfig = {
      capture_enabled: true, mode: 'all', rules: [], success_only: true, redact_pii: true, excluded_user_ids: [7],
      max_file_size_mb: 10240, session_idle_seconds: 1800, max_body_mb: 64, keep_local_hours: 0, entry_retention_days: 90, min_free_disk_gb: 5,
    }
    vi.mocked(archiveAPI.getConfig).mockResolvedValue(config)
    vi.mocked(archiveAPI.saveConfig).mockImplementation(async input => (config = { ...config, ...input }))
    vi.mocked(archiveAPI.buckets).mockResolvedValue({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
    vi.mocked(searchUsers).mockResolvedValue([{ id: 7, email: 'existing@example.org', deleted: false }, { id: 12, email: 'user@example.org', deleted: false }])
    vi.mocked(getById).mockResolvedValue({ id: 31, email: 'other@example.org' } as AdminUser)
    const wrapper = mount(GatewayLogArchiveView, { global: {
      stubs: { ...stubs, teleport: true }, plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { common: { loading: '加载中' } } } })],
    } })
    const save = async () => {
      await wrapper.findAll('button').find(b => b.text() === '保存采集配置')!.trigger('click')
      await flushPromises()
      return vi.mocked(archiveAPI.saveConfig).mock.lastCall![0]
    }
    const search = async (query: string) => {
      await wrapper.get('[aria-label="添加排除用户"]').trigger('click')
      await wrapper.get('input[aria-label="搜索用户 ID、邮箱或用户名"]').setValue(query)
      await vi.advanceTimersByTimeAsync(310)
      await flushPromises()
    }
    try {
      await flushPromises()
      expect(wrapper.get('[aria-label="排除用户列表"]').text()).toContain('#7')
      expect(getById).not.toHaveBeenCalled()
      expect(await save()).not.toHaveProperty('excluded_user_ids')
      await search('example')
      expect(searchUsers).toHaveBeenCalledWith('example')
      expect(wrapper.findAll('[role="option"]')).toHaveLength(1)
      await wrapper.get('[role="option"]').trigger('click')
      await search('31')
      expect(getById).toHaveBeenCalledWith(31, true)
      await wrapper.get('[role="option"]').trigger('click')
      await wrapper.get('[aria-label="移除排除用户 7"]').trigger('click')
      expect((await save()).excluded_user_ids).toEqual([12, 31])
      await wrapper.get('[aria-label="移除排除用户 12"]').trigger('click')
      await wrapper.get('[aria-label="移除排除用户 31"]').trigger('click')
      expect((await save()).excluded_user_ids).toEqual([])
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

describe('gateway log archive upload progress and stuck bundles', () => {
  const baseStatus = (uploading: ArchiveStatus['uploading']): ArchiveStatus => ({
    module_enabled: true, running: true, capturing: true, reason: '', node_id: 'node-a', data_dir: '/app/data', today: '2026-11-24',
    disk_free_bytes: 0, storage_buckets: 1, uploaded_bundles: 0, uploaded_bytes: 0, active_sessions: 0, queued_exchanges: 0,
    pending_flushes: 0, open_bundles: [], uploading, captured: 0, skipped_model: 0, skipped_status: 0, dropped: 0,
    sessions_archived: 0, flush_errors: 0, uploaded: 0, upload_errors: 0, last_error: '', last_error_at: null, bundles: null,
  })
  const row = (overrides: Partial<Bundle>): Bundle => ({
    id: 1, node_id: 'node-a', day: '2026-11-22', model_key: 'gpt-6-astra', part: 4, file_name: 'gpt-6-astra-2026-11-22-part004.jsonl',
    status: 'uploading', size_bytes: 10 * 2 ** 30, entry_count: 300, bucket: '', bucket_id: null, object_key: '', attempts: 3,
    last_error: '', local_deleted: false, created_at: '', sealed_at: '2026-11-23T00:00:01Z', uploaded_at: null, ...overrides,
  })

  it('shows a progress bar while loading, idle, and uploading an unknown size', async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    vi.mocked(archiveAPI.buckets).mockResolvedValue({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
    vi.mocked(archiveAPI.getConfig).mockResolvedValue({
      capture_enabled: true, mode: 'all', rules: [], success_only: true, redact_pii: false,
      max_file_size_mb: 10240, session_idle_seconds: 1800, max_body_mb: 64, keep_local_hours: 0, entry_retention_days: 90, min_free_disk_gb: 5,
    })
    vi.mocked(archiveAPI.status)
      .mockResolvedValueOnce(baseStatus(null))
      .mockResolvedValueOnce(baseStatus({ bundle_id: 4, file_name: 'archive.jsonl', bucket: '', total: 0, sent: 4096 }))
      .mockResolvedValueOnce({ ...baseStatus(null), bundles: [{ status: 'uploading', count: 2, bytes: 1000 }] })
      .mockResolvedValue(baseStatus(null))
    const wrapper = mount(GatewayLogArchiveView, { global: { stubs } })
    const bar = () => wrapper.get('[aria-label="当前节点上传进度"]')
    try {
      expect(bar().attributes('aria-valuenow')).toBeUndefined()
      await flushPromises()
      expect(bar().attributes('aria-valuetext')).toBe('当前节点暂无上传任务')
      await vi.advanceTimersByTimeAsync(10000)
      expect(bar().attributes('aria-valuenow')).toBeUndefined()
      expect(bar().attributes('aria-valuetext')).toBe('总大小待确定')
      expect(wrapper.get('[data-testid="upload-progress"]').text()).toContain('4.00 KB')
      await vi.advanceTimersByTimeAsync(3000)
      expect(bar().attributes('aria-valuenow')).toBeUndefined()
      expect(bar().attributes('aria-valuetext')).toContain('2 个归档标为上传中')
      await vi.advanceTimersByTimeAsync(3000)
      expect(bar().attributes('aria-valuenow')).toBe('0')
      expect(wrapper.get('[data-testid="upload-progress"]').find('button').exists()).toBe(false)
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })

  it('shows the running upload with rate and deletes a stuck bundle after confirmation', async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-24T10:00:00Z'))
    vi.mocked(archiveAPI.getConfig).mockResolvedValue({ capture_enabled: true, mode: 'all', rules: [], success_only: true, redact_pii: true,
      max_file_size_mb: 10240, session_idle_seconds: 1800, max_body_mb: 64, keep_local_hours: 0, entry_retention_days: 90, min_free_disk_gb: 5 })
    vi.mocked(archiveAPI.buckets).mockResolvedValue({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
    const progress = (sent: number, body: number) => ({ bundle_id: 1, file_name: 'gpt-6-astra-2026-11-22-part004.jsonl', total: 10 * 2 ** 30,
      sent, body_bytes: body, part_retries: 2, phase: '发送分片，等待存储桶确认', started_at: '2026-11-24T09:50:00Z', attempt: 3, bucket: '远端 RustFS' })
    vi.mocked(archiveAPI.status)
      .mockResolvedValueOnce(baseStatus(progress(2 ** 30, 2 ** 30)))
      .mockResolvedValue(baseStatus(progress(2 ** 30, 2 ** 30 + 30 * 2 ** 20)))
    vi.mocked(archiveAPI.bundles).mockResolvedValue({
      items: [row({}), row({ id: 2, file_name: 'gpt-6-astra-2026-11-21-part001.jsonl', status: 'failed', last_error: 'read: connection reset by peer' }), row({ id: 3, status: 'uploaded', file_name: 'done.jsonl' })],
      total: 3, page: 1, page_size: 20,
    })
    vi.mocked(archiveAPI.deleteBundle).mockResolvedValue({ delete_requested: true, node_id: 'node-a', local: true })
    const wrapper = mount(GatewayLogArchiveView, { global: { stubs } })
    try {
      await flushPromises()
      const card = () => wrapper.get('[data-testid="upload-progress"]').text()
      expect(card()).toContain('正在上传 gpt-6-astra-2026-11-22-part004.jsonl → 远端 RustFS')
      expect(card()).toContain('10.00%')
      expect(card()).toContain('第 3 次尝试')
      expect(card()).toContain('本次已用 10 分 0 秒')
      // While uploading the status refreshes every 3 s; 30 MB in 3 s is 10 MB/s.
      await vi.advanceTimersByTimeAsync(3000)
      await flushPromises()
      expect(card()).toContain('速度 10.00 MB/s')
      expect(card()).toContain('预计剩余')

      await wrapper.findAll('[role="tab"]').find(tab => tab.text() === '归档文件')!.trigger('click')
      await flushPromises()
      const rows = wrapper.findAll('tbody tr')
      expect(rows[0].find('[data-testid="row-progress"]').text()).toContain('10%')
      expect(rows[2].findAll('button').some(b => b.text() === '删除')).toBe(false)
      await rows[1].findAll('button').find(b => b.text() === '删除')!.trigger('click')
      expect(archiveAPI.deleteBundle).not.toHaveBeenCalled()
      expect(wrapper.get('[data-testid="delete-dialog"]').text()).toContain('gpt-6-astra-2026-11-21-part001.jsonl')
      await wrapper.get('[data-testid="confirm-delete"]').trigger('click')
      await flushPromises()
      expect(archiveAPI.deleteBundle).toHaveBeenCalledWith(2)
      expect(wrapper.text()).toContain('已删除，正在中止上传并清理本地数据')
    } finally {
      wrapper.unmount()
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

describe('gateway log archive error text', () => {
  it('keeps the cause at the end of a long storage error', () => {
    const sdk = 'operation error S3: PutObject, exceeded maximum number of attempts, 3, https response error StatusCode: 0, RequestID: , HostID: , request send failed, Put "http://rustfs:9000/archive/2026-11-20/claude-2026-11-20-1.jsonl": dial tcp 172.18.0.5:9000: connect: connection refused'
    const shown = shortError(sdk)
    expect(shown.length).toBeLessThanOrEqual(243)
    expect(shown.startsWith('operation error S3: PutObject')).toBe(true)
    expect(shown.endsWith('connect: connection refused')).toBe(true)
    expect(shortError('短错误')).toBe('短错误')
  })
})

describe('gateway log archive model detection settings', () => {
  const detector = (): DetectorConfig => ({
    enabled: false, url: '', timeout_seconds: 120, min_confidence: 0.8, fail_open: false,
    kinds: ['email', 'phone', 'account', 'payment_card', 'ip_address', 'common_secret', 'private_address', 'private_person'],
  })

  it('saves detection settings and sends a key only when a new one is typed', async () => {
    vi.resetAllMocks()
    vi.useFakeTimers()
    const config: ArchiveConfig = {
      capture_enabled: false, mode: 'all', rules: [], success_only: true, redact_pii: true,
      max_file_size_mb: 10240, session_idle_seconds: 1800, max_body_mb: 64,
      keep_local_hours: 0, entry_retention_days: 90, min_free_disk_gb: 5,
      detector: detector(), detector_api_key_set: true,
    }
    vi.mocked(archiveAPI.getConfig).mockResolvedValue(config)
    vi.mocked(archiveAPI.saveConfig).mockImplementation(async value => ({ ...value, detector_api_key_set: true }))
    vi.mocked(archiveAPI.buckets).mockResolvedValue({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
    const wrapper = mount(GatewayLogArchiveView, { global: { stubs } })
    const save = async () => {
      await wrapper.findAll('button').find(button => button.text() === '保存采集配置')!.trigger('click')
      await flushPromises()
      return vi.mocked(archiveAPI.saveConfig).mock.lastCall![0]
    }
    try {
      await flushPromises()
      expect(wrapper.text()).toContain('已保存密钥，留空保持不变')
      await wrapper.get('[aria-label="启用模型脱敏服务"]').trigger('click')
      await wrapper.get('input[aria-label="脱敏服务地址"]').setValue('http://privacy-detector:8090')
      await wrapper.get('input[aria-label="识别日期"]').setValue(true)
      await wrapper.get('input[aria-label="置信度阈值"]').setValue(65)
      let sent = await save()
      expect(sent.detector).toMatchObject({ enabled: true, url: 'http://privacy-detector:8090', min_confidence: 0.65 })
      expect(sent.detector!.kinds).toContain('private_date')
      expect(sent.detector!.api_key).toBeUndefined()
      expect(sent).not.toHaveProperty('detector_api_key_set')

      await wrapper.get('input[aria-label="脱敏服务密钥"]').setValue('new-secret-key-123456')
      expect(wrapper.text()).toContain('保存后替换为新密钥')
      sent = await save()
      expect(sent.detector!.api_key).toBe('new-secret-key-123456')
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('tests unsaved settings and shows what the upload would contain', async () => {
    vi.resetAllMocks()
    vi.mocked(archiveAPI.testDetector).mockResolvedValue({
      ok: true, model: 'guard', elapsed_ms: 42, sample: '我叫王小明',
      redacted: '我叫[REDACTED]', findings: [{ label: 'private_person', score: 0.97, text: '王小明', accepted: true }],
    })
    const model = { ...detector(), enabled: true, url: 'http://d:8090' }
    const wrapper = mount(DetectorSettings, {
      props: { modelValue: model, keySet: false, status: { enabled: true, url: 'http://d:8090', requests: 3, texts: 10, bytes: 2048, cache_hits: 4, errors: 1, fail_open_skips: 0, last_error: '模型脱敏服务：返回 HTTP 503' } },
      global: { stubs },
    })
    await wrapper.findAll('button').find(button => button.text().includes('测试脱敏服务'))!.trigger('click')
    await flushPromises()
    expect(archiveAPI.testDetector).toHaveBeenCalledWith(expect.objectContaining({ url: 'http://d:8090', min_confidence: 0.8 }))
    const result = wrapper.get('[data-testid="detector-test-result"]').text()
    expect(result).toContain('连接正常，耗时 42 ms，模型 guard')
    expect(result).toContain('我叫[REDACTED]')
    expect(result).toContain('姓名 · 王小明 · 97%')
    const status = wrapper.get('[data-testid="detector-status"]').text()
    expect(status).toContain('本节点请求 3 次')
    expect(status).toContain('返回 HTTP 503')
  })
})
