import { describe, expect, it } from 'vitest'
import {
  firstActualImageSize,
  firstImageDescription,
  gptImage2SizeFor,
  isGptImage2Model,
  mapClientPointToCanvas,
  wrapGalleryIndex
} from '../playgroundImageTools'

describe('playground image tools', () => {
  it('uses decoded image dimensions instead of the requested size', () => {
    expect(firstActualImageSize([{ width: 1086, height: 1448 }])).toBe('1086x1448')
    expect(firstActualImageSize([{ width: 0, height: 0 }, { width: 2048, height: 1152 }])).toBe('2048x1152')
  })

  it.each([
    ['1:1', '1K', '1024x1024'],
    ['1:1', '2K', '2048x2048'],
    ['16:9', '1K', '1536x864'],
    ['16:9', '2K', '2048x1152'],
    ['16:9', '4K', '3840x2160'],
    ['9:16', '1K', '864x1536'],
    ['9:16', '2K', '1152x2048'],
    ['9:16', '4K', '2160x3840'],
    ['2:1', '1K', '2048x1024'],
    ['2:1', '2K', '2688x1344'],
    ['2:1', '4K', '3840x1920'],
    ['1:2', '1K', '1024x2048'],
    ['1:2', '2K', '1344x2688'],
    ['1:2', '4K', '1920x3840'],
    ['21:9', '1K', '2016x864'],
    ['21:9', '2K', '2688x1152'],
    ['21:9', '4K', '3840x1648'],
    ['9:21', '1K', '864x2016'],
    ['9:21', '2K', '1152x2688'],
    ['9:21', '4K', '1648x3840']
  ] as const)('maps GPT Image 2 %s %s to %s', (ratio, resolution, expected) => {
    expect(gptImage2SizeFor(resolution, ratio)).toBe(expected)
  })

  it('marks GPT Image 2 square 4K and unknown ratios as unsupported', () => {
    expect(gptImage2SizeFor('4K', '1:1')).toBeNull()
    expect(gptImage2SizeFor('4K', '4:3')).toBeNull()
    expect(isGptImage2Model('gpt-image-2')).toBe(true)
    expect(isGptImage2Model('gpt-image-2-2026-04-21')).toBe(true)
    expect(isGptImage2Model('gpt-image-1.5')).toBe(false)
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

})
