<template>
  <AppLayout>
    <div class="archive-page mx-auto w-full max-w-7xl space-y-6">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-start gap-3 sm:gap-4">
          <div class="page-symbol"><Icon name="database" size="lg" /></div>
          <div class="min-w-0">
            <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">网关日志存储</h1>
            <p class="mt-1.5 text-sm leading-6 text-gray-500 dark:text-gray-400">
              按会话保存完整请求体与返回体（原始数据），同模型按天打包为 ZIP，滚动上传到存储桶
              <code class="path-sample">{{ samplePath }}</code>
            </p>
          </div>
        </div>
        <div class="flex shrink-0 gap-2">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="refreshAll">
            <Icon name="refresh" size="sm" />刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="busy || !status?.running" @click="flush">
            <Icon name="upload" size="sm" />立即封包上传
          </button>
        </div>
      </header>

      <p v-if="error" role="alert" class="feedback feedback-error">
        <Icon name="exclamationCircle" size="sm" /><span class="min-w-0 break-words">{{ error }}</span>
      </p>
      <p v-if="notice" role="status" class="feedback feedback-success">
        <Icon name="checkCircle" size="sm" /><span class="min-w-0 break-words">{{ notice }}</span>
      </p>

      <section v-if="status" class="archive-panel p-5">
        <div class="flex flex-wrap items-center gap-3">
          <span class="status-dot" :class="status.capturing ? 'bg-emerald-500' : 'bg-gray-400'" />
          <span class="font-medium text-gray-900 dark:text-gray-100">
            {{ status.capturing ? '正在采集' : '未在采集' }}
          </span>
          <span v-if="status.reason" class="text-sm text-amber-600 dark:text-amber-400">{{ status.reason }}</span>
          <span class="ml-auto text-xs text-gray-500 dark:text-gray-400">
            今日 {{ status.today }} · 数据目录 {{ status.data_dir }} · 剩余空间
            {{ status.disk_free_bytes >= 0 ? formatBytes(status.disk_free_bytes) : '未知' }}
          </span>
        </div>
        <dl class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8">
          <div v-for="[label, value] in metrics" :key="label" class="metric">
            <dt>{{ label }}</dt>
            <dd>{{ value }}</dd>
          </div>
        </dl>
        <div v-if="status.uploading" class="mt-4 text-sm text-gray-600 dark:text-gray-300">
          正在上传 {{ status.uploading.file_name }}：{{ formatBytes(status.uploading.sent) }} /
          {{ formatBytes(status.uploading.total) }}
          <div class="progress mt-2">
            <div class="progress-bar" :style="{ width: uploadPercent + '%' }" />
          </div>
        </div>
        <div v-if="status.open_bundles?.length" class="mt-4">
          <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-200">写入中的压缩包</h3>
          <ul class="mt-2 flex flex-wrap gap-2">
            <li v-for="b in status.open_bundles" :key="b.id" class="badge badge-gray">
              {{ b.day }}/{{ b.file_name }} · {{ b.entry_count }} 个会话 · {{ formatBytes(b.size_bytes) }}
            </li>
          </ul>
        </div>
        <p v-if="status.last_error" class="mt-3 break-words text-xs text-red-600 dark:text-red-400">
          最近错误（{{ formatTime(status.last_error_at) }}）：{{ status.last_error }}
        </p>
      </section>

      <nav class="tabs" role="tablist">
        <button
          v-for="name in tabs"
          :key="name"
          type="button"
          role="tab"
          class="tab"
          :class="{ 'tab-active': tab === name }"
          :aria-selected="tab === name"
          @click="tab = name"
        >
          {{ name }}
        </button>
      </nav>

      <!-- Configuration -->
      <section v-if="tab === '采集与存储配置' && form" class="space-y-6">
        <div class="archive-panel space-y-4 p-5">
          <h2 class="section-title">采集范围</h2>
          <label class="flex items-center gap-3">
            <input v-model="form.capture_enabled" type="checkbox" class="h-4 w-4" />
            <span class="text-sm text-gray-800 dark:text-gray-200">开启采集（需同时在「二开插件管理」中启用本扩展）</span>
          </label>
          <div class="flex flex-wrap gap-6">
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.mode" type="radio" value="all" />全量存储所有模型
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.mode" type="radio" value="selected" />只存储指定模型
            </label>
          </div>
          <div>
            <div class="flex items-center justify-between">
              <span class="input-label">模型规则</span>
              <button type="button" class="btn btn-secondary btn-sm" @click="addRule">
                <Icon name="plus" size="sm" />添加规则
              </button>
            </div>
            <p class="input-hint">
              匹配客户端请求的模型名，支持 * 通配（不区分大小写）。「压缩包名」相同的模型会打进同一个 ZIP，留空则使用模型名。全量模式下规则只用来合并命名。
            </p>
            <div v-for="(rule, i) in form.rules" :key="i" class="mt-2 flex flex-wrap items-center gap-2">
              <input v-model="rule.pattern" class="input flex-1" placeholder="如 claude-opus-5*" :aria-label="`规则 ${i + 1} 模型`" />
              <input v-model="rule.alias" class="input w-48" placeholder="压缩包名，如 Opus5" :aria-label="`规则 ${i + 1} 压缩包名`" />
              <button type="button" class="btn btn-ghost btn-sm" :aria-label="`删除规则 ${i + 1}`" @click="form.rules.splice(i, 1)">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>
          <label class="flex items-center gap-3">
            <input v-model="form.success_only" type="checkbox" class="h-4 w-4" />
            <span class="text-sm text-gray-800 dark:text-gray-200">只存储成功（2xx）的请求</span>
          </label>
          <div class="grid gap-4 sm:grid-cols-3">
            <label>
              <span class="input-label">单个 ZIP 上限（GB）</span>
              <input v-model.number="zipSizeGB" type="number" min="0.0625" step="1" class="input" />
            </label>
            <label>
              <span class="input-label">会话空闲结束（分钟）</span>
              <input v-model.number="idleMinutes" type="number" min="1" class="input" />
            </label>
            <label>
              <span class="input-label">单个请求/响应体上限（MB）</span>
              <input v-model.number="form.max_body_mb" type="number" min="1" max="256" class="input" />
            </label>
            <label>
              <span class="input-label">上传后本地保留（小时，0 为立即删除）</span>
              <input v-model.number="form.keep_local_hours" type="number" min="0" class="input" />
            </label>
            <label>
              <span class="input-label">后台索引保留（天）</span>
              <input v-model.number="form.entry_retention_days" type="number" min="1" class="input" />
            </label>
            <label>
              <span class="input-label">磁盘剩余低于（GB）暂停采集</span>
              <input v-model.number="form.min_free_disk_gb" type="number" min="0" class="input" />
            </label>
          </div>
        </div>

        <div class="archive-panel space-y-4 p-5">
          <h2 class="section-title">存储桶</h2>
          <div class="grid gap-4 sm:grid-cols-2">
            <label>
              <span class="input-label">服务商</span>
              <select v-model="form.provider" class="input">
                <option value="aws">Amazon Web Services / Amazon S3</option>
                <option value="s3-compatible">S3 兼容服务（R2 / OSS / MinIO 等）</option>
              </select>
            </label>
            <label>
              <span class="input-label">Bucket / 存储桶</span>
              <input v-model="form.bucket" class="input" placeholder="incoming-04-20260624" autocomplete="off" />
            </label>
            <label>
              <span class="input-label">Region / 地区代码</span>
              <input v-model="form.region" class="input" placeholder="us-east-1" autocomplete="off" />
            </label>
            <label>
              <span class="input-label">Endpoint / 服务地址</span>
              <input
                v-model="form.endpoint"
                class="input"
                :placeholder="form.provider === 'aws' ? '可留空，使用 AWS 默认地址' : 'https://...'"
                autocomplete="off"
              />
            </label>
            <label class="sm:col-span-2">
              <span class="input-label">S3 路径</span>
              <input v-model="form.prefix" class="input" placeholder="可留空；也可填 s3://bucket/前缀" autocomplete="off" />
              <span class="input-hint">文件上传到「前缀/日期/模型-日期.zip」，例如 {{ samplePath }}</span>
            </label>
            <label>
              <span class="input-label">Access Key ID / AK</span>
              <input v-model="accessKey" class="input" autocomplete="off" :placeholder="form.access_key_id || '未设置'" />
            </label>
            <label>
              <span class="input-label">Secret Access Key / SK</span>
              <input
                v-model="secretKey"
                type="password"
                class="input"
                autocomplete="new-password"
                :placeholder="form.secret_access_key_set ? '已设置，留空保持不变' : '未设置'"
              />
            </label>
            <label>
              <span class="input-label">Session Token</span>
              <input
                v-model="sessionToken"
                type="password"
                class="input"
                autocomplete="new-password"
                :placeholder="form.session_token_set ? '已设置，留空保持不变' : '可选，临时凭证使用'"
              />
            </label>
            <div class="flex flex-col justify-end gap-2">
              <label class="flex items-center gap-2 text-sm">
                <input v-model="form.force_path_style" type="checkbox" />使用 Path-Style 访问（MinIO 等需要）
              </label>
              <label v-if="form.session_token_set" class="flex items-center gap-2 text-sm">
                <input v-model="clearToken" type="checkbox" />清除已保存的 Session Token
              </label>
              <label v-if="form.secret_access_key_set" class="flex items-center gap-2 text-sm">
                <input v-model="clearCredentials" type="checkbox" />清除已保存的全部凭证（改用服务器环境凭证）
              </label>
            </div>
          </div>
          <div v-if="testSteps.length" class="space-y-1 text-sm">
            <p v-for="s in testSteps" :key="s.name" :class="s.ok ? 'text-emerald-600' : 'text-red-600'">
              {{ s.ok ? '✓' : '✗' }} {{ s.name }}<span v-if="s.error">：{{ s.error }}</span>
            </p>
          </div>
        </div>

        <div class="flex flex-wrap justify-end gap-3">
          <span v-if="form.updated_at" class="mr-auto self-center text-xs text-gray-500">
            最后保存：{{ formatTime(form.updated_at) }}
          </span>
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="testConfig">测试连接</button>
          <button type="button" class="btn btn-primary" :disabled="busy" @click="saveConfig">保存配置</button>
        </div>
      </section>

      <!-- Bundles -->
      <section v-if="tab === '压缩包'" class="space-y-4">
        <div class="flex flex-wrap gap-2">
          <input v-model="bundleFilter.day" type="date" class="input w-44" aria-label="日期" />
          <input v-model="bundleFilter.model" class="input w-48" placeholder="模型 / 压缩包名" aria-label="模型" />
          <select v-model="bundleFilter.status" class="input w-36" aria-label="状态">
            <option value="">全部状态</option>
            <option v-for="(label, key) in statusLabels" :key="key" :value="key">{{ label }}</option>
          </select>
          <button type="button" class="btn btn-secondary" @click="loadBundles(1)">查询</button>
        </div>
        <div class="table-wrapper">
          <table class="table">
            <thead>
              <tr>
                <th>日期</th>
                <th>文件</th>
                <th>状态</th>
                <th class="text-right">会话数</th>
                <th class="text-right">大小</th>
                <th>对象路径</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!bundles.items.length">
                <td colspan="7" class="py-8 text-center text-gray-500">暂无压缩包</td>
              </tr>
              <tr v-for="b in bundles.items" :key="b.id">
                <td class="whitespace-nowrap">{{ b.day }}</td>
                <td class="font-medium">{{ b.file_name }}</td>
                <td>
                  <span class="badge" :class="statusBadge(b.status)">{{ statusLabels[b.status] || b.status }}</span>
                  <p v-if="b.last_error" class="mt-1 max-w-xs break-words text-xs text-red-600" :title="b.last_error">
                    {{ b.last_error.slice(0, 120) }}
                  </p>
                </td>
                <td class="text-right">{{ b.entry_count }}</td>
                <td class="text-right whitespace-nowrap">{{ formatBytes(b.size_bytes) }}</td>
                <td class="max-w-xs break-all text-xs text-gray-500">
                  {{ b.object_key ? `s3://${b.bucket}/${b.object_key}` : '—' }}
                </td>
                <td class="whitespace-nowrap">
                  <button type="button" class="btn btn-ghost btn-sm" @click="openBundle(b)">查看数据</button>
                  <button v-if="b.status === 'uploaded'" type="button" class="btn btn-ghost btn-sm" @click="download(b)">下载</button>
                  <button
                    v-if="b.status === 'failed' && !b.local_deleted && b.sealed_at"
                    type="button"
                    class="btn btn-ghost btn-sm"
                    @click="retry(b)"
                  >
                    重试上传
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager :page="bundles.page" :total="bundles.total" :size="bundles.page_size" @change="loadBundles" />
      </section>

      <!-- Entries -->
      <section v-if="tab === '会话数据'" class="space-y-4">
        <div class="flex flex-wrap gap-2">
          <input v-model="entryFilter.day" type="date" class="input w-44" aria-label="日期" />
          <input v-model="entryFilter.model" class="input w-44" placeholder="模型" aria-label="模型" />
          <input v-model="entryFilter.session_id" class="input w-56" placeholder="会话 ID" aria-label="会话 ID" />
          <input v-model.number="entryFilter.user_id" type="number" min="0" class="input w-32" placeholder="用户 ID" aria-label="用户 ID" />
          <button type="button" class="btn btn-secondary" @click="loadEntries(1)">查询</button>
          <span v-if="entryFilter.bundle_id" class="badge badge-primary self-center">
            压缩包 #{{ entryFilter.bundle_id }}
            <button type="button" class="ml-1" aria-label="清除压缩包筛选" @click="clearBundleFilter">×</button>
          </span>
        </div>
        <div class="table-wrapper">
          <table class="table">
            <thead>
              <tr>
                <th>压缩包 / 文件</th>
                <th>会话 ID</th>
                <th>模型</th>
                <th class="text-right">轮次</th>
                <th class="text-right">大小</th>
                <th>用户 / Key</th>
                <th>时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!entries.items.length">
                <td colspan="8" class="py-8 text-center text-gray-500">暂无数据</td>
              </tr>
              <tr v-for="e in entries.items" :key="e.id">
                <td class="whitespace-nowrap text-xs">{{ e.day }}/{{ e.file_name }}<br />{{ e.entry_name }}</td>
                <td class="max-w-[14rem] break-all text-xs">{{ e.session_id }}</td>
                <td class="text-sm">{{ e.model }}</td>
                <td class="text-right">{{ e.exchange_count }}</td>
                <td class="text-right whitespace-nowrap">{{ formatBytes(e.uncompressed_size) }}</td>
                <td class="whitespace-nowrap text-xs">#{{ e.user_id }} / #{{ e.api_key_id }}</td>
                <td class="whitespace-nowrap text-xs">{{ formatTime(e.started_at) }}<br />{{ formatTime(e.ended_at) }}</td>
                <td class="whitespace-nowrap">
                  <button type="button" class="btn btn-ghost btn-sm" @click="preview(e)">预览</button>
                  <button type="button" class="btn btn-ghost btn-sm" @click="downloadEntry(e)">下载 JSON</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pager :page="entries.page" :total="entries.total" :size="entries.page_size" @change="loadEntries" />
      </section>
    </div>

    <BaseDialog :show="!!previewing" title="会话数据预览" width="extra-wide" @close="previewing = null">
      <div v-if="previewing" class="space-y-3">
        <p class="text-xs text-gray-500">
          {{ previewing.bundle.day }}/{{ previewing.bundle.file_name }} → {{ previewing.entry.entry_name }} · 会话
          {{ previewing.entry.session_id }} · {{ formatBytes(previewing.size) }}
          <span v-if="previewing.truncated" class="text-amber-600">（只显示前 {{ formatBytes(previewing.content.length) }}，完整内容请下载）</span>
        </p>
        <pre class="preview-body">{{ prettyContent }}</pre>
      </div>
      <template #footer>
        <button v-if="previewing" type="button" class="btn btn-secondary" @click="downloadEntry(previewing.entry)">下载 JSON</button>
        <button type="button" class="btn btn-primary" @click="previewing = null">关闭</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  archiveAPI,
  type ArchiveConfig,
  type ArchiveStatus,
  type Bundle,
  type Entry,
  type EntryContent,
  type Page,
} from './api'

