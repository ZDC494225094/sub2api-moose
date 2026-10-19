import { apiClient } from '@/api/client'

export interface GuideConfig {
  title: string
  content_md: string
  updated_at: string | null
  is_default: boolean
}

export interface GuideAdminConfig extends GuideConfig {
  default_title: string
  default_content_md: string
}

export const guideAPI = {
  /** Public manual content for the docs page. */
  async getPublic(): Promise<GuideConfig> {
    const { data } = await apiClient.get('/guide')
    return data
  },
  async getConfig(): Promise<GuideAdminConfig> {
    const { data } = await apiClient.get('/admin/guide/config')
    return data
  },
  async saveConfig(title: string, content_md: string): Promise<GuideConfig> {
    const { data } = await apiClient.put('/admin/guide/config', { title, content_md })
    return data
  },
  async resetConfig(): Promise<GuideConfig> {
    const { data } = await apiClient.post('/admin/guide/config/reset')
    return data
  },
}
