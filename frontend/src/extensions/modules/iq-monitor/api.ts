import { apiClient } from '@/api/client'

export type ResultStatus = 'pending' | 'running' | 'pass' | 'retry_pass' | 'fail' | 'error' | 'interrupted' | 'skipped'
export type Health = 'normal' | 'unstable' | 'degraded' | 'no_data'
export type TopicStatus = 'pending' | 'reviewing' | 'approved' | 'rejected' | 'failed' | 'used'
export type Kind = 'logic' | 'drawing'

export interface Stat { passed: number; total: number; rate: number }
export interface Cell {
  result_id: number
  round_id: number
  status: ResultStatus
  reason?: string
  truncated?: boolean
  total_ms?: number | null
  created_at: string
}
export interface KindView { stat: Stat; timeline: Cell[] }
export interface ModelCard {
  target_id: number
  display_name: string
  model: string
  group_id: number
  group_name: string
  platform: string
  rate_multiplier: number
  reasoning_effort: string
  sent_effort: string
  health: Health
  health_rate: Stat
  logic?: KindView
  drawing?: KindView
  avg_first_token_ms: number | null
  avg_total_ms: number | null
}
export interface WallCard {
  result_id: number
  target_id: number
  display_name: string
  model: string
  group_name: string
  platform: string
  rate_multiplier: number
  reasoning_effort: string
  status: ResultStatus
  reason?: string
  started_at: string | null
  total_ms: number | null
  output_tokens: number
  preview_url?: string
}
export interface WallView {
  round_id: number
  topic: string
  topic_source: 'user' | 'admin' | ''
  submitter?: string
  status: 'running' | 'done' | 'interrupted'
  started_at: string
  finished_at: string | null
  cards: WallCard[]
}
export interface Summary {
  models: number
  rounds_24h: number
  logic: Stat
  drawing: Stat
  normal: number
  unstable: number
  degraded: number
  queued_topics: number
  running: boolean
  last_round_at: string | null
  next_round_at: string | null
}
export interface Overview {
  logic_enabled: boolean
  drawing_enabled: boolean
  primary_kind: Kind
  interval_minutes: number
  thresholds: { normal: number; unstable: number }
  summary: Summary
  wall: WallView | null
  models: ModelCard[]
  queue: Array<{ topic: string; submitter: string; created_at: string }>
  timezone: string
  now: string
}
export interface Result {
  id: number
  round_id: number
  kind: Kind
  group_name: string
  model: string
  display_name: string
  reasoning_effort: string
  sent_effort: string
  status: ResultStatus
  reason: string
  attempts: number
  request_count: number
  input_tokens: number
  output_tokens: number
  first_token_ms: number | null
  total_ms: number | null
  truncated: boolean
  started_at: string | null
  finished_at: string | null
  created_at: string
}
export interface ResponseRow {
  id: number
  attempt: number
  content: string
  extracted: string
  correct: boolean
  finish_reason: string
  truncated: boolean
  error: string
  input_tokens: number
  output_tokens: number
  first_token_ms: number | null
  total_ms: number | null
  created_at: string
}
export interface ResultDetail {
  result: Result
  question?: string
  answers?: string[]
  topic?: string
  responses: ResponseRow[]
  preview_url?: string
}
export interface Topic {
  id: number
  user_id: number
  username?: string
  email?: string
  topic: string
  status: TopicStatus
  reject_reason: string
  attempts: number
  last_error?: string
  quota_day: string
  round_id: number | null
  created_at: string
  reviewed_at: string | null
  used_at: string | null
}
export interface MySubmissions {
  enabled: boolean
  quota: number
  used: number
  max_length: number
  day: string
  items: Topic[]
}

export interface Config {
  probe_user_id: number
  interval_minutes: number
  timeout_seconds: number
  request_retries: number
  max_output_tokens: number
  retention_days: number
  logic: { enabled: boolean; prompt_template: string; retry_wrong: number }
  drawing: { enabled: boolean; prompt_template: string; topics: string }
  thresholds: { normal: number; unstable: number }
  notify: { enabled: boolean; bot_ids: number[] }
  submissions: {
    enabled: boolean
    daily_quota: number
    max_length: number
    review_prompt: string
    review_api_key_id: number
    review_model: string
  }
  visibility: 'all' | 'allowlist'
}
export interface ConfigView extends Config {
  default_logic_prompt: string
  default_draw_prompt: string
  default_review_prompt: string
  default_topics: string
}
export interface Target {
  id: number
  group_id: number
  model: string
  display_name: string
  reasoning_effort: string
  api_key_id: number
  enabled: boolean
  sort_order: number
  sent_effort?: string
  effort_note?: string
  effort_denied?: boolean
}
export type TargetInput = Pick<Target, 'group_id' | 'model' | 'display_name' | 'reasoning_effort' | 'api_key_id' | 'enabled'>
export interface Question {
  id: number
  question: string
  answers: string[]
  enabled: boolean
  last_used_at: string | null
}
export interface GroupOption { id: number; name: string; platform: string; rate_multiplier: number; status: string }
export interface KeyOption { id: number; name: string; group_ids: number[]; status: string }
export interface UserOption { id: number; email: string; username: string }
export interface BotOption { id: number; name: string; platform: string; targets: number }
export interface Options {
  groups: GroupOption[]
  keys: KeyOption[]
  bots: BotOption[]
  probe_user: UserOption | null
}
export interface EffortOption { value: string; preview: { sent: string; denied: boolean; note?: string } }
export interface Round {
  id: number
  trigger: string
  status: string
  logic_enabled: boolean
  drawing_enabled: boolean
  topic: string
  topic_source: string
  question: string
  started_at: string
  finished_at: string | null
}
export interface RuntimeStatus {
  node_id: string
  local_running: boolean
  last_error: string
  running: Round | null
  latest: Round | null
  pending_topics: number
  queued_topics: number
  now: string
  next_round_at: string | null
}
export interface KindEstimate {
  samples: number
  avg_requests: number
  avg_input_tokens: number
  avg_output_tokens: number
  cost_per_run: number | null
}
export interface Estimate {
  rounds_per_day: number
  window_days: number
  tokens_daily: number
  cost_daily: number
  complete: boolean
  targets: Array<{
    target_id: number
    display_name: string
    model: string
    logic?: KindEstimate
    drawing?: KindEstimate
    tokens_daily: number
    cost_daily: number | null
  }>
}
export interface Viewer { user_id: number; email: string; username: string; note: string }

