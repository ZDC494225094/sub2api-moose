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
  replied: boolean; reply_content: string; created_at: string
}
export interface Status {
  bot: { id: number; username: string }
  polling: boolean; pushing: boolean; announcing: boolean; last_error: string; last_error_at: string | null
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
}
