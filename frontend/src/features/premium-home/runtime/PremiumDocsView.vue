<template>
  <div class="premium-home docs-home">
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
          <RouterLink to="/">首页</RouterLink>
          <a href="/#plans">套餐服务</a>
          <RouterLink class="active" to="/docs">文档中心</RouterLink>
        </div>

        <div class="nav-actions">
          <RouterLink class="login-btn" :to="dashboardPath">{{ isAuthenticated ? '控制台' : '登录' }}</RouterLink>
          <RouterLink class="primary-btn" :to="isAuthenticated ? dashboardPath : '/register'">
            {{ isAuthenticated ? '进入控制台' : '注册 / 免费试用' }}
          </RouterLink>
        </div>
      </nav>
    </header>

    <main class="shell docs-shell">
      <section class="docs-hero">
        <div>
          <div class="eyebrow">
            <Icon name="book" size="sm" />
            MooseCloud Developer Docs
          </div>
          <h1>文档中心</h1>
          <p>从注册、生成 API Key，到 Codex、Claude Code、gpt-image-2 与 CCSwitch 配置，按步骤完成接入。</p>
        </div>
        <div class="docs-hero-card">
          <span>Base URL</span>
          <strong>https://moosecloud.cc</strong>
          <RouterLink class="primary-btn" :to="isAuthenticated ? dashboardPath : '/login'">打开控制台</RouterLink>
        </div>
      </section>

      <div class="docs-layout">
        <aside class="docs-sidebar" aria-label="文档目录">
          <div class="docs-sidebar-card">
            <p>入门指南</p>
            <a v-for="item in guideNav" :key="item.id" :href="`#${item.id}`">
              <Icon :name="item.icon" size="sm" />
              {{ item.title }}
            </a>
            <p>安装教程</p>
            <a v-for="item in installNav" :key="item.id" :href="`#${item.id}`">
              <Icon :name="item.icon" size="sm" />
              {{ item.title }}
            </a>
          </div>
        </aside>

        <article class="docs-content">
          <section id="register" class="doc-section doc-feature-section">
            <span class="soft-badge">第一步</span>
            <h2>注册与登录</h2>
            <p>访问 MooseCloud 控制台创建账户，完成邮箱验证后进入个人仪表盘。后续 API Key、套餐购买与调用统计都在控制台完成。</p>
            <div class="doc-callout">
              <Icon name="login" size="lg" />
              <div>
                <strong>创建您的控制台账户</strong>
                <span>注册后建议先确认账户信息，再创建 API Key 并选择对应渠道。</span>
              </div>
              <RouterLink class="secondary-btn" to="/login">前往登录</RouterLink>
            </div>
          </section>

          <section id="apikey" class="doc-section">
            <span class="soft-badge">身份凭证</span>
            <h2>生成 API Key</h2>
            <p>API Key 是调用 MooseCloud 服务的身份凭证，请妥善保管。如果怀疑泄露，请删除后重新创建。</p>
            <div class="doc-steps">
              <div v-for="step in apiKeySteps" :key="step.title" class="doc-step-card">
                <Icon :name="step.icon" size="lg" />
                <strong>{{ step.title }}</strong>
                <span>{{ step.desc }}</span>
              </div>
            </div>
          </section>

          <section id="npm" class="doc-section">
            <span class="soft-badge">环境准备</span>
            <h2>安装 Node.js</h2>
            <p>在安装 Codex CLI 或 Claude Code 之前，请先下载安装 Node.js。Windows 用户可直接下载官方 MSI 安装包，安装完成后系统会自动包含 node 与 npm。</p>
            <div class="doc-downloads single">
              <a href="https://nodejs.org/dist/v24.16.0/node-v24.16.0-x64.msi" target="_blank" rel="noopener noreferrer">
                <span>下载 Node.js v24.16.0 Windows x64 MSI</span>
                <Icon name="download" size="sm" />
              </a>
            </div>
            <pre class="doc-code"><code># 安装完成后打开命令行验证
node -v
npm -v</code></pre>
          </section>

          <section id="codex-cli" class="doc-section">
            <span class="soft-badge">OpenAI Codex</span>
            <h2>Codex CLI 安装命令</h2>
            <p>如果你希望在终端中直接调用 Codex CLI，可以使用下面的命令完成安装。</p>
            <pre class="doc-code"><code># 全局安装 Codex CLI
npm install -g @openai/codex

# 安装后查看版本
codex --version

# 每次使用打开命令行窗口，输入以下命令开始使用
codex</code></pre>
          </section>

          <section id="codex-app" class="doc-section doc-dark-panel">
            <div>
              <span class="soft-badge">桌面应用</span>
              <h2>Codex APP 安装</h2>
              <p>Codex APP 适合希望通过桌面界面管理对话、代码与工作区的用户，可与 CLI 配合使用。</p>
            </div>
            <ul class="doc-check-list">
              <li>下载并安装 Codex APP。</li>
              <li>安装 CCSwitch，配置 API 请求地址 <code>https://moosecloud.cc/</code> 和 API Key。</li>
              <li>启用 CCSwitch 配置后，重启 Codex CLI 或 Codex APP。</li>
            </ul>
            <a class="primary-btn" href="https://openai.com/zh-Hans-CN/codex/" target="_blank" rel="noopener noreferrer">下载应用</a>
          </section>

          <section id="claude-code" class="doc-section">
            <span class="soft-badge">Anthropic Claude</span>
            <h2>ClaudeCode CLI 安装教程</h2>
            <p>ClaudeCode CLI 是直接调用 Claude Code 系列模型的命令行工具，如需 Claude Code 工作流可安装。</p>
            <pre class="doc-code"><code># 全局安装 Claude Code
