import { describe, expect, it } from 'vitest'
import { firstImageDescription, mapClientPointToCanvas, wrapGalleryIndex } from '../playgroundImageTools'

describe('playground image tools', () => {
  it('maps scaled pointer coordinates to the canvas backing pixels', () => {
    expect(mapClientPointToCanvas(310, 170, {
      left: 10,
      top: 20,
      width: 600,
      height: 300
    }, 1200, 600)).toEqual({ x: 600, y: 300 })
  })

  it('clamps pointer coordinates to the canvas bounds', () => {
    expect(mapClientPointToCanvas(-20, 900, {
      left: 10,
      top: 20,
      width: 600,
      height: 300
    }, 1200, 600)).toEqual({ x: 0, y: 600 })
  })

  it('returns null for a hidden canvas', () => {
    expect(mapClientPointToCanvas(0, 0, {
      left: 0,
      top: 0,
      width: 0,
      height: 0
    }, 1200, 600)).toBeNull()
  })

  it('wraps gallery navigation in both directions', () => {
    expect(wrapGalleryIndex(3, 1, 4)).toBe(0)
    expect(wrapGalleryIndex(0, -1, 4)).toBe(3)
    expect(wrapGalleryIndex(1, 1, 4)).toBe(2)
  })

  it('uses one non-empty description for an image generation group', () => {
    expect(firstImageDescription([
      { revisedPrompt: '  ' },
      { revisedPrompt: 'Shared generation prompt' },
      { revisedPrompt: 'A different upstream revision' }
    ])).toBe('Shared generation prompt')
    expect(firstImageDescription([])).toBe('')
  })
})
