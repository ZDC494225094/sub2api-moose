<template>
  <div class="premium-home">
    <header class="topbar">
      <nav class="shell nav" aria-label="主导航">
        <RouterLink class="brand" to="/">
          <span class="brand-mark" aria-hidden="true">
            <img v-if="siteLogo" class="brand-logo-img" :src="siteLogo" alt="" />
            <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none">
              <path d="M12 2 21 7v10l-9 5-9-5V7l9-5Z" fill="#2563ff" opacity=".95" />
              <path d="M8 8.3 12 6l4 2.3v4.4L12 15l-4-2.3V8.3Z" fill="#6ddcff" />
            </svg>
          </span>
          <span>{{ siteName }}</span>
        </RouterLink>

        <div class="nav-links">
          <a :class="{ active: activeHomeSection === 'top' }" href="#top" @click="setActiveHomeSection('top')">首页</a>
          <a :class="{ active: activeHomeSection === 'plans' }" href="#plans" @click="setActiveHomeSection('plans')">套餐服务</a>
          <RouterLink to="/docs">文档中心</RouterLink>
        </div>

        <div class="nav-actions">
          <button
            class="icon-btn menu-btn"
            type="button"
            aria-label="打开导航菜单"
            :aria-expanded="menuOpen"
            @click="menuOpen = !menuOpen"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <path d="M4 7h16M4 12h16M4 17h16" />
            </svg>
          </button>
          <div class="theme-switcher">
            <button
              class="icon-btn theme-trigger"
              type="button"
              aria-label="切换主题"
              :aria-expanded="themeMenuOpen"
              @click="themeMenuOpen = !themeMenuOpen"
            >
              <Icon :name="themeIcon" size="sm" />
            </button>
            <div class="theme-menu" :class="{ 'is-open': themeMenuOpen }">
              <button
                v-for="option in themeOptions"
                :key="option.value"
                type="button"
                :class="{ active: themeMode === option.value }"
                @click="setThemeMode(option.value)"
              >
                <Icon :name="option.icon" size="sm" />
                <span>{{ option.label }}</span>
              </button>
            </div>
          </div>
          <RouterLink class="login-btn" :to="dashboardPath">{{ isAuthenticated ? '控制台' : '登录' }}</RouterLink>
          <RouterLink class="primary-btn" :to="isAuthenticated ? dashboardPath : '/register'">
            {{ isAuthenticated ? '进入控制台' : '注册 / 免费试用' }}
          </RouterLink>
        </div>
      </nav>

      <div class="mobile-menu" :class="{ 'is-open': menuOpen }">
        <a :class="{ active: activeHomeSection === 'top' }" href="#top" @click="handleHomeNavClick('top')">首页</a>
        <a :class="{ active: activeHomeSection === 'plans' }" href="#plans" @click="handleHomeNavClick('plans')">套餐服务</a>
        <RouterLink to="/docs" @click="menuOpen = false">文档中心</RouterLink>
      </div>
    </header>

    <main id="top" class="shell">
      <section class="hero">
        <div class="hero-copy">
          <div class="eyebrow">
            <Icon name="badge" size="sm" />
            一站式 AI API 中转服务平台
          </div>
          <h1>连接全球顶尖 AI<br />赋能<span>无限可能</span></h1>
          <p>{{ siteSubtitle }}</p>
          <div class="hero-actions">
            <RouterLink class="primary-btn hero-btn" :to="isAuthenticated ? dashboardPath : '/register'">
              立即开始 
            </RouterLink>
            <RouterLink class="secondary-btn hero-btn" to="/docs">查看文档</RouterLink>
            <button class="secondary-btn hero-btn notice-menu-btn" type="button" @click="noticePanelOpen = true">
              <span class="red-dot" aria-hidden="true"></span>
              公告
            </button>
          </div>
        </div>

        <div class="hero-visual" aria-hidden="true">
          <div class="hero-orbit">
            <canvas ref="globeCanvas" class="globe-canvas"></canvas>
          </div>
        </div>

        <aside class="notice-card" :class="{ 'is-open': noticePanelOpen }" aria-label="公告">
          <div class="notice-head">
            <div class="notice-title">
              <Icon name="menu" size="sm" />
              公告
            </div>
            <button class="notice-close" type="button" aria-label="关闭公告弹窗" @click="noticePanelOpen = false">×</button>
          </div>
          <div class="notice-list">
            <button
              v-for="notice in visibleAnnouncements"
              :key="notice.id"
              class="notice-item"
              type="button"
              @click="openAnnouncement(notice)"
            >
              <div class="notice-date">
                {{ formatDate(notice.created_at || notice.starts_at) }}
                <span v-if="isNewNotice(notice.created_at)" class="new-tag">NEW</span>
              </div>
              <h3>{{ notice.title }}</h3>
              <p>{{ announcementExcerpt(notice.content) }}</p>
            </button>
          </div>
          <button class="subscribe" type="button" @click="openAnnouncement(visibleAnnouncements[0])">
            <Icon name="bell" size="sm" />
            查看全部公告
          </button>
        </aside>
        <div class="notice-backdrop" :class="{ 'is-open': noticePanelOpen }" @click="noticePanelOpen = false"></div>

        <div class="notice-reader-backdrop" :class="{ 'is-open': !!selectedAnnouncement }" @click="closeAnnouncement"></div>
        <section class="notice-reader" :class="{ 'is-open': !!selectedAnnouncement }" aria-label="公告全文">
          <div class="notice-reader-head">
            <div>
              <span>{{ formatDate(selectedAnnouncement?.created_at || selectedAnnouncement?.starts_at) }}</span>
              <h2>{{ selectedAnnouncement?.title }}</h2>
            </div>
            <button type="button" aria-label="关闭公告全文" @click="closeAnnouncement">×</button>
          </div>
          <div class="notice-reader-body markdown-body" v-html="selectedAnnouncementHtml"></div>
        </section>
      </section>

      <section id="capabilities" class="feature-strip" aria-label="平台优势">
        <div v-for="feature in features" :key="feature.title" class="feature-item">
          <Icon :name="feature.icon" size="xl" />
          <div><strong>{{ feature.title }}</strong><span>{{ feature.desc }}</span></div>
        </div>
      </section>

      <section id="plans" class="section">
        <div class="section-head">
          <div class="section-title">
            <h2>套餐服务</h2>
            <span class="soft-badge">超值套餐，灵活选择</span>
          </div>
          <RouterLink class="all-link" to="/purchase?tab=subscription">查看全部套餐</RouterLink>
        </div>

        <div v-if="plansLoading" class="loading-card">正在读取套餐配置...</div>
        <div v-else-if="visiblePlanTabs.length > 0" class="plans-panel">
          <div class="plan-tabs" role="tablist" aria-label="套餐类型">
            <button
              v-for="tab in visiblePlanTabs"
              :key="tab.key"
              type="button"
              role="tab"
              :aria-selected="activePlanTab === tab.key"
              :class="{ active: activePlanTab === tab.key }"
              @click="activePlanTab = tab.key"
            >
              {{ tab.label }}
            </button>
          </div>

          <div class="pricing-grid">
            <article
              v-for="plan in activePlans"
              :key="plan.id"
              class="plan-card"
              :class="{ hot: plan.id === recommendedPlanId }"
              :style="{ '--heat': `${discountPercent(plan)}%` }"
            >
              <div v-if="plan.id === recommendedPlanId" class="hot-ribbon">最受欢迎</div>
              <div class="plan-top">
                <h3>{{ plan.name }}</h3>
              </div>
              <div class="price">
                <strong>{{ formatPlanPrice(plan.price) }}</strong>
                <span>/ {{ validityText(plan) }}</span>
              </div>
              <div v-if="plan.original_price && plan.original_price > plan.price" class="price-compare">
                <span class="current-price">{{ formatPlanPrice(plan.price) }}</span>
                <span class="original-price">{{ formatPlanPrice(plan.original_price) }}</span>
              </div>
              <p class="plan-desc">{{ plan.description || plan.group_name || '灵活套餐配置' }}</p>
              <div class="deal-row">
                <span>接口折扣</span>
                <span class="promo-pill" :class="{ hot: plan.id === recommendedPlanId }">{{ discountLabel(plan) }}</span>
                <strong>{{ plan.original_price && plan.original_price > plan.price ? `省 ¥${Math.round(plan.original_price - plan.price)}` : savingsLabel(plan) }}</strong>
              </div>
              <div class="plan-stats">
                <div class="stat-chip"><span>折扣率</span><strong>{{ discountRateLabel(plan) }}</strong></div>
                <div class="stat-chip">
                  <span>已购买</span>
                  <strong>{{ formatPurchaseCount(displayPurchaseCount(plan)) }} 人</strong>
                </div>
              </div>
              <div class="heat">
                <div class="heat-head"><span>折扣力度</span><strong>{{ discountStrengthLabel(plan) }}</strong></div>
                <div class="heat-track"><i></i></div>
              </div>
              <ul class="features">
                <li v-for="item in planFeatures(plan)" :key="item">{{ item }}</li>
              </ul>
              <RouterLink class="plan-btn" :to="`/purchase?tab=subscription&group=${plan.group_id}`">
                立即购买
              </RouterLink>
            </article>
          </div>
        </div>
        <div v-else class="loading-card">暂无可购买订阅套餐</div>
      </section>

      <section id="models" class="section models-section">
        <div class="section-head">
          <div class="section-title">
            <h2>模型广场</h2>
            <span class="soft-badge">已接入 20+ 顶尖大模型</span>
          </div>
          <a class="all-link" :href="docUrl || '#'" :target="docUrl ? '_blank' : undefined" rel="noopener noreferrer">查看全部模型</a>
        </div>
        <div class="models-grid">
          <article v-for="model in models" :key="model.name" class="model-card">
            <img class="vendor-logo" :src="model.vendor === 'Google' ? googleLogo : model.logo" :alt="`${model.vendor} logo`" />
            <div>
              <h3>{{ model.name }}</h3>
              <p>{{ model.vendor }}</p>
              <div class="tags"><span v-for="tag in model.tags" :key="tag">{{ tag }}</span></div>
            </div>
          </article>
        </div>
      </section>

      <section id="support" class="support-card">
        <div>
          <h2>需要企业级接入方案？</h2>
          <p>支持额度策略、模型路由、私有化部署和多账号稳定调度。</p>
        </div>
        <RouterLink class="primary-btn" :to="isAuthenticated ? dashboardPath : '/login'">联系控制台</RouterLink>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserAnnouncement } from '@/types'