npm install -g @anthropic-ai/claude-code

# 安装后查看版本
claude --version

# 每次使用打开命令行窗口，输入以下命令开始使用
claude</code></pre>
          </section>

          <section id="gpt-image-2" class="doc-section">
            <span class="soft-badge">图像模型</span>
            <h2>gpt-image-2 使用方法</h2>
            <p>gpt-image-2 当前通过 <code>codex</code> 分组调用，支持生图与改图接口。调用时使用 OpenAI Image API 兼容端点，将模型名设置为 <code>gpt-image-2</code>。</p>
            <div class="doc-meta-grid">
              <div><span>分组名称</span><strong>codex</strong></div>
              <div><span>定价</span><strong>0.10 元 / 张</strong></div>
              <div><span>生图接口</span><strong>POST /v1/images/generations</strong></div>
              <div><span>改图接口</span><strong>POST /v1/images/edits</strong></div>
            </div>
            <pre class="doc-code"><code># 环境变量
export SUB2API_BASE='https://moosecloud.cc'
export SUB2API_KEY='sk-apikey'

# 使用 gpt-image-2 生成图片
curl $SUB2API_BASE/v1/images/generations \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一张未来感 AI 云平台控制台海报，干净明亮的科技风格",
    "size": "1024x1024",
    "quality": "high",
    "n": 1
  }'</code></pre>
          </section>

          <section id="ccswitch" class="doc-section">
            <span class="soft-badge">快速切换</span>
            <h2>CCSwitch 安装与配置</h2>
            <p>CCSwitch 可用于快速切换与配置 Claude Code、Codex 的 API Key 与请求路径。</p>
            <div class="doc-downloads">
              <a href="https://github.com/farion1231/cc-switch/releases/tag/v3.13.0" target="_blank" rel="noopener noreferrer">
                <span>macOS 打开发布页选择版本</span>
                <Icon name="download" size="sm" />
              </a>
              <a href="https://github.com/farion1231/cc-switch/releases/download/v3.13.0/CC-Switch-v3.13.0-Windows-Portable.zip" target="_blank" rel="noopener noreferrer">
                <span>Windows 版下载</span>
                <Icon name="download" size="sm" />
              </a>
            </div>
            <div class="doc-steps compact">
              <div class="doc-step-card">
                <Icon name="plus" size="lg" />
                <strong>切换类型并新增</strong>
                <span>切到 Codex 或 Claude，点击右上角 + 新增供应商。</span>
              </div>
              <div class="doc-step-card">
                <Icon name="key" size="lg" />
                <strong>填写地址和 Key</strong>
                <span>官网地址填写 https://moosecloud.cc/，再填入控制台生成的 API Key。</span>
              </div>
              <div class="doc-step-card">
                <Icon name="checkCircle" size="lg" />
                <strong>启用并重启</strong>
                <span>返回供应商列表启用配置，然后重启对应客户端。</span>
              </div>
            </div>
            <div class="doc-image-steps">
              <figure v-for="image in ccSwitchGuideImages" :key="image.title">
                <figcaption>{{ image.title }}</figcaption>
                <img :src="image.src" :alt="image.alt" loading="lazy" />
              </figure>
            </div>
          </section>
        </article>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import ccSwitchStep1 from './assets/docs/ccswitch-1.png'
import ccSwitchStep2 from './assets/docs/ccswitch-2.png'
import ccSwitchStep3 from './assets/docs/ccswitch-3.png'
import './premium-home.css'

type IconName = InstanceType<typeof Icon>['$props']['name']

const appStore = useAppStore()
const authStore = useAuthStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'AI Hub')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')

const guideNav: Array<{ id: string; title: string; icon: IconName }> = [
  { id: 'register', title: '注册与登录', icon: 'userPlus' },
  { id: 'apikey', title: '生成 API Key', icon: 'key' },
]

const installNav: Array<{ id: string; title: string; icon: IconName }> = [
  { id: 'npm', title: '安装 Node.js', icon: 'download' },
  { id: 'codex-cli', title: 'Codex CLI 安装命令', icon: 'terminal' },
  { id: 'codex-app', title: 'Codex APP 安装', icon: 'download' },
  { id: 'claude-code', title: 'ClaudeCode CLI', icon: 'cloud' },
  { id: 'gpt-image-2', title: 'gpt-image-2 调用', icon: 'sparkles' },
  { id: 'ccswitch', title: 'CCSwitch 安装与配置', icon: 'swap' },
]

const apiKeySteps: Array<{ title: string; desc: string; icon: IconName }> = [
  { title: '1. 点击 API 密钥页面', desc: '在仪表盘左侧菜单中选择“API密钥”。', icon: 'cog' },
  { title: '2. 创建密钥', desc: '点击“创建密钥”，设置备注名称，并选择对应渠道，Codex 默认选择 0.35 倍率渠道。', icon: 'plus' },
  { title: '3. 复制保存', desc: 'API Key 仅展示一次，请妥善保存。', icon: 'copy' },
]

const ccSwitchGuideImages = [
  { title: '步骤 1：切到 Codex 或者 Claude 并点击右上角 +', src: ccSwitchStep1, alt: 'CCSwitch 第一步截图' },
  { title: '步骤 2：填写官网地址和 API Key', src: ccSwitchStep2, alt: 'CCSwitch 第二步截图' },
  { title: '步骤 3：返回列表并点击启用', src: ccSwitchStep3, alt: 'CCSwitch 第三步截图' },
]

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings().catch(() => {})
  }
  authStore.checkAuth()
})
</script>
