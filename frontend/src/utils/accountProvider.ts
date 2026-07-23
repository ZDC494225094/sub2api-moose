import type { Account } from '@/types'

export const GITEE_AI_BASE_URL = 'https://ai.gitee.com/v1'
export const GITEE_AI_PROVIDER = 'gitee'
export const UPSTREAM_PROVIDER_EXTRA_KEY = 'upstream_provider'

type ProviderAccount = Pick<Account, 'platform'> & {
  credentials?: Record<string, unknown>
  extra?: Record<string, unknown>
}

export function isGiteeAIBaseUrl(value: unknown): boolean {
  if (typeof value !== 'string' || !value.trim()) return false

  try {
    return new URL(value.trim()).hostname.toLowerCase() === 'ai.gitee.com'
  } catch {
    return false
  }
}

export function getAccountUpstreamProvider(account?: ProviderAccount | null): string | undefined {
  if (!account || account.platform !== 'openai') return undefined

  const provider = account.extra?.[UPSTREAM_PROVIDER_EXTRA_KEY]
  if (typeof provider === 'string' && provider.trim()) {
    return provider.trim().toLowerCase()
  }

  return isGiteeAIBaseUrl(account.credentials?.base_url) ? GITEE_AI_PROVIDER : undefined
}

export function isGiteeAIAccount(account?: ProviderAccount | null): boolean {
  return getAccountUpstreamProvider(account) === GITEE_AI_PROVIDER
}