import { getPublicAnnouncements, getPublicPlans } from './api'
import { mountPremiumHomeGlobe } from './premium-home-globe'
import googleLogo from './assets/google-logo.png'
import './premium-home.css'

type IconName = InstanceType<typeof Icon>['$props']['name']
type ThemeMode = 'light' | 'dark' | 'system'

const fallbackAnnouncements: UserAnnouncement[] = [
  {
    id: 1,
    title: 'DeepSeek v4 pro 模型正式接入',
    content: '新模型已全面接入，支持深度推理与复杂任务处理。',
    notify_mode: 'popup',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 2,
    title: '套餐服务升级',
    content: '后台配置套餐将自动同步至首页展示，购买入口直达控制台。',
    notify_mode: 'silent',
    created_at: new Date(Date.now() - 86400000 * 3).toISOString(),
    updated_at: new Date(Date.now() - 86400000 * 3).toISOString(),
  },
]

const appStore = useAppStore()
const authStore = useAuthStore()
const globeCanvas = ref<HTMLCanvasElement | null>(null)
const menuOpen = ref(false)
const noticePanelOpen = ref(false)
const themeMenuOpen = ref(false)
const themeMode = ref<ThemeMode>(readInitialThemeMode())
const systemDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
const selectedAnnouncement = ref<UserAnnouncement | null>(null)
const plans = ref<SubscriptionPlan[]>([])
const announcements = ref<UserAnnouncement[]>([])
const plansLoading = ref(true)
const activePlanTab = ref('openai')
const virtualPurchaseCounts = ref<Record<number, number>>({})
const activeHomeSection = ref<'top' | 'plans'>('top')
let systemThemeQuery: MediaQueryList | null = null
let globeCleanup: (() => void) | null = null

