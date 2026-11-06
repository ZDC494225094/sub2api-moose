import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ThemeSwitcher from '../ThemeSwitcher.vue'
import { THEME_STORAGE_KEY } from '../themeMode'

enableAutoUnmount(afterEach)

const messages = {
  consoleTheme: {
    switcher: () => '切换主题',
    light: () => '浅色',
    dark: () => '深色',
    system: () => '跟随系统',
  },
}
const fixtureNodes: HTMLElement[] = []

function mountSwitcher(selected = 'dark') {
  localStorage.setItem(THEME_STORAGE_KEY, selected)
  const host = document.createElement('div')
  document.body.append(host)
  fixtureNodes.push(host)
  return mount(ThemeSwitcher, {
    attachTo: host,
    global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: messages } })] },
  })
}

function neighboringButton() {
  const button = document.createElement('button')
  button.textContent = 'Next control'
  document.body.append(button)
  fixtureNodes.push(button)
  return button
}

describe('console theme switcher keyboard interaction', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList)
  })

  afterEach(() => {
    fixtureNodes.splice(0).forEach(node => node.remove())
    document.documentElement.classList.remove('dark')
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it.each(['Enter', ' '])('focuses the selected item after native %s button activation', async key => {
    const wrapper = mountSwitcher('dark')
    const trigger = wrapper.get<HTMLButtonElement>('[data-test="console-theme-trigger"]')
    trigger.element.focus()
    await trigger.trigger('keydown', { key })
    if (key === ' ') await trigger.trigger('keyup', { key })
    // jsdom does not synthesize a button's native keyboard click. Its click()
    // has detail=0, matching the activation delivered by Enter or Space.
    trigger.element.click()
    await nextTick()

    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(document.activeElement).toBe(wrapper.get('[data-mode="dark"]').element)
    expect(wrapper.findAll('[role="menuitemradio"][tabindex="0"]')).toHaveLength(1)
  })

  it.each([
    ['ArrowDown', 'light'],
    ['ArrowUp', 'system'],
  ])('opens with %s and focuses the boundary item %s', async (key, expectedMode) => {
    const wrapper = mountSwitcher('dark')
    const trigger = wrapper.get<HTMLButtonElement>('[data-test="console-theme-trigger"]')
    trigger.element.focus()
    const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
    trigger.element.dispatchEvent(event)
    await nextTick()

    expect(event.defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(wrapper.get(`[data-mode="${expectedMode}"]`).element)
  })

  it('cycles arrows and moves to Home and End without changing the selected preference', async () => {
    const wrapper = mountSwitcher('dark')
    await wrapper.get('[data-test="console-theme-trigger"]').trigger('click')

    for (const [key, expectedMode] of [
      ['ArrowDown', 'system'],
      ['ArrowDown', 'light'],
      ['ArrowUp', 'system'],
      ['Home', 'light'],
      ['End', 'system'],
    ]) {
      const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
      document.activeElement?.dispatchEvent(event)
      await nextTick()
      expect(event.defaultPrevented).toBe(true)
      expect(document.activeElement).toBe(wrapper.get(`[data-mode="${expectedMode}"]`).element)
      expect(wrapper.findAll('[role="menuitemradio"][tabindex="0"]')).toHaveLength(1)
    }
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
    expect(wrapper.get('[data-mode="dark"]').attributes('aria-checked')).toBe('true')
  })

  it('returns focus to the trigger after Escape or choosing a preference', async () => {
    const wrapper = mountSwitcher('dark')
    const trigger = wrapper.get<HTMLButtonElement>('[data-test="console-theme-trigger"]')
    await trigger.trigger('click')
    await wrapper.get('[data-mode="dark"]').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)

    await trigger.trigger('click')
    await wrapper.get('[data-mode="light"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('lets forward Tab move to the next control before removing the menu', async () => {
    const wrapper = mountSwitcher('dark')
    const next = neighboringButton()
    await wrapper.get('[data-test="console-theme-trigger"]').trigger('click')
    const item = wrapper.get<HTMLButtonElement>('[data-mode="dark"]')
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
    item.element.dispatchEvent(event)

    expect(event.defaultPrevented).toBe(false)
    expect(item.element.isConnected).toBe(true)
    expect(document.activeElement).toBe(item.element)
    // jsdom has no native Tab navigation; emulate the browser's default focus
    // move and verify focusout closes the menu without stealing focus back.
    next.focus()
    await nextTick()
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(next)
  })

  it('lets Shift+Tab reach the trigger before the deferred menu close', async () => {
    vi.useFakeTimers()
    const wrapper = mountSwitcher('dark')
    const trigger = wrapper.get<HTMLButtonElement>('[data-test="console-theme-trigger"]')
    await trigger.trigger('click')
    const item = wrapper.get<HTMLButtonElement>('[data-mode="dark"]')
    const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true })
    item.element.dispatchEvent(event)

    expect(event.defaultPrevented).toBe(false)
    expect(item.element.isConnected).toBe(true)
    trigger.element.focus()
    await vi.runOnlyPendingTimersAsync()
    await nextTick()
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
  })

  it('closes on an outside click without moving focus from the outside target', async () => {
    const wrapper = mountSwitcher('dark')
    const outside = neighboringButton()
    await wrapper.get('[data-test="console-theme-trigger"]').trigger('click')
    outside.focus()
    outside.click()
    await nextTick()

    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    expect(document.activeElement).toBe(outside)
  })
})
