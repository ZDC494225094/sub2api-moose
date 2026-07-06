import { defineConfig, loadEnv, Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function injectPublicSettings(backendUrl: string): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        try {
          const response = await fetch(`${backendUrl}/api/v1/settings/public`, {
            signal: AbortSignal.timeout(2000)
          })
          if (response.ok) {
            const data = await response.json()
            if (data.code === 0 && data.data) {
              const encoded = Buffer.from(JSON.stringify(data.data), 'utf8').toString('base64')
              const meta = `<meta name="app-config" content="${encoded}" />`
              return html.replace('</head>', `${meta}\n</head>`)
            }
          }
        } catch (e) {
          console.warn('[vite] 无法获取公开配置，将回退到 API 调用:', (e as Error).message)
        }
        return html
      }
    }
  }
}

function normalizeDevBackendUrl(value: string | undefined): string {
  return (value || '').trim().replace(/\/+$/, '')
}

async function isSub2APIBackend(url: string): Promise<boolean> {
  try {
    const response = await fetch(`${url}/health`, {
      headers: { Accept: 'application/json' },
      signal: AbortSignal.timeout(1200)
    })
    if (!response.ok) return false
    const text = await response.text()
    const data = JSON.parse(text) as { status?: unknown }
    return data.status === 'ok'
  } catch {
    return false
  }
}

async function resolveDevBackendUrl(preferredUrl: string | undefined): Promise<string> {
  const candidates = [
    normalizeDevBackendUrl(preferredUrl),
    'http://localhost:6200',
    'http://127.0.0.1:6200',
    'http://[::1]:6200',
    'http://127.0.0.1:8080',
    'http://localhost:8080',
    'http://127.0.0.1:8090'
  ].filter((item, index, arr) => item && arr.indexOf(item) === index)

  for (const candidate of candidates) {
    if (await isSub2APIBackend(candidate)) {
      return candidate
    }
  }

  const fallback = candidates[0] || 'http://localhost:6200'
  console.warn(`[vite] 未能探测到 Sub2API 后端，将继续使用: ${fallback}`)
  return fallback
}

export default defineConfig(async ({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const backendUrl = await resolveDevBackendUrl(env.VITE_DEV_PROXY_TARGET)
  const devPort = Number(env.VITE_DEV_PORT || 5173)
  const enableChecker = env.VITE_ENABLE_CHECKER === 'true'

  const plugins: Plugin[] = [vue()]
  if (enableChecker) {
    const { default: checker } = await import('vite-plugin-checker')
    plugins.push(checker({ vueTsc: true }))
  }
  plugins.push(injectPublicSettings(backendUrl))

  return {
    plugins: [
      ...plugins
    ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
    }
  },
  define: {
    // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
    // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
    __INTLIFY_JIT_COMPILATION__: true
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        /**
         * 手动分包配置
         * 分离第三方库并按功能合并应用代码，避免循环依赖
         */
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Vue 核心库
            if (
              id.includes('/vue/') ||
              id.includes('/vue-router/') ||
              id.includes('/pinia/') ||
              id.includes('/@vue/')
            ) {
              return 'vendor-vue'
            }

            // UI 工具库（较大，单独分离）
            if (id.includes('/@vueuse/') || id.includes('/xlsx/')) {
              return 'vendor-ui'
            }

            // 图表库
            if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
              return 'vendor-chart'
            }

            // 国际化
            if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
              return 'vendor-i18n'
            }

            // 其他小型第三方库合并
            return 'vendor-misc'
          }

          // 应用代码：按入口点自动分包，不手动干预
          // 这样可以避免循环依赖，同时保持合理的 chunk 数量
        }
      }
    }
  },
    server: {
      host: '127.0.0.1',
      port: devPort,
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1beta': {
          target: backendUrl,
          changeOrigin: true
        },
        '/openai': {
          target: backendUrl,
          changeOrigin: true
        },
        '/antigravity': {
          target: backendUrl,
          changeOrigin: true
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        },
        '/health': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})
