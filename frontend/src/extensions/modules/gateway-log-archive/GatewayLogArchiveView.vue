<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-6">
      <header class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div class="flex min-w-0 items-start gap-4">
          <div class="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-900/20 dark:text-primary-300 dark:ring-primary-800/40">
            <Icon name="database" size="lg" />
          </div>
          <div class="min-w-0">
            <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">网关日志存储</h1>
            <p class="mt-1 max-w-3xl text-sm leading-6 text-gray-500 dark:text-dark-400">
              按会话保存完整请求体与返回体（原始数据），同模型按天打包为 ZIP，按优先级上传到存储桶。
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

      <p v-if="error" role="alert" class="flex items-start gap-2 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
        <Icon name="exclamationCircle" size="sm" class="mt-0.5 shrink-0" /><span class="min-w-0 break-words">{{ error }}</span>
      </p>
      <p v-if="notice" role="status" class="flex items-start gap-2 rounded-xl bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">
        <Icon name="checkCircle" size="sm" class="mt-0.5 shrink-0" /><span class="min-w-0 break-words">{{ notice }}</span>
      </p>

      <section v-if="status" class="card p-5">
        <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
          <span
            class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium"
            :class="status.capturing
              ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
              : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'"
          >
            <span class="h-2 w-2 rounded-full" :class="status.capturing ? 'animate-pulse bg-emerald-500' : 'bg-gray-400'" />
            {{ status.capturing ? '正在采集' : '未在采集' }}
          </span>
          <span v-if="status.reason" class="text-sm text-amber-600 dark:text-amber-400">{{ status.reason }}</span>
          <dl class="flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-500 dark:text-dark-400 lg:ml-auto">
            <div class="flex gap-1"><dt>今日</dt><dd class="font-mono text-gray-700 dark:text-gray-300">{{ status.today }}</dd></div>
            <div class="flex gap-1">
              <dt>剩余空间</dt>
              <dd class="font-mono text-gray-700 dark:text-gray-300">{{ status.disk_free_bytes >= 0 ? formatBytes(status.disk_free_bytes) : '未知' }}</dd>
            </div>
            <div class="flex min-w-0 gap-1" :title="status.data_dir">
              <dt class="shrink-0">数据目录</dt><dd class="max-w-[16rem] truncate font-mono text-gray-700 dark:text-gray-300">{{ status.data_dir }}</dd>
            </div>
          </dl>
        </div>

        <div class="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <button type="button" class="kpi kpi-action text-left" @click="tab = 'stats'">
            <span class="kpi-label"><Icon name="cloud" size="sm" />已上传总量</span>
            <span class="kpi-value">{{ formatBytes(status.uploaded_bytes) }}</span>
            <span class="kpi-sub">{{ status.uploaded_bundles.toLocaleString() }} 个 ZIP · <span class="text-primary-600 dark:text-primary-400">按模型查看 ›</span></span>
          </button>
          <button type="button" class="kpi kpi-action text-left" @click="tab = 'buckets'">
            <span class="kpi-label"><Icon name="server" size="sm" />可用存储桶</span>
            <span class="kpi-value" :class="{ 'text-amber-600 dark:text-amber-400': !status.storage_buckets }">{{ status.storage_buckets }}</span>
            <span class="kpi-sub">共 {{ buckets.length }} 个 · 失败自动切换 ›</span>
          </button>
          <div class="kpi">
            <span class="kpi-label"><Icon name="bolt" size="sm" />活跃会话</span>
            <span class="kpi-value">{{ status.active_sessions }}</span>
            <span class="kpi-sub">待写入 {{ status.queued_exchanges + status.pending_flushes }} 条</span>
          </div>
          <div class="kpi">
            <span class="kpi-label"><Icon name="chart" size="sm" />本次运行</span>
            <span class="kpi-value">{{ status.sessions_archived.toLocaleString() }} <small class="text-sm font-medium text-gray-500 dark:text-dark-400">个会话</small></span>
            <span class="kpi-sub">
              采集 {{ status.captured.toLocaleString() }} · 丢弃 {{ status.dropped }} ·
              <span :class="{ 'text-red-600 dark:text-red-400': status.upload_errors > 0 }">上传失败 {{ status.upload_errors }}</span>
            </span>
          </div>
        </div>

        <div v-if="status.uploading" class="mt-4 rounded-xl border border-primary-100 bg-primary-50/50 p-4 dark:border-primary-900/40 dark:bg-primary-900/10">
          <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
            <span class="font-medium text-gray-800 dark:text-gray-200">正在上传 {{ status.uploading.file_name }}</span>
            <span class="font-mono text-xs text-gray-500 dark:text-dark-400">
              {{ formatBytes(status.uploading.sent) }} / {{ formatBytes(status.uploading.total) }} · {{ uploadPercent }}%
            </span>
          </div>
          <div class="progress mt-2"><div class="progress-bar" :style="{ width: uploadPercent + '%' }" /></div>
        </div>
        <div v-if="status.open_bundles?.length" class="mt-4">
          <h3 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">写入中的压缩包</h3>
          <ul class="mt-2 flex flex-wrap gap-2">
            <li v-for="b in status.open_bundles" :key="b.id" class="badge badge-gray font-normal">
              {{ b.day }}/{{ b.file_name }} · {{ b.entry_count }} 个会话 · {{ formatBytes(b.size_bytes) }}
            </li>
          </ul>
        </div>
        <p v-if="status.last_error" class="mt-4 break-words rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/30 dark:text-red-300">
          最近错误（{{ formatTime(status.last_error_at) }}）：{{ status.last_error }}
        </p>
      </section>

      <nav class="tabs max-w-full overflow-x-auto sm:w-fit" role="tablist">
        <button
          v-for="item in tabs"
          :key="item.key"
          type="button"
          role="tab"
          class="tab whitespace-nowrap"
          :class="{ 'tab-active': tab === item.key }"
          :aria-selected="tab === item.key"
          @click="tab = item.key"
        >
          {{ item.label }}<span v-if="item.key === 'buckets' && buckets.length" class="ml-1.5 text-xs opacity-70">{{ buckets.length }}</span>
        </button>
      </nav>

      <!-- Capture settings -->
      <section v-if="tab === 'capture' && form" class="grid gap-6 lg:grid-cols-5">
        <div class="card space-y-6 p-5 lg:col-span-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">采集范围</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">需同时在「二开插件管理」中启用本扩展，并至少启用一个存储桶。</p>
          </div>
          <div class="setting-row">
            <div class="min-w-0">
              <p class="setting-title">开启采集</p>
              <p class="setting-desc">记录网关对话接口的原始请求与返回</p>
            </div>
            <Toggle v-model="form.capture_enabled" />
          </div>
          <div class="setting-row">
            <div class="min-w-0">
              <p class="setting-title">只存储成功的请求</p>
              <p class="setting-desc">仅保存 2xx 响应</p>
            </div>
            <Toggle v-model="form.success_only" />
          </div>
          <div>
            <p class="setting-title">存储模式</p>
            <div class="tabs mt-2 w-fit">
              <button type="button" class="tab" :class="{ 'tab-active': form.mode === 'all' }" @click="form.mode = 'all'">全量存储所有模型</button>
              <button type="button" class="tab" :class="{ 'tab-active': form.mode === 'selected' }" @click="form.mode = 'selected'">只存储指定模型</button>
            </div>
          </div>
          <div>
            <div class="flex items-center justify-between gap-3">
              <p class="setting-title">模型规则</p>
              <button type="button" class="btn btn-secondary btn-sm" @click="addRule"><Icon name="plus" size="sm" />添加规则</button>
            </div>
            <p class="setting-desc mt-1">
              按客户端请求的模型名匹配，支持 * 通配（不区分大小写）。「压缩包名」相同的模型打进同一个 ZIP，留空用模型名；全量模式下规则只用于合并命名。
            </p>
            <div class="mt-3 overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
              <div v-if="!form.rules.length" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                {{ form.mode === 'selected' ? '“指定模型”模式至少需要一条规则' : '暂无规则，所有模型按原名打包' }}
              </div>
              <div
                v-for="(rule, i) in form.rules"
                :key="i"
                class="flex flex-wrap items-center gap-2 border-b border-gray-100 px-3 py-2 last:border-b-0 dark:border-dark-700"
              >
                <input v-model="rule.pattern" class="input min-w-[12rem] flex-1 font-mono" placeholder="claude-opus-5*" :aria-label="`规则 ${i + 1} 模型`" />
                <Icon name="chevronRight" size="sm" class="text-gray-400" />
                <input v-model="rule.alias" class="input w-44 font-mono" placeholder="压缩包名，如 Opus5" :aria-label="`规则 ${i + 1} 压缩包名`" />
                <button type="button" class="btn btn-ghost btn-sm" :aria-label="`删除规则 ${i + 1}`" @click="form.rules.splice(i, 1)">
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="card space-y-4 p-5 lg:col-span-2">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">打包与保留</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">控制 ZIP 大小、会话切分与本地磁盘占用。</p>
          </div>
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
            <label>
              <span class="input-label">单个 ZIP 上限（GB）</span>
              <input v-model.number="zipSizeGB" type="number" min="0.0625" step="1" class="input" />
            </label>
            <label>
              <span class="input-label">会话空闲结束（分钟）</span>
              <input v-model.number="idleMinutes" type="number" min="1" class="input" />
            </label>
            <label>
              <span class="input-label">请求/响应体上限（MB）</span>
              <input v-model.number="form.max_body_mb" type="number" min="1" max="256" class="input" />
            </label>
            <label>
              <span class="input-label">上传后本地保留（小时）</span>
              <input v-model.number="form.keep_local_hours" type="number" min="0" class="input" />
              <span class="input-hint">0 为上传后立即删除</span>
            </label>
            <label>
              <span class="input-label">后台索引保留（天）</span>
              <input v-model.number="form.entry_retention_days" type="number" min="1" class="input" />
              <span class="input-hint">不影响上传统计</span>
            </label>
            <label>
              <span class="input-label">磁盘低于（GB）暂停采集</span>
              <input v-model.number="form.min_free_disk_gb" type="number" min="0" class="input" />
            </label>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-end gap-3 lg:col-span-5">
          <span v-if="form.updated_at" class="mr-auto text-xs text-gray-500 dark:text-dark-400">最后保存：{{ formatTime(form.updated_at) }}</span>
          <button type="button" class="btn btn-primary" :disabled="busy" @click="saveConfig">保存采集配置</button>
        </div>
      </section>

      <BucketsPanel v-if="tab === 'buckets'" :today="today" @changed="onBucketsChanged" />

      <ModelStatsPanel v-if="tab === 'stats'" :buckets="buckets" @view-bundles="viewModelBundles" />

      <!-- Bundles -->
      <section v-if="tab === 'bundles'" class="space-y-4">
        <div class="card flex flex-wrap items-center gap-2 p-3">
          <input v-model="bundleFilter.day" type="date" class="input w-44" aria-label="日期" />
          <input v-model="bundleFilter.model" class="input w-48" placeholder="模型 / 压缩包名" aria-label="模型" @keyup.enter="loadBundles(1)" />
          <select v-model="bundleFilter.status" class="input w-36" aria-label="状态">
            <option value="">全部状态</option>
            <option v-for="(label, key) in statusLabels" :key="key" :value="key">{{ label }}</option>
          </select>
          <button type="button" class="btn btn-secondary" @click="loadBundles(1)"><Icon name="search" size="sm" />查询</button>
        </div>
        <div class="card overflow-hidden">
          <div class="overflow-x-auto">
            <table class="table min-w-[980px]">
              <thead>
                <tr>
                  <th>日期</th>
                  <th>文件</th>
                  <th>状态</th>
                  <th class="!text-right">会话数</th>
                  <th class="!text-right">大小</th>
                  <th>存储位置</th>
                  <th class="!text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!bundles.items.length">
                  <td colspan="7" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无压缩包</td>
                </tr>
                <tr v-for="b in bundles.items" :key="b.id">
                  <td class="whitespace-nowrap font-mono text-xs">{{ b.day }}</td>
                  <td class="font-medium text-gray-900 dark:text-white">{{ b.file_name }}</td>
                  <td>
                    <span class="badge" :class="statusBadge(b.status)">{{ statusLabels[b.status] || b.status }}</span>
                    <p v-if="b.last_error" class="mt-1 max-w-xs break-words text-xs text-red-600 dark:text-red-400" :title="b.last_error">
                      {{ b.last_error.slice(0, 160) }}
                    </p>
                  </td>
                  <td class="text-right">{{ b.entry_count }}</td>
                  <td class="whitespace-nowrap text-right">{{ formatBytes(b.size_bytes) }}</td>
                  <td class="max-w-xs text-xs">
                    <template v-if="b.object_key">
                      <p class="font-medium text-gray-700 dark:text-gray-200">{{ bucketLabel(b) }}</p>
                      <p class="break-all font-mono text-gray-500 dark:text-dark-400">s3://{{ b.bucket }}/{{ b.object_key }}</p>
                    </template>
                    <span v-else class="text-gray-400">—</span>
                  </td>
                  <td class="whitespace-nowrap text-right">
                    <button type="button" class="btn btn-ghost btn-sm" @click="openBundle(b)">查看数据</button>
                    <button v-if="b.status === 'uploaded'" type="button" class="btn btn-ghost btn-sm" @click="download(b)">下载</button>
                    <button v-if="b.status === 'failed' && !b.local_deleted && b.sealed_at" type="button" class="btn btn-ghost btn-sm" @click="retry(b)">
                      重试上传
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <Pager :page="bundles.page" :total="bundles.total" :size="bundles.page_size" @change="loadBundles" />
      </section>

      <!-- Entries -->
      <section v-if="tab === 'entries'" class="space-y-4">
        <div class="card flex flex-wrap items-center gap-2 p-3">
          <input v-model="entryFilter.day" type="date" class="input w-44" aria-label="日期" />
          <input v-model="entryFilter.model" class="input w-44" placeholder="模型" aria-label="模型" @keyup.enter="loadEntries(1)" />
          <input v-model="entryFilter.session_id" class="input w-56" placeholder="会话 ID" aria-label="会话 ID" @keyup.enter="loadEntries(1)" />
          <input v-model.number="entryFilter.user_id" type="number" min="0" class="input w-32" placeholder="用户 ID" aria-label="用户 ID" />
          <button type="button" class="btn btn-secondary" @click="loadEntries(1)"><Icon name="search" size="sm" />查询</button>
          <span v-if="entryFilter.bundle_id" class="badge badge-primary">
            压缩包 #{{ entryFilter.bundle_id }}
            <button type="button" class="ml-1" aria-label="清除压缩包筛选" @click="clearBundleFilter">×</button>
          </span>
        </div>
        <div class="card overflow-hidden">
          <div class="overflow-x-auto">
            <table class="table min-w-[1040px]">
              <thead>
                <tr>
                  <th>压缩包 / 文件</th>
                  <th>会话 ID</th>
                  <th>模型</th>
                  <th class="!text-right">轮次</th>
                  <th class="!text-right">大小</th>
                  <th>用户 / Key</th>
                  <th>时间</th>
                  <th class="!text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!entries.items.length">
                  <td colspan="8" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">暂无数据</td>
                </tr>
                <tr v-for="e in entries.items" :key="e.id">
                  <td class="whitespace-nowrap text-xs">
                    <p class="font-medium text-gray-800 dark:text-gray-200">{{ e.day }}/{{ e.file_name }}</p>
                    <p class="font-mono text-gray-500 dark:text-dark-400">{{ e.entry_name }}</p>
                  </td>
                  <td class="max-w-[14rem] break-all font-mono text-xs">{{ e.session_id || '—' }}</td>
                  <td class="text-sm">{{ e.model }}</td>
                  <td class="text-right">{{ e.exchange_count }}</td>
                  <td class="whitespace-nowrap text-right">{{ formatBytes(e.uncompressed_size) }}</td>
                  <td class="whitespace-nowrap font-mono text-xs">#{{ e.user_id }} / #{{ e.api_key_id }}</td>
                  <td class="whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ formatTime(e.started_at) }}<br />{{ formatTime(e.ended_at) }}</td>
                  <td class="whitespace-nowrap text-right">
                    <button type="button" class="btn btn-ghost btn-sm" @click="preview(e)">预览</button>
                    <button type="button" class="btn btn-ghost btn-sm" @click="downloadEntry(e)">下载 JSON</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <Pager :page="entries.page" :total="entries.total" :size="entries.page_size" @change="loadEntries" />
      </section>
    </div>

    <BaseDialog :show="!!previewing" title="会话数据预览" width="extra-wide" @close="previewing = null">
      <div v-if="previewing" class="space-y-3">
        <p class="text-xs text-gray-500 dark:text-dark-400">
          {{ previewing.bundle.day }}/{{ previewing.bundle.file_name }} → {{ previewing.entry.entry_name }} · 会话
          {{ previewing.entry.session_id || '—' }} · {{ formatBytes(previewing.size) }}
          <span v-if="previewing.truncated" class="text-amber-600 dark:text-amber-400">（只显示前 {{ formatBytes(previewing.content.length) }}，完整内容请下载）</span>
        </p>
        <pre class="max-h-[65vh] overflow-auto whitespace-pre-wrap break-all rounded-xl bg-gray-900 p-4 font-mono text-[12.5px] leading-relaxed text-gray-100 dark:bg-dark-950">{{ prettyContent }}</pre>
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
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import BucketsPanel from './BucketsPanel.vue'
import ModelStatsPanel from './ModelStatsPanel.vue'
import {
  archiveAPI,
  type ArchiveBucket,
  type ArchiveConfig,
  type ArchiveStatus,
  type Bundle,
  type Entry,
  type EntryContent,
  type Page,
} from './api'
import { errorMessage, formatBytes, formatTime } from './format'