marked.setOptions({
  breaks: true,
  gfm: true,
})

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'AI Hub')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() =>
  appStore.cachedPublicSettings?.site_subtitle || '聚合最前沿的大模型 API，稳定高效的中转服务，助力开发者与企业快速构建智能应用'
)
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
const displayPlans = computed(() => plans.value)
const visibleAnnouncements = computed(() => (announcements.value.length > 0 ? announcements.value : fallbackAnnouncements).slice(0, 5))
const isDark = computed(() => themeMode.value === 'dark' || (themeMode.value === 'system' && systemDark.value))
const themeIcon = computed<IconName>(() => {
  if (themeMode.value === 'system') return 'cpu'
  return isDark.value ? 'moon' : 'sun'
})
const selectedAnnouncementHtml = computed(() => {
  if (!selectedAnnouncement.value?.content) return ''
  const html = marked.parse(selectedAnnouncement.value.content) as string
  return DOMPurify.sanitize(html)
})
const planTabs = [
  { key: 'openai', label: 'OpenAI', keywords: ['openai', 'gpt', 'chatgpt', 'o1', 'o3', 'o4'] },
  { key: 'anthropic', label: 'Anthropic', keywords: ['anthropic', 'claude', 'opus', 'sonnet', 'haiku'] },
  { key: 'google', label: 'Google', keywords: ['google', 'gemini'] },
  { key: 'deepseek', label: 'DeepSeek', keywords: ['deepseek'] },
]

