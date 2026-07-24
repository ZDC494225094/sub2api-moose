export interface CanvasPoint {
  x: number
  y: number
}

export interface CanvasClientRect {
  left: number
  top: number
  width: number
  height: number
}

export interface ImageDescriptionLike {
  revisedPrompt?: unknown
}

export interface ImageDimensionsLike {
  width?: unknown
  height?: unknown
}

export type GptImage2Resolution = '1K' | '2K' | '4K'
export type GptImage2AspectRatio = '1:1' | '16:9' | '9:16' | '2:1' | '1:2' | '21:9' | '9:21'

const GPT_IMAGE_2_SIZE_TABLE: Record<GptImage2AspectRatio, Partial<Record<GptImage2Resolution, string>>> = {
  '1:1': { '1K': '1024x1024', '2K': '2048x2048' },
  '16:9': { '1K': '1536x864', '2K': '2048x1152', '4K': '3840x2160' },
  '9:16': { '1K': '864x1536', '2K': '1152x2048', '4K': '2160x3840' },
  '2:1': { '1K': '2048x1024', '2K': '2688x1344', '4K': '3840x1920' },
  '1:2': { '1K': '1024x2048', '2K': '1344x2688', '4K': '1920x3840' },
  '21:9': { '1K': '2016x864', '2K': '2688x1152', '4K': '3840x1648' },
  '9:21': { '1K': '864x2016', '2K': '1152x2688', '4K': '1648x3840' }
}

export function isGptImage2Model(model: string): boolean {
  const normalized = model.trim().toLowerCase()
  return normalized === 'gpt-image-2' || normalized.startsWith('gpt-image-2-')
}

export function gptImage2SizeFor(resolution: GptImage2Resolution, ratio: string): string | null {
  const sizes = GPT_IMAGE_2_SIZE_TABLE[ratio as GptImage2AspectRatio]
  return sizes?.[resolution] || null
}

export function firstActualImageSize(images?: readonly ImageDimensionsLike[]): string {
  for (const image of images || []) {
    const width = Number(image.width)
    const height = Number(image.height)
    if (!Number.isFinite(width) || width <= 0 || !Number.isFinite(height) || height <= 0) continue
    return `${Math.round(width)}x${Math.round(height)}`
  }
  return ''
}

export function mapClientPointToCanvas(
  clientX: number,
  clientY: number,
  rect: CanvasClientRect,
  canvasWidth: number,
  canvasHeight: number
): CanvasPoint | null {
  if (rect.width <= 0 || rect.height <= 0 || canvasWidth <= 0 || canvasHeight <= 0) return null
  return {
    x: Math.min(Math.max((clientX - rect.left) * canvasWidth / rect.width, 0), canvasWidth),
    y: Math.min(Math.max((clientY - rect.top) * canvasHeight / rect.height, 0), canvasHeight)
  }
}

export function wrapGalleryIndex(index: number, direction: -1 | 1, total: number): number {
  if (total <= 0) return 0
  return (index + direction + total) % total
}

export function firstImageDescription(images: ImageDescriptionLike[] | undefined): string {
  if (!images?.length) return ''
  for (const image of images) {
    if (typeof image.revisedPrompt !== 'string') continue
    const description = image.revisedPrompt.trim()
    if (description) return description
  }
  return ''
}