const tabs = ['采集与存储配置', '压缩包', '会话数据'] as const
const tab = ref<(typeof tabs)[number]>('采集与存储配置')
const status = ref<ArchiveStatus | null>(null)
const form = ref<ArchiveConfig | null>(null)
const accessKey = ref('')
const secretKey = ref('')
const sessionToken = ref('')
const clearToken = ref(false)
const clearCredentials = ref(false)
const testSteps = ref<{ name: string; ok: boolean; error?: string }[]>([])
const busy = ref(false)
const error = ref('')
const notice = ref('')
const emptyPage = <T,>(): Page<T> => ({ items: [], total: 0, page: 1, page_size: 20 })
const bundles = ref<Page<Bundle>>(emptyPage<Bundle>())
const entries = ref<Page<Entry>>(emptyPage<Entry>())
const bundleFilter = reactive({ day: '', model: '', status: '' })
const entryFilter = reactive<{ day: string; model: string; session_id: string; user_id: number | ''; bundle_id: number }>({
  day: '',
  model: '',
  session_id: '',
  user_id: '',
  bundle_id: 0,
})
const previewing = ref<EntryContent | null>(null)

const statusLabels: Record<string, string> = {
  open: '写入中',
  sealed: '待上传',
  uploading: '上传中',
  uploaded: '已上传',
  failed: '失败',
}

