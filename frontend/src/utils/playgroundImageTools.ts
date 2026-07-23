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

export function firstActualImageSize(images?: readonly ImageDimensionsLike[]): string {
  for (const image of images || []) {
    const width = Number(image.width)
    const height = Number(image.height)
    if (!Number.isFinite(width) || width <= 0 || !Number.isFinite(height) || height <= 0) continue
    return `${Math.round(width)}x${Math.round(height)}`
  }
  return ''
}

const GPT_IMAGE_2_MAX_EDGE = 3840
const GPT_IMAGE_2_MIN_PIXELS = 655360
const GPT_IMAGE_2_MAX_PIXELS = 8294400
const GPT_IMAGE_2_MAX_ASPECT_RATIO = 3
const GPT_IMAGE_2_DIMENSION_STEP = 16

export function fitGptImage2Size(model: string, size: string): string {
  const normalizedModel = model.trim().toLowerCase()
  const normalizedSize = size.trim().toLowerCase()
  if (normalizedModel !== 'gpt-image-2' && !normalizedModel.startsWith('gpt-image-2-')) return size
  const match = /^(\d+)x(\d+)$/.exec(normalizedSize)
  if (!match) return size
  let width = Number(match[1])
  let height = Number(match[2])
  if (!Number.isSafeInteger(width) || !Number.isSafeInteger(height) || width <= 0 || height <= 0) return size
  const pixels = width * height
  const aspectRatio = Math.max(width / height, height / width)
  if (
    width <= GPT_IMAGE_2_MAX_EDGE &&
    height <= GPT_IMAGE_2_MAX_EDGE &&
    pixels >= GPT_IMAGE_2_MIN_PIXELS &&
    pixels <= GPT_IMAGE_2_MAX_PIXELS &&
    aspectRatio <= GPT_IMAGE_2_MAX_ASPECT_RATIO &&
    width % GPT_IMAGE_2_DIMENSION_STEP === 0 &&
    height % GPT_IMAGE_2_DIMENSION_STEP === 0
  ) {
    return `${width}x${height}`
  }

  let scaledWidth = width
  let scaledHeight = height
  if (scaledWidth / scaledHeight > GPT_IMAGE_2_MAX_ASPECT_RATIO) {
    scaledHeight = scaledWidth / GPT_IMAGE_2_MAX_ASPECT_RATIO
  } else if (scaledHeight / scaledWidth > GPT_IMAGE_2_MAX_ASPECT_RATIO) {
    scaledWidth = scaledHeight / GPT_IMAGE_2_MAX_ASPECT_RATIO
  }

  let scale = 1
  const longestEdge = Math.max(scaledWidth, scaledHeight)
  if (longestEdge > GPT_IMAGE_2_MAX_EDGE) scale = Math.min(scale, GPT_IMAGE_2_MAX_EDGE / longestEdge)
  const adjustedPixels = scaledWidth * scaledHeight
  if (adjustedPixels > GPT_IMAGE_2_MAX_PIXELS) {
    scale = Math.min(scale, Math.sqrt(GPT_IMAGE_2_MAX_PIXELS / adjustedPixels))
  }
  if (adjustedPixels * scale * scale < GPT_IMAGE_2_MIN_PIXELS) {
    scale = Math.sqrt(GPT_IMAGE_2_MIN_PIXELS / adjustedPixels)
  }
  scaledWidth *= scale
  scaledHeight *= scale
  width = Math.floor(scaledWidth / GPT_IMAGE_2_DIMENSION_STEP) * GPT_IMAGE_2_DIMENSION_STEP
  height = Math.floor(scaledHeight / GPT_IMAGE_2_DIMENSION_STEP) * GPT_IMAGE_2_DIMENSION_STEP
  if (width * height < GPT_IMAGE_2_MIN_PIXELS) {
    width = Math.ceil(scaledWidth / GPT_IMAGE_2_DIMENSION_STEP) * GPT_IMAGE_2_DIMENSION_STEP
    height = Math.ceil(scaledHeight / GPT_IMAGE_2_DIMENSION_STEP) * GPT_IMAGE_2_DIMENSION_STEP
  }
  if (width / height > GPT_IMAGE_2_MAX_ASPECT_RATIO) {
    height = Math.ceil(width / GPT_IMAGE_2_MAX_ASPECT_RATIO / GPT_IMAGE_2_DIMENSION_STEP) * GPT_IMAGE_2_DIMENSION_STEP
  } else if (height / width > GPT_IMAGE_2_MAX_ASPECT_RATIO) {
    width = Math.ceil(height / GPT_IMAGE_2_MAX_ASPECT_RATIO / GPT_IMAGE_2_DIMENSION_STEP) * GPT_IMAGE_2_DIMENSION_STEP
  }
  const targetRatio = scaledWidth / scaledHeight
  while (width > GPT_IMAGE_2_MAX_EDGE || height > GPT_IMAGE_2_MAX_EDGE || width * height > GPT_IMAGE_2_MAX_PIXELS) {
    if (width / height > targetRatio) width -= GPT_IMAGE_2_DIMENSION_STEP
    else height -= GPT_IMAGE_2_DIMENSION_STEP
  }
  while (width * height < GPT_IMAGE_2_MIN_PIXELS) {
    if (width / height < targetRatio) width += GPT_IMAGE_2_DIMENSION_STEP
    else height += GPT_IMAGE_2_DIMENSION_STEP
  }
  return `${width}x${height}`
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
