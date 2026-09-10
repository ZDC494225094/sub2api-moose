export interface PremiumHomeGlobeController {
  (): void
  setTheme(isDark: boolean): void
  setAnimating(enabled: boolean): void
  setSiteLogo(url: string): void
}

export function mountPremiumHomeGlobe(
  canvas: HTMLCanvasElement | null,
  options?: {
    maxSize?: number
    maxPixelRatio?: number
    isDark?: boolean
    animate?: boolean
    providerLogos?: Record<string, string>
  },
): Promise<PremiumHomeGlobeController>
export const globeProviderMarkers: ReadonlyArray<{ id: string; name: string; model: string }>
