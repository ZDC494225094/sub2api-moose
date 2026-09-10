import { onBeforeUnmount, onMounted, type Ref } from 'vue'

/** Observe asynchronously loaded sections too; no scroll-time layout reads. */
export function useHomeMotion(root: Ref<HTMLElement | null>) {
  let observer: IntersectionObserver | undefined
  let mutations: MutationObserver | undefined
  const seen = new WeakSet<Element>()
  onMounted(() => {
    if (!root.value) return
    observer = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) return
        entry.target.classList.add('is-revealed')
        observer?.unobserve(entry.target)
      })
    }, { threshold: 0.06, rootMargin: '0px 0px -32px 0px' })
    const register = () => {
      root.value?.querySelectorAll<HTMLElement>('.provider-band, .feature-item, .section-head, .pricing-compare-panel, .plan-card, .loading-card, .model-card, .support-card, .home-footer, .model-plaza-home-section > :not(.section-head)').forEach((element) => {
        if (seen.has(element)) return
        seen.add(element)
        const siblings = Array.from(element.parentElement?.children || [])
        element.style.setProperty('--reveal-delay', `${Math.min(siblings.indexOf(element) % 5, 4) * 65}ms`)
        element.classList.add('scroll-reveal')
        observer?.observe(element)
      })
    }
    register()
    mutations = new MutationObserver(register)
    mutations.observe(root.value, { childList: true, subtree: true })
  })
  onBeforeUnmount(() => { observer?.disconnect(); mutations?.disconnect() })
}
