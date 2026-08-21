<template>
  <main class="access-restricted-page">
    <section class="access-restricted-content" aria-labelledby="access-restricted-title">
      <header class="access-restricted-hero">
        <div class="access-restricted-logo">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" />
        </div>
        <p class="access-restricted-code">HTTP 451</p>
        <h1 id="access-restricted-title">依法限制访问</h1>
        <p class="access-restricted-status">访问受限</p>
      </header>

      <div class="access-restricted-rule"></div>

      <section class="access-restricted-section" aria-labelledby="mainland-china-notice-title">
        <h2 id="mainland-china-notice-title">本站点不向中国大陆地区提供服务</h2>
        <p>
          本站点遵循中华人民共和国《生成式人工智能服务管理暂行办法》及相关法律法规的要求，不面向中国大陆境内公众提供生成式人工智能服务，并已对来自中国大陆地区的网络访问予以限制。
        </p>
        <a
          class="access-restricted-law-link"
          href="https://www.gov.cn/zhengce/zhengceku/202307/content_6891752.htm"
          target="_blank"
          rel="noopener noreferrer"
        >
          法规依据 · 官方发布 《生成式人工智能服务管理暂行办法》（国家互联网信息办公室等七部门联合发布）
        </a>
      </section>

      <div class="access-restricted-rule"></div>

      <section class="access-restricted-section" aria-labelledby="english-notice-title">
        <p class="access-restricted-english-message">
          In accordance with the Interim Measures for the Administration of Generative Artificial Intelligence Services and related laws and regulations, this service is not available to users in mainland China.
        </p>
        <h2 id="english-notice-title" class="access-restricted-english-title">
          Error 451 · Unavailable For Legal Reasons
        </h2>
        <p class="access-restricted-english-status">Access Restricted</p>
      </section>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '@/stores/app'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const siteLogo = computed(() => sanitizeUrl(
  appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '',
  { allowRelative: true, allowDataUrl: true },
))
</script>

<style scoped>
.access-restricted-page {
  min-height: 100svh;
  background: #1f1f1f;
  color: #f5f5f4;
  padding: 4.25rem 1.5rem 0.375rem;
}

.access-restricted-content {
  width: min(100%, 48rem);
  margin: 0 auto;
}

.access-restricted-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.access-restricted-logo {
  display: flex;
  width: 3.75rem;
  height: 3.75rem;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 0.875rem;
  background: #fff;
}

.access-restricted-logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.access-restricted-code {
  margin: 1.5rem 0 0;
  color: #f4f4f5;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.9375rem;
  font-weight: 800;
  letter-spacing: 0.12em;
}

.access-restricted-hero h1 {
  margin: 0.375rem 0 0;
  color: #fff;
  font-size: clamp(2rem, 6vw, 2.5rem);
  font-weight: 800;
  line-height: 1.2;
}

.access-restricted-status {
  margin: 0.35rem 0 0;
  color: #c8c8c8;
  font-size: 1rem;
  font-weight: 600;
}

.access-restricted-rule {
  height: 1px;
  margin: 2.5rem 0;
  background: #383838;
}

.access-restricted-section {
  padding: 0 2.5rem;
}

.access-restricted-section h2 {
  margin: 0;
  color: #fff;
  font-size: clamp(1.125rem, 3.8vw, 1.25rem);
  font-weight: 800;
  line-height: 1.4;
}

.access-restricted-section p {
  margin: 1rem 0 0;
  color: #d0d0d0;
  font-size: 1rem;
  line-height: 1.6;
}

.access-restricted-law-link {
  display: inline;
  color: #fff;
  font-size: 0.875rem;
  font-weight: 700;
  line-height: 1.65;
  text-decoration: underline;
  text-decoration-thickness: 1px;
  text-underline-offset: 0.25rem;
}

.access-restricted-law-link:hover {
  color: #d4d4d8;
}

.access-restricted-law-link:focus-visible {
  outline: 2px solid #fff;
  outline-offset: 0.25rem;
}

.access-restricted-english-message {
  margin-top: 0;
}

.access-restricted-section .access-restricted-english-title {
  margin-top: 1.5rem;
}

.access-restricted-english-status {
  margin-top: 0.35rem;
  color: #c8c8c8;
  font-size: 1rem;
  font-weight: 600;
}

.access-restricted-section + .access-restricted-rule {
  margin-top: 1.875rem;
  margin-bottom: 2.125rem;
}

@media (max-width: 40rem) {
  .access-restricted-page {
    padding: 3rem 1.25rem;
  }

  .access-restricted-section {
    padding: 0;
  }

  .access-restricted-rule {
    margin: 2rem 0;
  }
}
</style>
