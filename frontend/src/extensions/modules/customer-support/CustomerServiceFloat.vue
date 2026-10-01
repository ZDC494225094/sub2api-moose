<template>
  <div
    v-if="hasDetails || alwaysVisible"
    ref="containerRef"
    :class="{ 'support-home': alwaysVisible }"
    class="fixed bottom-4 right-4 z-40 flex flex-col items-end sm:bottom-6 sm:right-6"
    @keydown.esc.stop="closePanel"
  >
    <Transition name="support-panel">
      <section
        v-if="panelOpen"
        id="customer-service-panel"
        class="mb-3 w-[min(calc(100vw-2rem),20rem)] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl shadow-gray-900/10 dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30"
        role="region"
        :aria-label="t('common.customerService.title')"
      >
        <header class="flex items-start justify-between gap-4 border-b border-gray-100 px-4 py-3.5 dark:border-dark-700">
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('common.customerService.title') }}
            </h2>
            <p class="mt-0.5 text-xs leading-5 text-gray-500 dark:text-dark-300">
              {{ t('common.customerService.description') }}
            </p>
          </div>
          <button
            type="button"
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700 dark:hover:text-white"
            :aria-label="t('common.close')"
            :title="t('common.close')"
            @click="closePanel"
          >
            <Icon name="x" size="sm" />
          </button>
        </header>

        <p v-if="!hasDetails" class="px-4 py-5 text-sm text-gray-500 dark:text-dark-300">{{ t('common.customerService.unavailable') }}</p>

        <div
          v-if="contactInfo || afterSalesGroup"
          class="divide-y divide-gray-100 px-4 dark:divide-dark-700"
        >
          <div v-if="contactInfo" class="flex items-center gap-3 py-3.5">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
              <Icon name="chat" size="md" />
            </span>
            <div class="min-w-0 flex-1">
              <p class="text-xs text-gray-500 dark:text-dark-300">
                {{ t('common.customerService.contact') }}
              </p>
              <p class="mt-0.5 whitespace-pre-wrap break-words text-sm font-medium text-gray-900 dark:text-white">
                {{ contactInfo }}
              </p>
            </div>
            <button
              type="button"
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-primary-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700 dark:hover:text-primary-300"
              :aria-label="`${t('common.copy')} ${t('common.customerService.contact')}`"
              :title="t('common.copy')"
              @click="copyToClipboard(contactInfo)"
            >
              <Icon name="copy" size="sm" />
            </button>
          </div>

          <div v-if="afterSalesGroup" class="flex items-center gap-3 py-3.5">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300">
              <Icon name="users" size="md" />
            </span>
            <div class="min-w-0 flex-1">
              <p class="text-xs text-gray-500 dark:text-dark-300">
                {{ t('common.customerService.afterSalesGroup') }}
              </p>
              <p class="mt-0.5 whitespace-pre-wrap break-words text-sm font-medium text-gray-900 dark:text-white">
                {{ afterSalesGroup }}
              </p>
            </div>
            <button
              type="button"
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-emerald-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700 dark:hover:text-emerald-300"
              :aria-label="`${t('common.copy')} ${t('common.customerService.afterSalesGroup')}`"
              :title="t('common.copy')"
              @click="copyToClipboard(afterSalesGroup)"
            >
              <Icon name="copy" size="sm" />
            </button>
          </div>
        </div>

        <div
          v-if="customerServiceLink"
          class="px-4 py-3"
          :class="contactInfo || afterSalesGroup ? 'border-t border-gray-100 dark:border-dark-700' : ''"
        >
          <a
            :href="customerServiceLink"
            target="_blank"
            rel="noopener noreferrer"
            class="support-direct-link flex min-h-11 w-full items-center justify-center gap-2 rounded-md bg-primary-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-primary-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:ring-offset-dark-800"
          >
            <span>{{ t('common.customerService.contactNow') }}</span>
            <Icon name="externalLink" size="sm" :stroke-width="2" />
          </a>
        </div>
      </section>
    </Transition>

    <a
      v-if="directLink && customerServiceLink"
      :href="customerServiceLink"
      target="_blank"
      rel="noopener noreferrer"
      class="support-direct-link flex min-h-12 items-center justify-center gap-2 rounded-full bg-primary-600 px-5 py-3 text-sm font-semibold text-white shadow-lg shadow-primary-600/25 transition hover:-translate-y-0.5 hover:bg-primary-700"
    ><Icon name="chat" size="md" /><span>{{ t('common.customerService.contactUs') }}</span></a>
    <button
      v-else
      type="button"
      class="flex h-12 w-12 items-center justify-center rounded-full bg-primary-600 text-white shadow-lg shadow-primary-600/25 transition duration-200 hover:-translate-y-0.5 hover:bg-primary-700 hover:shadow-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 active:translate-y-0 dark:ring-offset-dark-950"
      :class="{ 'support-home-trigger': alwaysVisible }"
      :aria-label="t('common.customerService.title')"
      :aria-expanded="panelOpen"
      aria-controls="customer-service-panel"
      :title="t('common.customerService.title')"
      @click="panelOpen = !panelOpen"
    >
      <Icon :name="panelOpen ? 'x' : 'chat'" size="lg" :stroke-width="2" />
      <span v-if="alwaysVisible">{{ t('common.customerService.contactUs') }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { sanitizeUrl } from '@/utils/url'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
withDefaults(defineProps<{ alwaysVisible?: boolean; directLink?: boolean }>(), { alwaysVisible: false, directLink: false })
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const containerRef = ref<HTMLElement | null>(null)
const panelOpen = ref(false)

const contactInfo = computed(() =>
  (appStore.cachedPublicSettings?.contact_info || appStore.contactInfo || '').trim()
)
const afterSalesGroup = computed(() =>
  (appStore.cachedPublicSettings?.after_sales_group || '').trim()
)
const customerServiceLink = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.customer_service_link || '')
)
const hasDetails = computed(() =>
  Boolean(contactInfo.value || afterSalesGroup.value || customerServiceLink.value)
)

