import { afterEach, describe, expect, it, vi } from 'vitest'

import { fetchModels } from '@/api/playground'

describe('playground API', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads models with only the explicitly supplied API key', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      object: 'list',
      data: [{ id: 'shared-model', owned_by: 'shared-account' }]
    }), {
      status: 200,
      headers: { 'content-type': 'application/json' }
    }))

    await expect(fetchModels('shared-api-key')).resolves.toEqual([{
      id: 'shared-model',
      label: 'shared-model',
      owned_by: 'shared-account'
    }])

    expect(fetchMock).toHaveBeenCalledWith('/v1/models', expect.objectContaining({
      method: 'GET',
      credentials: 'omit',
      headers: { Authorization: 'Bearer shared-api-key' }
    }))
  })
})
