import { apiClient } from '@/api/client'

export interface Config {
  bot_token: string
  bot_username: string
  bot_id: number
  receive_enabled: boolean
  push_enabled: boolean
  chat_ids: string[]
  interval_minutes: number
  push_template: string
}
export interface Rule {
  id: number
  keyword: string
  match_type: 'contains' | 'exact' | 'command'
  reply_template: string
  enabled: boolean
  sort: number
}
export interface BotMessage {
  id: number; platform: string; chat_id: string; chat_type: string; sender_name: string
  sender_id: string; text: string; rule_id: number | null; replied: boolean; created_at: string
}
export interface Status {
  bot: { id: number; username: string }
  polling: boolean; pushing: boolean; last_error: string; last_error_at: string | null
}
const base = '/admin/bot-notify'
export const botAPI = {
  async config() { return (await apiClient.get<Config>(`${base}/config`)).data },
  async saveConfig(config: Config) { return (await apiClient.put<Config>(`${base}/config`, config)).data },
  async status() { return (await apiClient.get<Status>(`${base}/status`)).data },
  async test(chat_id: string) { await apiClient.post(`${base}/test-send`, { chat_id }) },
  async messages(page: number, chat_id: string, keyword: string) { return (await apiClient.get<{ items: BotMessage[]; total: number }>(`${base}/messages`, { params: { page, page_size: 20, chat_id, keyword } })).data },
  async rules() { return (await apiClient.get<{ items: Rule[] }>(`${base}/rules`)).data.items },
  async saveRule(rule: Rule) { if (rule.id) await apiClient.put(`${base}/rules/${rule.id}`, rule); else await apiClient.post(`${base}/rules`, rule) },
  async deleteRule(id: number) { await apiClient.delete(`${base}/rules/${id}`) },
}