const Pager = defineComponent({
  props: { page: { type: Number, required: true }, total: { type: Number, required: true }, size: { type: Number, required: true } },
  emits: ['change'],
  setup(props, { emit }) {
    return () => {
      const pages = Math.max(1, Math.ceil(props.total / props.size))
      return h('div', { class: 'flex items-center justify-end gap-3 text-sm text-gray-500' }, [
        h('span', `共 ${props.total} 条`),
        h('button', { type: 'button', class: 'btn btn-secondary btn-sm', disabled: props.page <= 1, onClick: () => emit('change', props.page - 1) }, '上一页'),
        h('span', `${props.page} / ${pages}`),
        h('button', { type: 'button', class: 'btn btn-secondary btn-sm', disabled: props.page >= pages, onClick: () => emit('change', props.page + 1) }, '下一页'),
      ])
    }
  },
})

const zipSizeGB = computed({
  get: () => (form.value ? Math.round((form.value.max_zip_size_mb / 1024) * 1000) / 1000 : 10),
  set: (v: number) => {
    if (form.value) form.value.max_zip_size_mb = Math.round((Number(v) || 0) * 1024)
  },
})
const idleMinutes = computed({
  get: () => (form.value ? Math.round(form.value.session_idle_seconds / 60) : 30),
  set: (v: number) => {
    if (form.value) form.value.session_idle_seconds = Math.round((Number(v) || 0) * 60)
  },
})

