import { apiClient } from '@/api/client'

export interface ModelRule {
  pattern: string
  alias: string
}

export interface ArchiveConfig {
  capture_enabled: boolean
  mode: 'all' | 'selected'
  rules: ModelRule[]
  success_only: boolean
  max_zip_size_mb: number
  session_idle_seconds: number
  max_body_mb: number
  keep_local_hours: number
  entry_retention_days: number
  min_free_disk_gb: number
  provider: 'aws' | 's3-compatible'
  bucket: string
  region: string
  endpoint: string
  prefix: string
  force_path_style: boolean
  /** Masked when read back; send a new value to replace it. */
  access_key_id: string
  secret_access_key_set?: boolean
  session_token_set?: boolean
  updated_at?: string | null
}

export interface ArchiveConfigInput extends ArchiveConfig {
  secret_access_key?: string
  session_token?: string
  clear_credentials?: boolean
  clear_session_token?: boolean
}

export interface OpenBundle {
  id: number
  day: string
  model_key: string
  file_name: string
  size_bytes: number
  entry_count: number
}

export interface ArchiveStatus {
  module_enabled: boolean
  running: boolean
  capturing: boolean
  reason: string
  node_id: string
  data_dir: string
  today: string
  disk_free_bytes: number
  active_sessions: number
  queued_exchanges: number
  pending_flushes: number
  open_bundles: OpenBundle[]
  uploading: { bundle_id: number; file_name: string; total: number; sent: number } | null
  captured: number
  skipped_model: number
  skipped_status: number
  dropped: number
  sessions_archived: number
  flush_errors: number
  uploaded: number
  upload_errors: number
  last_error: string
  last_error_at: string | null
  bundles: { status: string; count: number; bytes: number }[] | null
}

export interface Bundle {
  id: number
  node_id: string
  day: string
  model_key: string
  part: number
  file_name: string
  status: 'open' | 'sealed' | 'uploading' | 'uploaded' | 'failed'
  size_bytes: number
  entry_count: number
  bucket: string
  object_key: string
  attempts: number
  last_error: string
  local_deleted: boolean
  created_at: string
  sealed_at: string | null
  uploaded_at: string | null
}

export interface Entry {
  id: number
  bundle_id: number
  seq: number
  entry_name: string
  session_id: string
  model: string
  platform: string
  user_id: number
  api_key_id: number
  exchange_count: number
  compressed_size: number
  uncompressed_size: number
  started_at: string
  ended_at: string
  day: string
  file_name: string
  bundle_status: string
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface EntryContent {
  entry: Entry
  bundle: Bundle
  content: string
  truncated: boolean
  size: number
}

const base = '/admin/gateway-log-archive'

export const archiveAPI = {
  async getConfig(): Promise<ArchiveConfig> {
    const { data } = await apiClient.get(`${base}/config`)
    return data
  },
  async saveConfig(input: ArchiveConfigInput): Promise<ArchiveConfig> {
    const { data } = await apiClient.put(`${base}/config`, input)
    return data
  },
  async testConfig(input: ArchiveConfigInput): Promise<{ ok: boolean; steps: { name: string; ok: boolean; error?: string }[] }> {
    const { data } = await apiClient.post(`${base}/config/test`, input, { timeout: 60000 })
    return data
  },
  async status(): Promise<ArchiveStatus> {
    const { data } = await apiClient.get(`${base}/status`)
    return data
  },
  async flush(): Promise<ArchiveStatus> {
    const { data } = await apiClient.post(`${base}/flush`, {}, { timeout: 150000 })
    return data
  },
  async bundles(params: { page: number; page_size: number; day?: string; model?: string; status?: string }): Promise<Page<Bundle>> {
    const { data } = await apiClient.get(`${base}/bundles`, { params })
    return data
  },
  async retry(id: number): Promise<void> {
    await apiClient.post(`${base}/bundles/${id}/retry`)
  },
  async downloadURL(id: number): Promise<string> {
    const { data } = await apiClient.get(`${base}/bundles/${id}/download`)
    return data.url
  },
  async entries(params: {
    page: number
    page_size: number
    bundle_id?: number
    session_id?: string
    day?: string
    model?: string
    user_id?: number
  }): Promise<Page<Entry>> {
    const { data } = await apiClient.get(`${base}/entries`, { params })
    return data
  },
  async entryContent(id: number, limit = 1 << 20): Promise<EntryContent> {
    const { data } = await apiClient.get(`${base}/entries/${id}/content`, { params: { limit }, timeout: 120000 })
    return data
  },
  async downloadEntry(id: number): Promise<Blob> {
    const { data } = await apiClient.get(`${base}/entries/${id}/content`, {
      params: { download: 1 },
      responseType: 'blob',
      timeout: 300000,
    })
    return data
  },
}
