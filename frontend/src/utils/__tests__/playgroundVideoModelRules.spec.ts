import { describe, expect, it } from 'vitest'
import {
  normalizePlaygroundVideoDuration,
  playgroundVideoModelKind,
  playgroundVideoModelRule,
  validateGeminiOmniPrompt
} from '../playgroundVideoModelRules'

describe('playgroundVideoModelRules', () => {
  it('limits grok-video-10 config and downgrades multi-reference duration', () => {
    expect(playgroundVideoModelRule('grok-video-10', 1).durations).toEqual([6, 10, 16])
    expect(playgroundVideoModelRule('grok-video-10', 2).durations).toEqual([6, 10])
    expect(playgroundVideoModelRule('grok-video-10').resolutions).toEqual(['480p', '720p'])
    expect(playgroundVideoModelRule('grok-video-10').aspectRatios).toEqual([
      '16:9', '9:16', '4:3', '3:4', '2:3', '3:2', '1:1'
    ])
    expect(normalizePlaygroundVideoDuration('grok-video-10', 16, 2)).toBe(10)
  })

  it('exposes the full grok-video-r duration range and reference limit', () => {
    const rule = playgroundVideoModelRule('vendor/grok-video-r')
    expect(rule.durations[0]).toBe(6)
    expect(rule.durations.at(-1)).toBe(30)
    expect(rule.durations).toHaveLength(25)
    expect(rule.maxReferenceImages).toBe(7)
  })

  it('limits Gemini Omni media and detects forbidden prompt content', () => {
    const rule = playgroundVideoModelRule('gemini-omni-flash-preview')
    expect(rule.maxReferenceImages).toBe(5)
    expect(rule.maxReferenceVideos).toBe(1)
    expect(rule.durations.at(-1)).toBe(10)
    expect(validateGeminiOmniPrompt('电影画面，比例 16:9')).toBe('ratio')
    expect(validateGeminiOmniPrompt('镜头持续 8 秒')).toBe('duration')
    expect(validateGeminiOmniPrompt('分镜 1：人物走入画面')).toBe('storyboard')
    expect(validateGeminiOmniPrompt('人物缓慢走入雨夜街道')).toBeNull()
  })

  it('exposes Kling and Seedance frame-pair duration rules', () => {
    const kling = playgroundVideoModelRule('kling-v1-6')
    expect(kling.kind).toBe('kling')
    expect(kling.durations).toEqual([5, 10, 15])
    expect(kling.supportsFramePair).toBe(true)
    expect(normalizePlaygroundVideoDuration('keling-v1-6', 8)).toBe(10)

    const seedance20 = playgroundVideoModelRule('doubao-seedance-2.0')
    expect(seedance20.kind).toBe('seedance-2.0')
    expect(seedance20.durations[0]).toBe(4)
    expect(seedance20.durations.at(-1)).toBe(15)
    expect(seedance20.supportsFramePair).toBe(true)

    const seedance25 = playgroundVideoModelRule('seedance-2-5-pro')
    expect(seedance25.kind).toBe('seedance-2.5')
    expect(seedance25.durations[0]).toBe(4)
    expect(seedance25.durations.at(-1)).toBe(30)
  })

  it('recognizes supported model aliases without changing unknown models', () => {
    expect(playgroundVideoModelKind('models/grok-video-10-fast')).toBe('grok-video-10')
    expect(playgroundVideoModelKind('grok-video-r')).toBe('grok-video-r')
    expect(playgroundVideoModelKind('kling-v1-6')).toBe('kling')
    expect(playgroundVideoModelKind('doubao-seedance-2.0')).toBe('seedance-2.0')
    expect(playgroundVideoModelKind('doubao-seedance-2.5')).toBe('seedance-2.5')
    expect(playgroundVideoModelKind('other-video')).toBe('default')
  })
})