export const iqMonitorAPI = {
  async access(): Promise<boolean> {
    const { data } = await apiClient.get('/iq-monitor/access')
    return data.allowed === true
  },
  async overview(): Promise<Overview> {
    const { data } = await apiClient.get('/iq-monitor/overview')
    return data
  },
  async result(id: number): Promise<ResultDetail> {
    const { data } = await apiClient.get(`/iq-monitor/results/${id}`)
    return data
  },
  async submissions(): Promise<MySubmissions> {
    const { data } = await apiClient.get('/iq-monitor/submissions')
    return data
  },
  async submit(topic: string): Promise<Topic> {
    const { data } = await apiClient.post('/iq-monitor/submissions', { topic })
    return data
  },
}

const A = '/admin/iq-monitor'

export const iqMonitorAdminAPI = {
  async config(): Promise<ConfigView> {
    const { data } = await apiClient.get(`${A}/config`)
    return data
  },
  async saveConfig(cfg: Config): Promise<ConfigView> {
    const { data } = await apiClient.put(`${A}/config`, cfg)
    return data
  },
  async status(): Promise<RuntimeStatus> {
    const { data } = await apiClient.get(`${A}/status`)
    return data
  },
  async trigger(): Promise<number> {
    const { data } = await apiClient.post(`${A}/trigger`)
    return data.round_id
  },
  async estimate(): Promise<Estimate> {
    const { data } = await apiClient.get(`${A}/estimate`)
    return data
  },
  async clearHistory(input: { all: true } | { all: false; group_id: number; model: string }): Promise<number> {
    const { data } = await apiClient.post(`${A}/history/clear`, input)
    return data.deleted
  },
  async options(): Promise<Options> {
    const { data } = await apiClient.get(`${A}/options`)
    return data
  },
  async models(keyId: number): Promise<{ models: string[]; error?: string }> {
    const { data } = await apiClient.get(`${A}/options/models`, { params: { key_id: keyId } })
    return data
  },
  async efforts(groupId: number, model: string): Promise<{ platform: string; items: EffortOption[] }> {
    const { data } = await apiClient.get(`${A}/options/efforts`, { params: { group_id: groupId, model } })
    return data
  },
  async users(q: string): Promise<UserOption[]> {
    const { data } = await apiClient.get(`${A}/options/users`, { params: { q } })
    return data.items ?? []
  },
  async targets(): Promise<{ items: Target[]; max: number }> {
    const { data } = await apiClient.get(`${A}/targets`)
    return data
  },
  async saveTarget(input: TargetInput, id?: number): Promise<void> {
    if (id) await apiClient.put(`${A}/targets/${id}`, input)
    else await apiClient.post(`${A}/targets`, input)
  },
  async deleteTarget(id: number): Promise<void> {
    await apiClient.delete(`${A}/targets/${id}`)
  },
  async reorderTargets(ids: number[]): Promise<void> {
    await apiClient.put(`${A}/targets-order`, { ids })
  },
  async questions(): Promise<Question[]> {
    const { data } = await apiClient.get(`${A}/questions`)
    return data.items ?? []
  },
  async saveQuestion(input: Pick<Question, 'question' | 'answers' | 'enabled'>, id?: number): Promise<void> {
    if (id) await apiClient.put(`${A}/questions/${id}`, input)
    else await apiClient.post(`${A}/questions`, input)
  },
  async deleteQuestion(id: number): Promise<void> {
    await apiClient.delete(`${A}/questions/${id}`)
  },
  async topics(params: { status?: string; page?: number; page_size?: number }): Promise<{ items: Topic[]; total: number }> {
    const { data } = await apiClient.get(`${A}/topics`, { params })
    return { items: data.items ?? [], total: data.total ?? 0 }
  },
  async deleteTopic(id: number): Promise<void> {
    await apiClient.delete(`${A}/topics/${id}`)
  },
  async viewers(): Promise<Viewer[]> {
    const { data } = await apiClient.get(`${A}/viewers`)
    return data.items ?? []
  },
  async addViewer(userId: number, note: string): Promise<void> {
    await apiClient.post(`${A}/viewers`, { user_id: userId, note })
  },
  async removeViewer(userId: number): Promise<void> {
    await apiClient.delete(`${A}/viewers/${userId}`)
  },
  async result(id: number): Promise<ResultDetail> {
    const { data } = await apiClient.get(`${A}/results/${id}`)
    return data
  },
}