type TabKey = 'capture' | 'buckets' | 'stats' | 'bundles' | 'entries'
const tabs: { key: TabKey; label: string }[] = [
  { key: 'capture', label: '采集配置' },
  { key: 'buckets', label: '存储桶' },
  { key: 'stats', label: '上传统计' },
  { key: 'bundles', label: '压缩包' },
  { key: 'entries', label: '会话数据' },
]
const tab = ref<TabKey>('capture')
const status = ref<ArchiveStatus | null>(null)
const form = ref<ArchiveConfig | null>(null)
const buckets = ref<ArchiveBucket[]>([])
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
      return h('div', { class: 'flex items-center justify-end gap-3 text-sm text-gray-500 dark:text-dark-400' }, [
        h('span', `共 ${props.total} 条`),
        h('button', { type: 'button', class: 'btn btn-secondary btn-sm', disabled: props.page <= 1, onClick: () => emit('change', props.page - 1) }, '上一页'),
        h('span', { class: 'font-mono' }, `${props.page} / ${pages}`),
        h('button', { type: 'button', class: 'btn btn-secondary btn-sm', disabled: props.page >= pages, onClick: () => emit('change', props.page + 1) }, '下一页'),
      ])
    }
  },
})

const today = computed(() => status.value?.today || new Date().toISOString().slice(0, 10))

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