function closePanel() {
  panelOpen.value = false
}

defineExpose({ openPanel: () => { panelOpen.value = true } })

function handlePointerDown(event: PointerEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    closePanel()
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', handlePointerDown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handlePointerDown)
})
</script>

<style scoped>
.support-home {
  --support-ink: var(--ink, #172033);
  --support-muted: var(--muted, #657185);
  font-family: inherit;
}
.support-home section {
  width: min(calc(100vw - 2rem), 350px);
  border: 1px solid var(--line, #dfe5ef);
  border-radius: 20px;
  background: linear-gradient(145deg, #fff, #f5f8ff);
  box-shadow: 0 18px 60px #1c355d20, 0 3px 12px #1c355d08;
  color: var(--support-ink);
}
.support-home section header { padding: 21px 20px 17px; border-color: var(--line, #dfe5ef); }
.support-home section header h2 { color: var(--support-ink); font-size: 16px; font-weight: 650; letter-spacing: -.025em; }
.support-home section header p { color: var(--support-muted); font-size: 12px; line-height: 1.75; margin-top: 5px; }
.support-home section > .divide-y { padding: 0 20px; }
.support-home section > .divide-y > div { padding: 17px 0; border-color: var(--line, #dfe5ef); gap: 12px; }
.support-home section > .divide-y > div > span {
  width: 40px;
  height: 40px;
  border: 1px solid #315cf414;
  border-radius: 12px;
  color: #416cef;
  background: #315cf409;
}
.support-home section .min-w-0.flex-1 > p:first-child { color: var(--support-muted); font-size: 11px; }
.support-home section .min-w-0.flex-1 > p:last-child {
  color: var(--support-ink);
  font-family: inherit;
  font-size: 14px;
  font-weight: 550;
  line-height: 1.7;
  font-variant-numeric: tabular-nums;
}
.support-home section > div:last-child:not(.divide-y) { padding: 14px 20px 20px; border-color: var(--line, #dfe5ef); }
.support-home section button { border-radius: 9px; color: var(--support-muted); }
.support-home section button:hover { color: #416cef; background: #315cf40c; }
.support-home .support-direct-link,
.support-home .support-home-trigger {
  background: linear-gradient(135deg, #396bff, #3154df);
  border: 1px solid #ffffff24;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  box-shadow: 0 5px 16px #315cf427, inset 0 1px 0 #ffffff26;
  transition: transform .2s ease, box-shadow .2s ease;
}
.support-home .support-home-trigger { height: 46px; border-radius: 14px; padding: 0 18px; }
.support-home .support-home-trigger > svg { width: 19px; height: 19px; }
.support-home section .support-direct-link { min-height: 44px; border-radius: 11px; }
.support-home .support-direct-link:hover,
.support-home .support-home-trigger:hover { transform: translateY(-2px); box-shadow: 0 8px 24px #315cf438; }
.support-home :is(button, a):focus-visible { outline: 2px solid #81a3ff; outline-offset: 3px; }
.dark .support-home {
  --support-ink: #edf2ff;
  --support-muted: #9ba9c0;
  --line: #a2b9e520;
  color-scheme: dark;
}
.dark .support-home section { background: linear-gradient(145deg, #18253b, #111c2f); box-shadow: 0 18px 60px #0005; }
.dark .support-home section > .divide-y > div > span { background: #729bff12; border-color: #729bff20; color: #91b2ff; }
.dark .support-home section button:hover { color: #91b2ff; background: #729bff12; }
.support-direct-link { color: white; }
.support-home-trigger { width: auto; padding: 0 20px; gap: 8px; font-size: 14px; }
.support-panel-enter-active,
.support-panel-leave-active {
  transition:
    opacity 180ms ease-out,
    transform 180ms ease-out;
  transform-origin: bottom right;
}

.support-panel-enter-from,
.support-panel-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}

@media (prefers-reduced-motion: reduce) {
  .support-panel-enter-active,
  .support-panel-leave-active {
    transition: none;
  }
}
</style>
