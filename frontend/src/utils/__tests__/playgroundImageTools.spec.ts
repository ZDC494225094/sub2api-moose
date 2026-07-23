import { describe, expect, it } from 'vitest'
import { firstActualImageSize, firstImageDescription, fitGptImage2Size, mapClientPointToCanvas, wrapGalleryIndex } from '../playgroundImageTools'

describe('playground image tools', () => {
  it('uses decoded image dimensions instead of the requested size', () => {
    expect(firstActualImageSize([{ width: 1086, height: 1448 }])).toBe('1086x1448')
    expect(firstActualImageSize([{ width: 0, height: 0 }, { width: 2048, height: 1152 }])).toBe('2048x1152')
  })

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

  it('fits gpt-image-2 sizes within the upstream 4K pixel limits', () => {
    expect(fitGptImage2Size('gpt-image-2', '4096x4096')).toBe('2880x2880')
    expect(fitGptImage2Size('gpt-image-2', '4096x2304')).toBe('3840x2160')
    expect(fitGptImage2Size('gpt-image-2', '4096x3072')).toBe('3312x2480')
    expect(fitGptImage2Size('gpt-image-2', '256x256')).toBe('816x816')
    expect(fitGptImage2Size('gpt-image-2', '4096x256')).toBe('3840x1280')
    expect(fitGptImage2Size('gpt-image-1.5', '4096x4096')).toBe('4096x4096')
  })
})