function bucketLabel(b: Bundle) {
  const match = buckets.value.find(item => (b.bucket_id ? item.id === b.bucket_id : item.bucket === b.bucket))
  return match ? match.name || match.bucket : b.bucket_id ? '已删除的存储桶' : b.bucket
}

function statusBadge(s: string) {
  return { uploaded: 'badge-success', failed: 'badge-danger', uploading: 'badge-primary', sealed: 'badge-warning' }[s] || 'badge-gray'
}

async function run(action: () => Promise<void>, fallback: string) {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await action()
  } catch (e) {
    error.value = errorMessage(e, fallback)
  } finally {
    busy.value = false
  }
}

function applyConfig(cfg: ArchiveConfig) {
  form.value = { ...cfg, rules: (cfg.rules || []).map(r => ({ ...r })) }
}

async function loadStatus() {
  try {
    status.value = await archiveAPI.status()
  } catch (e) {
    error.value = errorMessage(e, '加载运行状态失败')
  }
}

async function loadBuckets() {
  try {
    buckets.value = (await archiveAPI.buckets()).items
  } catch (e) {
    error.value = errorMessage(e, '加载存储桶失败')
  }
}

async function saveConfig() {
  await run(async () => {
    applyConfig(await archiveAPI.saveConfig(form.value!))
    notice.value = '采集配置已保存，几秒内生效。'
    setTimeout(loadStatus, 2500)
  }, '保存失败')
}