const samplePath = computed(() => {
  const today = status.value?.today || new Date().toISOString().slice(0, 10)
  const bucket = form.value?.bucket || 'bucket'
  const prefix = (form.value?.prefix || '').replace(/^s3:\/\/[^/]*\/?/, '').replace(/^\/+|\/+$/g, '')
  return `s3://${bucket}/${prefix ? prefix + '/' : ''}${today}/Opus5-${today}.zip`
})

const metrics = computed<[string, string | number][]>(() => {
  const s = status.value
  if (!s) return []
  const uploaded = s.bundles?.find(b => b.status === 'uploaded')
  return [
    ['活跃会话', s.active_sessions],
    ['待写入', s.queued_exchanges + s.pending_flushes],
    ['本次采集请求', s.captured],
    ['归档会话', s.sessions_archived],
    ['已上传 ZIP', uploaded?.count ?? 0],
    ['已上传数据', formatBytes(uploaded?.bytes ?? 0)],
    ['丢弃', s.dropped],
    ['上传失败', s.upload_errors],
  ]
})

const uploadPercent = computed(() => {
  const up = status.value?.uploading
  return up && up.total > 0 ? Math.min(100, Math.round((up.sent / up.total) * 100)) : 0
})

const prettyContent = computed(() => {
  const p = previewing.value
  if (!p) return ''
  if (p.truncated) return p.content
  try {
    return JSON.stringify(JSON.parse(p.content), null, 2)
  } catch {
    return p.content
  }
})

