<template>
  <div class="premium-home docs-home">
    <!-- Unstyled hook: the console theme draws its honeycomb backdrop here. -->
    <div class="home-backdrop" aria-hidden="true"></div>
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
          <span class="brand-text">{{ siteName }}</span>
        </RouterLink>

        <div class="nav-links">
          <RouterLink to="/">首页</RouterLink>
          <RouterLink :to="{ path: '/', hash: '#plans' }">套餐服务</RouterLink>
          <RouterLink class="active" to="/docs">文档中心</RouterLink>
        </div>

        <div class="nav-actions">
          <RouterLink class="primary-btn" :to="isAuthenticated ? '/dashboard' : '/login'">
            {{ isAuthenticated ? '打开控制台' : '登录' }}
          </RouterLink>
        </div>
      </nav>
    </header>

    <main class="shell guide-shell">
      <div v-if="loading" class="guide-state">正在加载文档…</div>
      <div v-else-if="loadError" class="guide-state">
        <h1>文档暂时无法加载</h1>
        <p>{{ loadError }}</p>
      </div>
      <div v-else class="guide-layout">
        <aside class="guide-toc" aria-label="文档目录">
          <div class="guide-toc-card">
            <p class="guide-toc-title">本页目录</p>
            <a
              v-for="item in toc"
              :key="item.id"
              :href="`#${item.id}`"
              :class="{ 'is-h3': item.level === 3, 'is-active': item.id === activeId }"
              :aria-current="item.id === activeId ? 'location' : undefined"
            >{{ item.text }}</a>
          </div>
        </aside>
        <article class="guide-article">
          <p class="guide-eyebrow">文档中心</p>
          <h1 class="guide-title">{{ title }}</h1>
          <p v-if="updatedText" class="guide-meta">更新于 {{ updatedText }}</p>
          <!-- Content is admin-authored markdown, rendered through marked + DOMPurify. -->
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div ref="contentRef" class="markdown-body" v-html="html" @click="onContentClick"></div>
        </article>
      </div>
    </main>

    <div v-if="lightbox" class="guide-lightbox" @click="lightbox = ''">
      <img :src="lightbox" alt="" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAppStore, useAuthStore } from '@/stores'
import { guideAPI } from './api'
import { renderMarkdown, type GuideTocItem } from './render'
import '@/features/premium-home/runtime/premium-home.css'
import './guide.css'

const appStore = useAppStore()
const authStore = useAuthStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'AI Hub')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const isAuthenticated = computed(() => authStore.isAuthenticated)

const loading = ref(true)
const loadError = ref('')
const title = ref('')
const html = ref('')
const toc = ref<GuideTocItem[]>([])
const lightbox = ref('')
const updatedAt = ref<string | null>(null)
const activeId = ref('')
const contentRef = ref<HTMLElement | null>(null)

const updatedText = computed(() => {
  if (!updatedAt.value) return ''
  const date = new Date(updatedAt.value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
})

// Highlights the table-of-contents entry of the heading nearest the top of the
// viewport (below the sticky top bar) while the reader scrolls.
let headingObserver: IntersectionObserver | undefined
function watchHeadings() {
  headingObserver?.disconnect()
  const root = contentRef.value
  if (!root || typeof IntersectionObserver === 'undefined') return
  const headings = Array.from(root.querySelectorAll<HTMLElement>('h2[id], h3[id]'))
  if (headings.length === 0) return
  activeId.value = headings[0].id
  const visible = new Set<string>()
  headingObserver = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) visible.add(entry.target.id)
      else visible.delete(entry.target.id)
    })
    const first = headings.find((heading) => visible.has(heading.id))
    if (first) {
      activeId.value = first.id
      return
    }
    // Between headings: keep the last one that scrolled above the band.
    const passed = headings.filter((heading) => heading.getBoundingClientRect().top < 120)
    if (passed.length > 0) activeId.value = passed[passed.length - 1].id
  }, { rootMargin: '-96px 0px -60% 0px' })
  headings.forEach((heading) => headingObserver?.observe(heading))
}

function onContentClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (target.tagName === 'IMG') {
    lightbox.value = (target as HTMLImageElement).src
  }
}

onMounted(async () => {
  try {
    await appStore.fetchPublicSettings()
  } catch { /* brand falls back to defaults */ }
  try {
    const cfg = await guideAPI.getPublic()
    title.value = cfg.title
    updatedAt.value = cfg.is_default ? null : cfg.updated_at
    document.title = cfg.title
    const rendered = renderMarkdown(cfg.content_md)
    html.value = rendered.html
    toc.value = rendered.toc
  } catch {
    loadError.value = '文档中心暂时不可用，请稍后再试。'
  } finally {
    loading.value = false
  }
  await nextTick()
  watchHeadings()
})

onBeforeUnmount(() => headingObserver?.disconnect())
</script>
