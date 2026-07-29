export type PlaygroundVideoResolution = '480p' | '720p' | '1080p'

export type PlaygroundVideoModelKind =
  | 'grok-video-10'
  | 'grok-video-r'
  | 'gemini-omni-flash'
  | 'default'

export type GeminiOmniPromptViolation = 'ratio' | 'duration' | 'storyboard'

export interface PlaygroundVideoModelRule {
  kind: PlaygroundVideoModelKind
  durations: number[]
  resolutions: PlaygroundVideoResolution[]
  aspectRatios: string[]
  maxReferenceImages: number | null
  maxReferenceVideos: number
}

const DEFAULT_DURATIONS = [4, 5, 6, 8, 10]
const DEFAULT_RESOLUTIONS: PlaygroundVideoResolution[] = ['480p', '720p', '1080p']
const DEFAULT_ASPECT_RATIOS = ['16:9', '9:16', '1:1']
const GROK_VIDEO_10_ASPECT_RATIOS = ['16:9', '9:16', '4:3', '3:4', '2:3', '3:2', '1:1']

function integerRange(min: number, max: number): number[] {
  return Array.from({ length: max - min + 1 }, (_, index) => min + index)
}

export function playgroundVideoModelKind(model: string): PlaygroundVideoModelKind {
  const normalized = model.trim().toLowerCase()
  if (/(^|\/)grok-video-10(?:$|[-_:])/.test(normalized)) return 'grok-video-10'
  if (/(^|\/)grok-video-r(?:$|[-_:])/.test(normalized)) return 'grok-video-r'
  if (/(^|\/)gemini-omni-flash(?:$|[-_:])/.test(normalized)) return 'gemini-omni-flash'
  return 'default'
}

export function playgroundVideoModelRule(model: string, referenceImageCount = 0): PlaygroundVideoModelRule {
  const kind = playgroundVideoModelKind(model)
  switch (kind) {
    case 'grok-video-10':
      return {
        kind,
        durations: referenceImageCount > 1 ? [6, 10] : [6, 10, 16],
        resolutions: ['480p', '720p'],
        aspectRatios: GROK_VIDEO_10_ASPECT_RATIOS,
        maxReferenceImages: null,
        maxReferenceVideos: 0
      }
    case 'grok-video-r':
      return {
        kind,
        durations: integerRange(6, 30),
        resolutions: DEFAULT_RESOLUTIONS,
        aspectRatios: DEFAULT_ASPECT_RATIOS,
        maxReferenceImages: 7,
        maxReferenceVideos: 0
      }
    case 'gemini-omni-flash':
      return {
        kind,
        durations: integerRange(1, 10),
        resolutions: DEFAULT_RESOLUTIONS,
        aspectRatios: DEFAULT_ASPECT_RATIOS,
        maxReferenceImages: 5,
        maxReferenceVideos: 1
      }
    default:
      return {
        kind,
        durations: DEFAULT_DURATIONS,
        resolutions: DEFAULT_RESOLUTIONS,
        aspectRatios: DEFAULT_ASPECT_RATIOS,
        maxReferenceImages: 1,
        maxReferenceVideos: 0
      }
  }
}

export function normalizePlaygroundVideoDuration(model: string, duration: number, referenceImageCount = 0): number {
  const rule = playgroundVideoModelRule(model, referenceImageCount)
  if (rule.durations.includes(duration)) return duration
  const min = rule.durations[0]
  const max = rule.durations[rule.durations.length - 1]
  if (duration < min) return min
  if (duration > max) return max
  return rule.kind === 'grok-video-10' ? 10 : rule.durations.reduce((closest, candidate) => (
    Math.abs(candidate - duration) < Math.abs(closest - duration) ? candidate : closest
  ), rule.durations[0])
}

export function validateGeminiOmniPrompt(prompt: string): GeminiOmniPromptViolation | null {
  if (/(?:16\s*[:：./比]\s*9|9\s*[:：./比]\s*16)/i.test(prompt)) return 'ratio'
  if (/(?:\b\d+(?:\.\d+)?\s*(?:s|sec|secs|second|seconds)\b|\d+(?:\.\d+)?\s*(?:秒|秒钟))/i.test(prompt)) return 'duration'
  if (/(?:故事板|故事版|分镜|story\s*board|storyboard|(?:shot|scene)\s*\d+)/i.test(prompt)) return 'storyboard'
  return null
}