async function flush() {
  await run(async () => {
    status.value = await archiveAPI.flush()
    notice.value = '已结束当前会话并封包，压缩包正在上传。'
    if (tab.value === 'bundles') await loadBundles(1)
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

function viewModelBundles(model: string) {
  Object.assign(bundleFilter, { day: '', model, status: 'uploaded' })
  tab.value = 'bundles'
  void loadBundles(1)
}

function openBundle(b: Bundle) {
  Object.assign(entryFilter, { bundle_id: b.id, day: '', model: '', session_id: '', user_id: '' })
  tab.value = 'entries'
  void loadEntries(1)
}

function clearBundleFilter() {
  entryFilter.bundle_id = 0
  void loadEntries(1)
}

async function onBucketsChanged() {
  await loadBuckets()
  setTimeout(loadStatus, 2500)
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
  await Promise.all([loadStatus(), loadBuckets()])
  if (tab.value === 'bundles') await loadBundles()
  if (tab.value === 'entries') await loadEntries()
}

watch(tab, value => {
  if (value === 'bundles' && !bundles.value.items.length) void loadBundles(1)
  if (value === 'entries' && !entries.value.items.length) void loadEntries(1)
})

let timer: ReturnType<typeof setInterval> | undefined
onMounted(async () => {
  await run(async () => {
    const [cfg] = await Promise.all([archiveAPI.getConfig(), loadStatus(), loadBuckets()])
    applyConfig(cfg)
  }, '加载配置失败')
  timer = setInterval(loadStatus, 10000)
})
onUnmounted(() => clearInterval(timer))

function addRule() {
  form.value?.rules.push({ pattern: '', alias: '' })
}
</script>

<style scoped>
.kpi {
  @apply flex flex-col gap-1 rounded-xl border border-gray-100 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/40;
}

.kpi-action {
  @apply transition-colors hover:border-primary-200 hover:bg-primary-50/40 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:hover:border-primary-800/60 dark:hover:bg-primary-900/10;
}

.kpi-label {
  @apply inline-flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-dark-400;
}

.kpi-value {
  @apply text-2xl font-bold leading-tight text-gray-900 dark:text-white;
}

.kpi-sub {
  @apply text-xs text-gray-500 dark:text-dark-400;
}

.setting-row {
  @apply flex items-center justify-between gap-4 rounded-xl border border-gray-100 px-4 py-3 dark:border-dark-700;
}

.setting-title {
  @apply text-sm font-medium text-gray-900 dark:text-white;
}

.setting-desc {
  @apply text-xs leading-5 text-gray-500 dark:text-dark-400;
}
</style>