const plansByTab = computed(() => {
  return planTabs.reduce<Record<string, SubscriptionPlan[]>>((groups, tab) => {
    groups[tab.key] = displayPlans.value.filter((plan) => planMatchesTab(plan, tab.key))
    return groups
  }, {})
})

const visiblePlanTabs = computed(() => planTabs.filter((tab) => (plansByTab.value[tab.key] || []).length > 0))
const activePlans = computed(() => plansByTab.value[activePlanTab.value] || [])
const recommendedPlanId = computed(() => {
  if (activePlans.value.length === 0) return -1
  let id = -1
  let maxDiscount = 0
  activePlans.value.forEach((plan) => {
    const discount = discountPercent(plan)
    if (discount > maxDiscount) {
      maxDiscount = discount
      id = plan.id
    }
  })
  return id
})

const features: Array<{ title: string; desc: string; icon: IconName }> = [
  { title: '稳定可靠', desc: '99.9% 可用性保障', icon: 'shield' },
  { title: '极速响应', desc: '毫秒级 API 响应', icon: 'bolt' },
  { title: '安全隐私', desc: '数据加密 · 隐私保护', icon: 'lock' },
  { title: '丰富模型', desc: '接入 20+ 顶尖大模型', icon: 'cube' },
  { title: '优质服务', desc: '7x24 小时技术支持', icon: 'users' },
]

const models = [
  { name: 'GPT-5.5', vendor: 'OpenAI', logo: 'https://openai.com/favicon.ico', tags: ['文本', '视觉', '音频'] },
  { name: 'claude opus4.7', vendor: 'Anthropic', logo: 'https://www.anthropic.com/favicon.ico', tags: ['文本', '推理', '分析'] },
  { name: 'gemini 3.1 flash', vendor: 'Google', logo: 'https://www.google.com/favicon.ico', tags: ['文本', '代码', '多模态'] },
  { name: 'DeepSeek v4 pro', vendor: 'DeepSeek', logo: 'https://www.deepseek.com/favicon.ico', tags: ['推理', '代码', '数学'] },
]

const themeOptions: Array<{ value: ThemeMode; label: string; icon: IconName }> = [
  { value: 'light', label: '浅色', icon: 'sun' },
  { value: 'dark', label: '深色', icon: 'moon' },
  { value: 'system', label: '系统', icon: 'cpu' },
]

function readInitialThemeMode(): ThemeMode {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'light' || savedTheme === 'dark' || savedTheme === 'system') {
    return savedTheme
  }
  return 'system'
}

