export function mountPremiumHomeGlobe(
  canvas: HTMLCanvasElement | null,
  options?: {
    maxSize?: number
    maxPixelRatio?: number
  },
): Promise<() => void>