function formatBytes(n: number) {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 2)} ${units[i]}`
}

function formatTime(v: string | null | undefined) {
  if (!v) return '—'
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? v : d.toLocaleString('zh-CN', { hour12: false })
}

function statusBadge(s: string) {
  return { uploaded: 'badge-success', failed: 'badge-danger', uploading: 'badge-primary', sealed: 'badge-warning' }[s] || 'badge-gray'
}

function message(e: unknown, fallback: string) {
  const m = (e as { message?: string })?.message
  return m && typeof m === 'string' ? m : fallback
}

async function run(action: () => Promise<void>, fallback: string) {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await action()
  } catch (e) {
    error.value = message(e, fallback)
  } finally {
    busy.value = false
  }
}

function applyConfig(cfg: ArchiveConfig) {
  form.value = { ...cfg, rules: (cfg.rules || []).map(r => ({ ...r })) }
  accessKey.value = ''
  secretKey.value = ''
  sessionToken.value = ''
  clearToken.value = false
  clearCredentials.value = false
}

function configInput() {
  const cfg = form.value!
  return {
    ...cfg,
    access_key_id: accessKey.value.trim(),
    secret_access_key: secretKey.value.trim(),
    session_token: sessionToken.value.trim(),
    clear_credentials: clearCredentials.value,
    clear_session_token: clearToken.value,
  }
}

async function loadStatus() {
  try {
    status.value = await archiveAPI.status()
  } catch (e) {
    error.value = message(e, '加载运行状态失败')
  }
}

async function loadConfig() {
  applyConfig(await archiveAPI.getConfig())
}

async function saveConfig() {
  await run(async () => {
    applyConfig(await archiveAPI.saveConfig(configInput()))
    notice.value = '配置已保存，几秒内生效。'
    setTimeout(loadStatus, 2500)
  }, '保存失败')
}

async function testConfig() {
  await run(async () => {
    const result = await archiveAPI.testConfig(configInput())
    testSteps.value = result.steps
    if (result.ok) notice.value = '连接测试通过，可以写入存储桶。'
    else error.value = '连接测试未通过，请检查下方步骤。'
  }, '测试失败')
}

async function flush() {
  await run(async () => {
    status.value = await archiveAPI.flush()
    notice.value = '已结束当前会话并封包，压缩包正在上传。'
    await loadBundles(1)
  }, '操作失败')
}

async function loadBundles(page = bundles.value.page) {
  await run(async () => {
    bundles.value = await archiveAPI.bundles({
      page,
      page_size: 20,
      day: bundleFilter.day || undefined,
      model: bundleFilter.model.trim() || undefined,
      status: bundleFilter.status || undefined,
    })
  }, '加载压缩包失败')
}

async function loadEntries(page = entries.value.page) {
  await run(async () => {
    entries.value = await archiveAPI.entries({
      page,
      page_size: 20,
      bundle_id: entryFilter.bundle_id || undefined,
      day: entryFilter.day || undefined,
      model: entryFilter.model.trim() || undefined,
      session_id: entryFilter.session_id.trim() || undefined,
      user_id: entryFilter.user_id || undefined,
    })
  }, '加载会话数据失败')
}

function openBundle(b: Bundle) {
  Object.assign(entryFilter, { bundle_id: b.id, day: '', model: '', session_id: '', user_id: '' })
  tab.value = '会话数据'
  void loadEntries(1)
}

function clearBundleFilter() {
  entryFilter.bundle_id = 0
  void loadEntries(1)
}

async function retry(b: Bundle) {
  await run(async () => {
    await archiveAPI.retry(b.id)
    notice.value = `${b.file_name} 已重新加入上传队列。`
    await loadBundles()
  }, '重试失败')
}

async function download(b: Bundle) {
  await run(async () => {
    const url = await archiveAPI.downloadURL(b.id)
    window.open(url, '_blank', 'noopener')
  }, '获取下载地址失败')
}

async function preview(e: Entry) {
  await run(async () => {
    previewing.value = await archiveAPI.entryContent(e.id)
  }, '读取数据失败')
}

async function downloadEntry(e: Entry) {
  await run(async () => {
    const blob = await archiveAPI.downloadEntry(e.id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${e.file_name.replace(/\.zip$/, '')}-${e.entry_name}`
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }, '下载失败')
}