async function startGlobeAnimation() {
  const canvas = globeCanvas.value
  if (!canvas || globeCleanup || typeof window === 'undefined') return
  const cleanup = await mountPremiumHomeGlobe(canvas, { maxSize: 760, maxPixelRatio: 2 })
  if (!globeCanvas.value || globeCanvas.value !== canvas) {
    cleanup()
    return
  }

  globeCleanup = () => {
    cleanup()
    globeCleanup = null
  }
}

function stopGlobeAnimation() {
  globeCleanup?.()
}

function applyThemeClass() {
  document.documentElement.classList.toggle('dark', isDark.value)
}

function setThemeMode(mode: ThemeMode) {
  themeMode.value = mode
  localStorage.setItem('theme', mode)
  applyThemeClass()
  themeMenuOpen.value = false
}

function normalizePlanPlatform(value?: string) {
  const platform = (value || '').toLowerCase()
  if (platform.includes('anthropic') || platform.includes('claude')) return 'anthropic'
  if (platform.includes('google') || platform.includes('gemini')) return 'google'
  if (platform.includes('deepseek')) return 'deepseek'
  if (platform.includes('openai') || platform.includes('gpt') || platform.includes('chatgpt')) return 'openai'
  return ''
}

function normalizedPlanNameText(plan: SubscriptionPlan) {
  return [
    plan.group_name,
    plan.name,
  ].filter(Boolean).join(' ').toLowerCase()
}

function planMatchesTab(plan: SubscriptionPlan, tabKey: string) {
  const tab = planTabs.find((entry) => entry.key === tabKey)
  if (!tab) return false
  const explicitPlatform = normalizePlanPlatform(plan.group_platform)
  if (explicitPlatform) {
    return explicitPlatform === tabKey
  }
  const text = normalizedPlanNameText(plan)
  return tab.keywords.some((keyword) => text.includes(keyword))
}

function seededCount(seed: number) {
  const value = Math.sin(seed * 12.9898) * 43758.5453
  const fraction = value - Math.floor(value)
  return 118 + Math.floor(fraction * 220)
}

function initializeVirtualPurchaseCounts(sourcePlans: SubscriptionPlan[]) {
  if (sourcePlans.length === 0) {
    virtualPurchaseCounts.value = {}
    return
  }

  const sortedPlans = [...sourcePlans].sort((a, b) => a.price - b.price || a.id - b.id)
  const cheapestPlanId = sortedPlans[0]?.id
  const counts: Record<number, number> = {}

  sortedPlans.forEach((plan, index) => {
    counts[plan.id] = plan.id === cheapestPlanId ? 395 : seededCount(plan.id + index * 17)
  })
  virtualPurchaseCounts.value = counts
}

function displayPurchaseCount(plan: SubscriptionPlan) {
  return virtualPurchaseCounts.value[plan.id] ?? 0
}

function formatPlanPrice(price: number) {
  if (price <= 0) return '¥0'
  return `¥${Number.isInteger(price) ? price : price.toFixed(2)}`
}

function validityText(plan: SubscriptionPlan) {
  if (plan.validity_unit === 'month') return '月'
  if (plan.validity_unit === 'year') return '年'
  return `${plan.validity_days}天`
}

function normalizedDiscountRate(plan: SubscriptionPlan) {
  if (typeof plan.discount_rate === 'number' && Number.isFinite(plan.discount_rate) && plan.discount_rate > 0 && plan.discount_rate < 1) {
    return plan.discount_rate
  }
  return undefined
}

function discountPercent(plan: SubscriptionPlan) {
  const rate = normalizedDiscountRate(plan)
  return rate ? Math.round((1 - rate) * 100) : 0
}

function discountRateLabel(plan: SubscriptionPlan) {
  const rate = normalizedDiscountRate(plan)
  if (!rate) return '无折扣'
  const discount = Math.round(rate * 100) / 10
  return `${Number.isInteger(discount) ? discount.toFixed(0) : discount.toFixed(1)} 折`
}

function discountLabel(plan: SubscriptionPlan) {
  return normalizedDiscountRate(plan) ? discountRateLabel(plan) : '原价'
}

