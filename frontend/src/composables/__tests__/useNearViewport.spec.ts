import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { useNearViewport } from '../useNearViewport'

afterEach(() => vi.unstubAllGlobals())

describe('deferred section loading', () => {
  it('waits for an enabled section to approach the viewport and keeps it loaded', async () => {
    let intersect!: IntersectionObserverCallback
    const observe = vi.fn()
    const disconnect = vi.fn()
    vi.stubGlobal('IntersectionObserver', vi.fn(function (callback: IntersectionObserverCallback) {
      intersect = callback
      return { observe, disconnect }
    }))
    const enabled = ref(false)
    const wrapper = mount(defineComponent({
      setup() {
        const target = ref<HTMLElement | null>(null)
        return { target, enabled, visible: useNearViewport(target) }
      },
      template: '<div><section v-if="enabled" ref="target" /><span>{{ visible }}</span></div>'
    }))
    expect(observe).not.toHaveBeenCalled()
    enabled.value = true
    await nextTick()
    expect(observe).toHaveBeenCalledWith(wrapper.find('section').element)
    expect(wrapper.text()).toBe('false')
    intersect([{ isIntersecting: false }] as IntersectionObserverEntry[], {} as IntersectionObserver)
    await nextTick()
    expect(wrapper.text()).toBe('false')
    intersect([{ isIntersecting: true }] as IntersectionObserverEntry[], {} as IntersectionObserver)
    await nextTick()
    expect(wrapper.text()).toBe('true')
    enabled.value = false
    await nextTick()
    expect(wrapper.text()).toBe('true')
    wrapper.unmount()
    expect(disconnect).toHaveBeenCalled()
  })

  it('loads normally in browsers without IntersectionObserver', async () => {
    vi.stubGlobal('IntersectionObserver', undefined)
    const wrapper = mount(defineComponent({
      setup() {
        const target = ref<HTMLElement | null>(null)
        return { target, visible: useNearViewport(target) }
      },
      template: '<section ref="target">{{ visible }}</section>'
    }))
    await nextTick()
    expect(wrapper.text()).toBe('true')
    wrapper.unmount()
  })
})
