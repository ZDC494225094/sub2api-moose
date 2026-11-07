import { onBeforeUnmount, onMounted, ref } from 'vue'
import { CONSOLE_THEME_ATTRIBUTE, isConsoleThemeApplied } from './index'

/**
 * Reactive view of the root theme attribute, for pieces the stylesheet cannot
 * reach (canvas and WebGL colours). It follows the attribute itself, so it
 * agrees with the CSS from the first frame, including the cached state painted
 * before the extension state arrives.
 */
export function useConsoleThemeApplied() {
  const applied = ref(isConsoleThemeApplied())
  let observer: MutationObserver | undefined

  onMounted(() => {
    applied.value = isConsoleThemeApplied()
    if (typeof MutationObserver === 'undefined') return
    observer = new MutationObserver(() => { applied.value = isConsoleThemeApplied() })
    observer.observe(document.documentElement, { attributes: true, attributeFilter: [CONSOLE_THEME_ATTRIBUTE] })
  })
  onBeforeUnmount(() => observer?.disconnect())

  return applied
}
