<template>
  <div
    v-if="visible"
    class="header-invite group relative"
    data-testid="header-invite"
  >
    <button
      type="button"
      class="header-invite-button flex items-center gap-1.5 rounded-full bg-gradient-to-r from-primary-500 to-primary-600 px-3 py-1.5 text-sm font-semibold text-white shadow-sm shadow-primary-500/30 transition hover:from-primary-600 hover:to-primary-700 hover:shadow-md focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-400 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-dark-900"
      :aria-label="buttonLabel"
      :disabled="copying"
      data-testid="header-invite-button"
      @click="handleCopy"
    >
      <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M21 11.25v8.25a1.5 1.5 0 01-1.5 1.5H5.25a1.5 1.5 0 01-1.5-1.5v-8.25M12 4.875A2.625 2.625 0 109.375 7.5H12m0-2.625V7.5m0-2.625A2.625 2.625 0 1114.625 7.5H12m0 0V21m-8.625-9.75h18c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125h-18c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z"
        />
      </svg>
      <span class="hidden whitespace-nowrap sm:inline">{{ buttonLabel }}</span>
    </button>

    <!-- Hover / focus card -->
    <div
      class="pointer-events-none absolute right-0 top-full z-40 mt-2 hidden w-72 rounded-xl border border-gray-200 bg-white p-4 text-left shadow-xl group-hover:block group-focus-within:block dark:border-dark-700 dark:bg-dark-800"
      role="tooltip"
      data-testid="header-invite-card"
    >
      <div class="flex items-center gap-2">
        <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
          <Icon name="users" size="sm" />
        </span>
        <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('affiliate.header.title') }}</p>
      </div>
      <ul class="mt-3 space-y-1.5 text-xs leading-relaxed text-gray-600 dark:text-dark-300">
        <li>{{ rateDescription }}</li>
        <li>{{ t('affiliate.header.line2') }}</li>
        <li>{{ t('affiliate.header.line3') }}</li>
      </ul>
      <p class="mt-3 border-t border-gray-100 pt-3 text-xs font-medium text-primary-600 dark:border-dark-700 dark:text-primary-300">
        {{ t('affiliate.header.clickToCopy') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { getAffiliateDetail } from '@/api/user'
import { useClipboard } from '@/composables/useClipboard'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import Icon from '@/components/icons/Icon.vue'
import type { UserAffiliateDetail } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const detail = ref<UserAffiliateDetail | null>(null)
const copying = ref(false)
let loadPromise: Promise<UserAffiliateDetail | null> | null = null

// 与侧边栏「邀请返利」入口保持一致：功能开关开启、已登录、非简易模式才显示。
const visible = computed(() =>
  Boolean(authStore.user) && !authStore.isSimpleMode && isFeatureFlagEnabled(FeatureFlags.affiliate),
)

const ratePercent = computed(() => {
  const raw = Number(detail.value?.effective_rebate_rate_percent ?? 0)
  return Number.isFinite(raw) && raw > 0 ? raw : 0
})

const rateText = computed(() => {
  const value = ratePercent.value
  if (value <= 0) return ''
  return `${Number.isInteger(value) ? value : Number(value.toFixed(2))}%`
})

const buttonLabel = computed(() =>
  rateText.value
    ? t('affiliate.header.button', { rate: rateText.value })
    : t('affiliate.header.buttonNoRate'),
)

const rateDescription = computed(() =>
  rateText.value
    ? t('affiliate.header.line1', { rate: rateText.value })
    : t('affiliate.header.line1NoRate'),
)

function buildInviteLink(code: string): string {
  const path = `/register?aff=${encodeURIComponent(code)}`
  if (typeof window === 'undefined') return path
  return `${window.location.origin}${path}`
}

function loadDetail(): Promise<UserAffiliateDetail | null> {
  if (detail.value) return Promise.resolve(detail.value)
  if (!loadPromise) {
    loadPromise = getAffiliateDetail()
      .then((data) => {
        detail.value = data
        return data
      })
      .catch(() => null)
      .finally(() => {
        loadPromise = null
      })
  }
  return loadPromise
}

async function handleCopy(): Promise<void> {
  if (copying.value) return
  copying.value = true
  try {
    const data = await loadDetail()
    if (!data?.aff_code) {
      appStore.showError(t('affiliate.loadFailed'))
      return
    }
    await copyToClipboard(buildInviteLink(data.aff_code), t('affiliate.linkCopied'))
  } finally {
    copying.value = false
  }
}

// 预取一次，保证点击时可以同步拿到链接（部分浏览器在异步等待后会拒绝写剪贴板）。
onMounted(() => {
  if (visible.value) void loadDetail()
})

watch(visible, (next) => {
  if (next) void loadDetail()
  else detail.value = null
})
</script>