function savingsLabel(plan: SubscriptionPlan) {
  if (plan.original_price && plan.original_price > plan.price) {
    return `省 ¥${Math.round(plan.original_price - plan.price)}`
  }
  if (plan.price <= 0) return '免费体验'
  return '按原价'
}

function discountStrengthLabel(plan: SubscriptionPlan) {
  const rate = normalizedDiscountRate(plan)
  if (!rate) return '暂无折扣'
  if (rate > 0.9) return '大折扣'
  if (rate > 0.7) return '超大折扣'
  return '限时优惠'
  const percent = discountPercent(plan)
  if (percent >= 40) return '高折扣'
  if (percent >= 20) return '中折扣'
  if (percent > 0) return '轻折扣'
  return '暂无折扣'
}

function formatPurchaseCount(value?: number) {
  return new Intl.NumberFormat('zh-CN').format(value || 0)
}

function planFeatures(plan: SubscriptionPlan) {
  const base = [...(plan.features || [])]
  if (plan.rate_multiplier && plan.rate_multiplier !== 1) {
    base.unshift(`倍率 ×${plan.rate_multiplier}`)
  }
  if (plan.monthly_limit_usd != null) {
    base.unshift(`月额度 $${plan.monthly_limit_usd}`)
  } else if (plan.daily_limit_usd == null && plan.weekly_limit_usd == null) {
    base.unshift('不限额度策略')
  }
  return base
}

function stripMarkdown(content: string) {
  return content
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)]\([^)]*\)/g, '$1')
    .replace(/[#>*_\-~|]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function announcementExcerpt(content: string) {
  const plain = stripMarkdown(content)
  return plain.length > 64 ? `${plain.slice(0, 64)}...` : plain
}

function openAnnouncement(notice?: UserAnnouncement) {
  if (!notice) return
  selectedAnnouncement.value = notice
  noticePanelOpen.value = false
}

function closeAnnouncement() {
  selectedAnnouncement.value = null
}

function formatDate(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString().slice(0, 10)
}

function isNewNotice(value?: string) {
  if (!value) return false
  const created = new Date(value).getTime()
  return Number.isFinite(created) && Date.now() - created < 7 * 86400000
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    menuOpen.value = false
    themeMenuOpen.value = false
    noticePanelOpen.value = false
    closeAnnouncement()
  }
}

function onSystemThemeChange(event: MediaQueryListEvent) {
  systemDark.value = event.matches
  if (themeMode.value === 'system') {
    applyThemeClass()
  }
}

function setActiveHomeSection(section: 'top' | 'plans') {
  activeHomeSection.value = section
}

function handleHomeNavClick(section: 'top' | 'plans') {
  setActiveHomeSection(section)
  menuOpen.value = false
}

function syncActiveSectionFromHash() {
  activeHomeSection.value = window.location.hash === '#plans' ? 'plans' : 'top'
}

watch(visiblePlanTabs, (tabs) => {
  if (tabs.length === 0) return
  if (!tabs.some((tab) => tab.key === activePlanTab.value)) {
    activePlanTab.value = tabs[0].key
  }
}, { immediate: true })

onMounted(async () => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('hashchange', syncActiveSectionFromHash)
  syncActiveSectionFromHash()
  systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)')
  systemDark.value = systemThemeQuery.matches
  systemThemeQuery.addEventListener('change', onSystemThemeChange)
  applyThemeClass()
  startGlobeAnimation().catch((error) => {
    console.warn('Premium home globe fallback:', error)
  })

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings().catch(() => {})
  }
  authStore.checkAuth()

  try {
    const [publicPlans, publicAnnouncements] = await Promise.all([
      getPublicPlans(),
      getPublicAnnouncements(),
    ])
    plans.value = publicPlans
    initializeVirtualPurchaseCounts(publicPlans)
    announcements.value = publicAnnouncements
  } catch (error) {
    console.warn('Premium home public data fallback:', error)
  } finally {
    plansLoading.value = false
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('hashchange', syncActiveSectionFromHash)
  systemThemeQuery?.removeEventListener('change', onSystemThemeChange)
  systemThemeQuery = null
  stopGlobeAnimation()
})
</script>
