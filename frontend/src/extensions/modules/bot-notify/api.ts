import { apiClient } from '@/api/client'

export interface BotEntry {
  id: number
  name: string
  platform: string
  bot_token: string
  bot_username: string
  bot_id: number
  receive_enabled: boolean
  push_enabled: boolean
  chat_ids: string[]
  interval_minutes: number
  push_template: string
  app_id?: string
  announce_sync_enabled: boolean
}
export interface Rule {
  id: number
  bot_id: number
  keyword: string
  match_type: 'contains' | 'exact' | 'command'
  reply_template: string
  enabled: boolean
  sort: number
}
export interface BotMessage {
  id: number; bot_row_id: number; platform: string; chat_id: string; chat_type: string
  sender_name: string; sender_id: string; text: string; rule_id: number | null
  replied: boolean; reply_content: string; image_count?: number; created_at: string
}
export interface Status {
  bot: { id: number; username: string }
  polling: boolean; pushing: boolean; announcing: boolean; last_error: string; last_error_at: string | null
}
/** AI 客服配置只保存 API Key 引用，绝不回传密钥明文。 */
export interface AiConfig {
  enabled: boolean
  api_key_id: string | number | null
  model: string
  max_input_chars: number
  max_output_tokens: number
  cooldown_seconds: number
  daily_limit: number
  allow_recharge: boolean
  allow_technical: boolean
  /** 推理强度：'' 交给模型默认，low 回答最快。 */
  reasoning_effort: '' | 'low' | 'medium' | 'high'
  allow_images: boolean
  /** 允许回答公开分组近 24 小时整体缓存命中率（全站聚合，不含个人数据）。 */
  allow_usage_stats: boolean
  /** 把文档中心的使用手册提供给 AI 参考。 */
  use_user_guide: boolean
  /** 站点资料；保存内置默认内容时后端存为空，继续跟随默认更新。 */
  knowledge_base: string
  /** 仅读取：当前是否在用内置默认资料。 */
  knowledge_base_is_default?: boolean
  /** 仅读取：内置默认资料，用于「恢复默认」。 */
  default_knowledge_base?: string
}
export interface AiTestResult {
  answer: string
  blocked: boolean
  reason?: string
  error_code?: string
  latency_ms: number
  input_tokens: number
  output_tokens: number
  tools: string[]
  images_dropped?: boolean
}
export interface AiKeyOption {
  id: string | number
  name: string
  platform?: string
  status?: string
  models?: string[]
}
export interface AiRecord {
  id: number | string
  created_at: string
  chat_id: string
  sender_name?: string
  question: string
  answer?: string
  blocked: boolean
  blocked_reason?: string
  model?: string
  latency_ms?: number
  error_code?: string
  input_tokens?: number
  output_tokens?: number
  image_count?: number
}
const base = '/admin/bot-notify'
export const botAPI = {
  async listBots() { return (await apiClient.get<{ items: BotEntry[] }>(`${base}/bots`)).data.items },
  async createBot(entry: Partial<BotEntry>) { return (await apiClient.post<BotEntry>(`${base}/bots`, entry)).data },
  async getBot(id: number) { return (await apiClient.get<BotEntry>(`${base}/bots/${id}`)).data },
  async saveBot(id: number, entry: BotEntry) { return (await apiClient.put<BotEntry>(`${base}/bots/${id}`, entry)).data },
  async deleteBot(id: number) { await apiClient.delete(`${base}/bots/${id}`) },
  async status(id: number) { return (await apiClient.get<Status>(`${base}/bots/${id}/status`)).data },
  async test(id: number, chat_id: string) { await apiClient.post(`${base}/bots/${id}/test-send`, { chat_id }) },
  async pushNow(id: number) { await apiClient.post(`${base}/bots/${id}/push-now`) },
  async messages(id: number, page: number, chat_id: string, keyword: string) { return (await apiClient.get<{ items: BotMessage[]; total: number }>(`${base}/bots/${id}/messages`, { params: { page, page_size: 20, chat_id, keyword } })).data },
  async rules(id: number) { return (await apiClient.get<{ items: Rule[] }>(`${base}/bots/${id}/rules`)).data.items },
  async saveRule(botId: number, rule: Rule) { if (rule.id) await apiClient.put(`${base}/bots/${botId}/rules/${rule.id}`, rule); else await apiClient.post(`${base}/bots/${botId}/rules`, rule) },
  async deleteRule(botId: number, ruleId: number) { await apiClient.delete(`${base}/bots/${botId}/rules/${ruleId}`) },
  async aiConfig(botId: number) { return (await apiClient.get<AiConfig>(`${base}/bots/${botId}/ai/config`)).data },
  async saveAiConfig(botId: number, config: AiConfig) { return (await apiClient.put<AiConfig>(`${base}/bots/${botId}/ai/config`, config)).data },
  async aiKeys(botId: number) { return (await apiClient.get<{ items: AiKeyOption[] }>(`${base}/bots/${botId}/ai/keys`)).data.items },
  async aiRecords(botId: number, page: number, keyword = '', blocked = '') {
    return (await apiClient.get<{ items: AiRecord[]; total: number }>(`${base}/bots/${botId}/ai/records`, { params: { page, page_size: 20, keyword, blocked } })).data
  },
  /** 用已保存的配置试答一次：不发送到任何聊天、不记录，模型调用按所选 Key 计费。 */
  async aiTest(botId: number, text: string, image = '') {
    return (await apiClient.post<AiTestResult>(`${base}/bots/${botId}/ai/test`, { text, image }, { timeout: 240000 })).data
  },
}
