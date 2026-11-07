import { afterEach, describe, expect, it } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { applyConsoleTheme } from '../index'
import { useConsoleThemeApplied } from '../useConsoleThemeApplied'

const Probe = defineComponent({
  setup() {
    const applied = useConsoleThemeApplied()
    return () => h('span', applied.value ? 'glass' : 'default')
  },
})

// MutationObserver callbacks run as microtasks.
const flush = async () => { await Promise.resolve(); await nextTick() }

describe('useConsoleThemeApplied', () => {
  afterEach(() => applyConsoleTheme(false))

  it('follows the root theme attribute while mounted', async () => {
    applyConsoleTheme(true)
    const wrapper = mount(Probe)
    expect(wrapper.text()).toBe('glass')

    applyConsoleTheme(false)
    await flush()
    expect(wrapper.text()).toBe('default')

    applyConsoleTheme(true)
    await flush()
    expect(wrapper.text()).toBe('glass')

    wrapper.unmount()
  })
})
