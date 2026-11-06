<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useThemeMode, type ThemeMode } from './themeMode'

const { t } = useI18n()
const { mode, setMode } = useThemeMode()
const open = ref(false)
const container = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const focusedIndex = ref(0)
let tabCloseTimer: ReturnType<typeof setTimeout> | undefined

const options = computed(() => [
  { value: 'light' as ThemeMode, label: t('consoleTheme.light'), icon: 'sun' as const },
  { value: 'dark' as ThemeMode, label: t('consoleTheme.dark'), icon: 'moon' as const },
  { value: 'system' as ThemeMode, label: t('consoleTheme.system'), icon: 'cpu' as const },
])
const current = computed(() => options.value.find(option => option.value === mode.value) ?? options.value[2])

function cancelTabClose() {
  if (tabCloseTimer !== undefined) clearTimeout(tabCloseTimer)
  tabCloseTimer = undefined
}

function closeMenu(restoreFocus = false) {
  cancelTabClose()
  open.value = false
  if (restoreFocus) trigger.value?.focus()
}

function focusItem(index: number) {
  focusedIndex.value = (index + options.value.length) % options.value.length
  menu.value?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')[focusedIndex.value]?.focus()
}

async function openMenu(index = options.value.findIndex(option => option.value === mode.value)) {
  cancelTabClose()
  focusedIndex.value = index < 0 ? 0 : index
  open.value = true
  await nextTick()
  if (open.value) focusItem(focusedIndex.value)
}

function toggleMenu() {
  if (open.value) closeMenu()
  else void openMenu()
}

function onTriggerKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
  event.preventDefault()
  void openMenu(event.key === 'ArrowDown' ? 0 : options.value.length - 1)
}

function onMenuKeydown(event: KeyboardEvent) {
  if (event.key === 'Tab') {
    // Keep the focused item mounted until the browser completes native Tab
    // navigation. Only the roving item is tabbable; Shift+Tab can reach the
    // trigger, and forward Tab can reach the following page control.
    cancelTabClose()
    tabCloseTimer = setTimeout(() => closeMenu(), 0)
    return
  }
  const nextIndex = event.key === 'ArrowDown' ? focusedIndex.value + 1
    : event.key === 'ArrowUp' ? focusedIndex.value - 1
    : event.key === 'Home' ? 0
    : event.key === 'End' ? options.value.length - 1
    : null
  if (nextIndex === null) return
  event.preventDefault()
  focusItem(nextIndex)
}

function onFocusout(event: FocusEvent) {
  const next = event.relatedTarget
  if (open.value && (!(next instanceof Node) || !container.value?.contains(next))) closeMenu()
}

function choose(next: ThemeMode) {
  setMode(next)
  closeMenu(true)
}

function onDocumentClick(event: MouseEvent) {
  if (open.value && container.value && !container.value.contains(event.target as Node)) closeMenu()
}
function onKeydown(event: KeyboardEvent) {
  if (open.value && event.key === 'Escape') {
    event.preventDefault()
    closeMenu(true)
  }
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
  document.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  cancelTabClose()
  document.removeEventListener('click', onDocumentClick)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="container" class="ct-theme-switcher" @focusout="onFocusout">
    <button
      ref="trigger"
      type="button"
      class="ct-theme-trigger"
      :title="`${t('consoleTheme.switcher')} · ${current.label}`"
      :aria-label="t('consoleTheme.switcher')"
      aria-haspopup="menu"
      :aria-expanded="open"
      data-test="console-theme-trigger"
      @click="toggleMenu"
      @keydown="onTriggerKeydown"
    >
      <Icon :name="current.icon" size="sm" />
    </button>
    <div v-if="open" ref="menu" class="ct-theme-menu" role="menu" :aria-label="t('consoleTheme.switcher')" @keydown="onMenuKeydown">
      <button
        v-for="(option, index) in options"
        :key="option.value"
        type="button"
        role="menuitemradio"
        :aria-checked="mode === option.value"
        :class="{ 'is-active': mode === option.value }"
        :data-mode="option.value"
        :tabindex="focusedIndex === index ? 0 : -1"
        @focus="focusedIndex = index"
        @click="choose(option.value)"
      >
        <Icon :name="option.icon" size="sm" />
        <span>{{ option.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.ct-theme-switcher {
  position: relative;
  display: inline-flex;
}

.ct-theme-trigger {
  display: inline-grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border: 0;
  border-radius: 999px;
  background: var(--ct-fill, rgba(229, 229, 229, 0.7));
  color: var(--ct-ink, #181818);
  cursor: pointer;
  transition: background-color 0.16s ease;
}

.ct-theme-trigger:hover,
.ct-theme-trigger[aria-expanded='true'] {
  background: var(--ct-fill-hover, rgba(212, 212, 212, 0.8));
}

.ct-theme-trigger:focus-visible {
  outline: 2px solid rgba(0, 111, 238, 0.55);
  outline-offset: 2px;
}

.ct-theme-menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 60;
  display: grid;
  min-width: 148px;
  gap: 2px;
  padding: 6px;
  border: 1px solid var(--ct-edge, rgba(255, 255, 255, 0.75));
  border-radius: 16px;
  background: var(--ct-surface-menu, rgba(255, 255, 255, 0.9));
  box-shadow: var(--ct-shadow-pop, 0 18px 48px -12px rgba(24, 24, 24, 0.22));
  backdrop-filter: blur(24px) saturate(170%);
  -webkit-backdrop-filter: blur(24px) saturate(170%);
  animation: ct-theme-menu-in 0.16s ease-out;
}

.ct-theme-menu button {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--ct-ink, #181818);
  cursor: pointer;
  padding: 0 10px;
  font-size: 13px;
  font-weight: 500;
  text-align: left;
}

.ct-theme-menu button:hover {
  background: var(--ct-fill, rgba(229, 229, 229, 0.7));
}

.ct-theme-menu button.is-active {
  background: var(--ct-fill, rgba(229, 229, 229, 0.7));
  font-weight: 600;
}

@keyframes ct-theme-menu-in {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (prefers-reduced-motion: reduce) {
  .ct-theme-menu { animation: none; }
}
</style>
