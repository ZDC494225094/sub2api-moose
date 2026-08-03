import { reactive } from 'vue'
import { describe, expect, it } from 'vitest'
import { toCloneablePlaygroundState } from '../playgroundPersistence'

describe('playground persistence', () => {
  it('converts nested Vue proxies into an IndexedDB-cloneable value', () => {
    const state = reactive({
      keySourceMode: 'manual',
      manualApiKey: 'shared-api-key',
      selectedModelsByMode: { image: 'gemini-3-pro-image-preview' },
      threads: [{
        pendingAttachments: [{
          id: 'attachment-1',
          kind: 'image',
          storageId: 'stored-image-1',
          dataUrl: 'playground-image://stored-image-1',
          thumbnailUrl: undefined
        }]
      }]
    })

    expect(() => structuredClone(state)).toThrow()

    const cloneable = toCloneablePlaygroundState(state)

    expect(cloneable).toEqual({
      keySourceMode: 'manual',
      manualApiKey: 'shared-api-key',
      selectedModelsByMode: { image: 'gemini-3-pro-image-preview' },
      threads: [{
        pendingAttachments: [{
          id: 'attachment-1',
          kind: 'image',
          storageId: 'stored-image-1',
          dataUrl: 'playground-image://stored-image-1'
        }]
      }]
    })
    expect(() => structuredClone(cloneable)).not.toThrow()
  })
})
