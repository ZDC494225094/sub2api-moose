import { apiClient } from '@/api/client'
import type { ExtensionId } from './catalog'

export interface ExtensionState {
  id: string
  name: string
  description: string
  managed: boolean
  enabled: boolean | null
  disable_behavior: string
  paths: string[]
  slots?: string[]
}

export const extensionAPI = {
  publicState: () => apiClient.get<{ enabled: Record<ExtensionId, boolean> }>('/custom-extensions'),
  list: () => apiClient.get<{ items: ExtensionState[] }>('/admin/custom-extensions'),
  update: (id: ExtensionId, enabled: boolean) =>
    apiClient.put<{ id: ExtensionId; enabled: boolean }>(`/admin/custom-extensions/${id}`, { enabled }),
}
