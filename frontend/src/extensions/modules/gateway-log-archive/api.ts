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
  updated_at?: string | null
}

export type BucketProvider = 'aws' | 's3-compatible'

export interface BucketUsage {
  bundle_count: number
  size_bytes: number
}

/** A storage destination; secrets are never returned, only whether they are set. */
export interface ArchiveBucket {
  id: number
  name: string
  provider: BucketProvider
  bucket: string
  region: string
  endpoint: string
  prefix: string
  force_path_style: boolean
  enabled: boolean
  priority: number
  /** Masked when read back; send a new value to replace it. */
  access_key_id: string
  secret_access_key_set: boolean
  session_token_set: boolean
  usage: BucketUsage
  created_at: string
  updated_at: string
}

export interface ArchiveBucketInput {
  id?: number
  name: string
  provider: BucketProvider
  bucket: string
  region: string
  endpoint: string
  prefix: string
  force_path_style: boolean
  enabled: boolean
  priority: number
  /** Empty or masked values keep the stored credential. */
  access_key_id?: string
  secret_access_key?: string
  session_token?: string
  clear_credentials?: boolean
  clear_session_token?: boolean
}

export interface BucketList {
  items: ArchiveBucket[]
  /** Totals of deleted buckets, kept so the overall total stays complete. */
  unattributed: BucketUsage
}

export interface ModelStat {
  model_key: string
  bundle_count: number
  entry_count: number
  size_bytes: number
  last_uploaded_at: string | null
}

export interface ModelStats {
  items: ModelStat[]
  total_bundles: number
  total_entries: number
  total_bytes: number
}

export interface TestResult {
  ok: boolean
  steps: { name: string; ok: boolean; error?: string }[]
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
  /** Enabled buckets at the last reconcile. */
  storage_buckets: number
  /** Cumulative uploads (not affected by index retention). */
  uploaded_bundles: number
  uploaded_bytes: number
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
  bucket_id: number | null
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
  async saveConfig(input: ArchiveConfig): Promise<ArchiveConfig> {
    const { data } = await apiClient.put(`${base}/config`, input)
    return data
  },
  async buckets(): Promise<BucketList> {
    const { data } = await apiClient.get(`${base}/buckets`)
    return data
  },
  async createBucket(input: ArchiveBucketInput): Promise<ArchiveBucket> {
    const { data } = await apiClient.post(`${base}/buckets`, input)
    return data
  },
  async updateBucket(id: number, input: ArchiveBucketInput): Promise<ArchiveBucket> {
    const { data } = await apiClient.put(`${base}/buckets/${id}`, input)
    return data
  },
  async deleteBucket(id: number): Promise<void> {
    await apiClient.delete(`${base}/buckets/${id}`)
  },
  /** Tests unsaved form values; with an id, stored secrets fill empty fields. */
  async testBucket(input: ArchiveBucketInput): Promise<TestResult> {
    const { data } = await apiClient.post(`${base}/buckets/test`, input, { timeout: 60000 })
    return data
  },
  async modelStats(bucketId?: number): Promise<ModelStats> {
    const { data } = await apiClient.get(`${base}/stats/models`, { params: { bucket_id: bucketId || undefined } })
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
