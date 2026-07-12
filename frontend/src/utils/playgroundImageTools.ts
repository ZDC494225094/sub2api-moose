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