async function refreshAll() {
  await loadStatus()
  if (tab.value === '压缩包') await loadBundles()
  if (tab.value === '会话数据') await loadEntries()
}

watch(tab, value => {
  if (value === '压缩包' && !bundles.value.items.length) void loadBundles(1)
  if (value === '会话数据' && !entries.value.items.length) void loadEntries(1)
})

let timer: ReturnType<typeof setInterval> | undefined
onMounted(async () => {
  await run(async () => {
    await Promise.all([loadConfig(), loadStatus()])
  }, '加载配置失败')
  timer = setInterval(loadStatus, 10000)
})
onUnmounted(() => clearInterval(timer))

function addRule() {
  form.value?.rules.push({ pattern: '', alias: '' })
}
</script>

<style scoped>
.page-symbol {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: #eef2ff;
  color: #4f46e5;
  flex-shrink: 0;
}

.archive-panel {
  border: 1px solid #e5e7eb;
  border-radius: 16px;
  background: #fff;
}

:global(.dark) .archive-panel {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
}

.section-title {
  font-size: 15px;
  font-weight: 600;
}

.status-dot {
  display: inline-block;
  width: 9px;
  height: 9px;
  border-radius: 999px;
}

.metric {
  border-radius: 12px;
  background: rgb(249 250 251);
  padding: 10px 12px;
}

:global(.dark) .metric {
  background: rgb(31 41 55);
}

.metric dt {
  font-size: 12px;
  color: #6b7280;
}

.metric dd {
  margin-top: 2px;
  font-size: 17px;
  font-weight: 600;
}

.path-sample {
  font-size: 12px;
  word-break: break-all;
}

.feedback {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 14px;
}

.feedback-error {
  background: #fef2f2;
  color: #b91c1c;
}

.feedback-success {
  background: #f0fdf4;
  color: #15803d;
}

.preview-body {
  max-height: 65vh;
  overflow: auto;
  border-radius: 10px;
  background: #0f172a;
  color: #e2e8f0;
  padding: 14px;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
