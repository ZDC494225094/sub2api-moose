import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { HEX_ROW, hexCellAt, installPointerField } from '../pointerField'

function pointerMove(x: number, y: number, pointerType = 'mouse') {
  const event = new MouseEvent('pointermove', { clientX: x, clientY: y })
  Object.defineProperty(event, 'pointerType', { value: pointerType })
  window.dispatchEvent(event)
}

describe('glass honeycomb pointer field', () => {
  let frames: Map<number, FrameRequestCallback>
  let frameId: number
  let stop: (() => void) | undefined
  let layer: HTMLDivElement

  function flushFrame() {
    const callbacks = [...frames.values()]
    frames.clear()
    callbacks.forEach(callback => callback(0))
  }

  beforeEach(() => {
    frames = new Map()
    frameId = 0
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(callback => {
      frames.set(++frameId, callback)
      return frameId
    })
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(id => { frames.delete(id) })
    layer = document.createElement('div')
    layer.className = 'bg-mesh-gradient'
    vi.spyOn(layer, 'getBoundingClientRect').mockReturnValue({
      x: 100, y: 50, left: 100, top: 50, width: 500, height: 700, right: 600, bottom: 750,
      toJSON: () => ({}),
    })
    document.body.append(layer)
  })

  afterEach(() => {
    stop?.()
    stop = undefined
    layer.remove()
    vi.restoreAllMocks()
  })

  it('aligns cell centres with the scaled SVG grid, including negative coordinates', () => {
    expect(hexCellAt(12, 0)).toEqual({ x: 12, y: 0 })
    expect(hexCellAt(48, HEX_ROW / 2)).toEqual({ x: 48, y: HEX_ROW / 2 })
    expect(hexCellAt(-24, -HEX_ROW / 2)).toEqual({ x: -24, y: -HEX_ROW / 2 })
    expect(hexCellAt(68, HEX_ROW / 2)).toEqual({ x: 48, y: HEX_ROW / 2 })
    expect(hexCellAt(73, HEX_ROW / 2)).toEqual({ x: 84, y: HEX_ROW })
  })

  it('coalesces pointer moves and positions the spotlight relative to its layer', () => {
    stop = installPointerField(() => true)
    pointerMove(130, 70)
    pointerMove(148, 71)
    expect(frames.size).toBe(1)
    flushFrame()
    expect(layer.style.getPropertyValue('--ct-px')).toBe('48.0px')
    expect(layer.style.getPropertyValue('--ct-py')).toBe('21.0px')
    expect(layer.style.getPropertyValue('--ct-cx')).toBe('48.00px')
    expect(layer.style.getPropertyValue('--ct-cy')).toBe('20.78px')
    expect(layer.style.getPropertyValue('--ct-pointer')).toBe('1')
  })

  it('does no animation work for touch or while the theme is inactive', () => {
    let active = false
    stop = installPointerField(() => active)
    pointerMove(148, 71)
    expect(frames.size).toBe(0)
    active = true
    pointerMove(148, 71, 'touch')
    expect(frames.size).toBe(0)
    expect(layer.style.getPropertyValue('--ct-pointer')).toBe('')
  })

  it('fades out on blur and clears pending work and stale highlights on cleanup', () => {
    stop = installPointerField(() => true)
    pointerMove(148, 71)
    flushFrame()
    window.dispatchEvent(new Event('blur'))
    flushFrame()
    expect(layer.style.getPropertyValue('--ct-pointer')).toBe('0')

    pointerMove(160, 80)
    expect(frames.size).toBe(1)
    stop()
    stop = undefined
    expect(frames.size).toBe(0)
    expect(layer.getAttribute('style') || '').toBe('')
    pointerMove(170, 90)
    expect(frames.size).toBe(0)
  })

  it('tracks an auth backdrop mounted after the pointer runtime starts', () => {
    stop = installPointerField(() => true)
    layer.className = 'auth-layout-decor'
    pointerMove(148, 71)
    flushFrame()
    expect(layer.style.getPropertyValue('--ct-pointer')).toBe('1')
  })

  it('tracks the homepage and docs backdrop', () => {
    stop = installPointerField(() => true)
    layer.className = 'home-backdrop'
    pointerMove(148, 71)
    flushFrame()
    expect(layer.style.getPropertyValue('--ct-pointer')).toBe('1')
    expect(layer.style.getPropertyValue('--ct-px')).toBe('48.0px')
  })
})
