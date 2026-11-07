export type PremiumHomeGlobePalette = 'default' | 'glass'

export interface PremiumHomeGlobeController {
  (): void
  setTheme(isDark: boolean): void
  /** "glass" switches to the console theme's neutral colours; "default" restores the original. */
  setPalette(name: PremiumHomeGlobePalette): void
  setAnimating(enabled: boolean): void
  setSiteLogo(url: string): Promise<void> | void
}

export function mountPremiumHomeGlobe(
  canvas: HTMLCanvasElement | null,
  options?: {
    maxSize?: number
    maxPixelRatio?: number
    isDark?: boolean
    palette?: PremiumHomeGlobePalette
    animate?: boolean
    providerLogos?: Record<string, string>
  },
): Promise<PremiumHomeGlobeController>
export const globeProviderMarkers: ReadonlyArray<{ id: string; name: string; model: string }>
