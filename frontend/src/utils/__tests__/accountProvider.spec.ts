import { describe, expect, it } from 'vitest'

import {
  GITEE_AI_PROVIDER,
  getAccountUpstreamProvider,
  isGiteeAIAccount,
  isGiteeAIBaseUrl
} from '../accountProvider'

describe('accountProvider', () => {
  it('recognizes Gitee AI from explicit provider metadata', () => {
    const account = {
      platform: 'openai' as const,
      credentials: { base_url: 'https://proxy.example.com/v1' },
      extra: { upstream_provider: 'GITEE' }
    }

    expect(getAccountUpstreamProvider(account)).toBe(GITEE_AI_PROVIDER)
    expect(isGiteeAIAccount(account)).toBe(true)
  })

  it('recognizes historical OpenAI-compatible accounts by the exact Gitee hostname', () => {
    expect(isGiteeAIBaseUrl('https://ai.gitee.com/v1/')).toBe(true)
    expect(isGiteeAIAccount({
      platform: 'openai',
      credentials: { base_url: 'https://ai.gitee.com/v1' }
    })).toBe(true)
  })

  it('does not match lookalike hosts or non-OpenAI platforms', () => {
    expect(isGiteeAIBaseUrl('https://ai.gitee.com.example.org/v1')).toBe(false)
    expect(isGiteeAIAccount({
      platform: 'anthropic',
      credentials: { base_url: 'https://ai.gitee.com/v1' }
    })).toBe(false)
  })
})
