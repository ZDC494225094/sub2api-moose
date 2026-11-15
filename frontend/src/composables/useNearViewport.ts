import { onBeforeUnmount, ref, watch, type Ref } from 'vue'

/** Load a section once it approaches the viewport, including sections enabled later. */
export function useNearViewport(target: Ref<HTMLElement | null>, rootMargin = '300px') {
  const visible = ref(false)
  let observer: IntersectionObserver | undefined

  watch(target, (element) => {
    observer?.disconnect()
    if (!element || visible.value) return
    if (typeof IntersectionObserver === 'undefined') {
      visible.value = true
      return
    }
    observer = new IntersectionObserver((entries) => {
      if (!entries.some(entry => entry.isIntersecting)) return
      visible.value = true
      observer?.disconnect()
    }, { rootMargin })
    observer.observe(element)
  }, { flush: 'post' })

  onBeforeUnmount(() => observer?.disconnect())
  return visible
}
