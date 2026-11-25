<script setup lang="ts">
import { computed } from 'vue'
import type { ArchiveStatus, RedactionCounts } from './api'
import { formatBytes, formatTime } from './format'

const props = defineProps<{ status: ArchiveStatus }>()

const kindLabels: Record<string, string> = {
  'model:private_person': '人名',
  'model:private_address': '地点',
  'model:phone': '电话',
  'model:email': '邮箱',
  'model:account': '证件号 / 账号',
  'model:payment_card': '银行卡',
  'model:common_secret': '密钥 / 口令',
  'model:ip_address': 'IP 地址',
  'model:url': '网址',
  'model:private_date': '日期',
  'rule:email': '邮箱',
  'rule:phone': '手机号',
  'rule:id_card': '身份证号',
  'rule:account': '证件 / 卡号',
  'rule:person': '标注的姓名',
  'rule:address': '标注的地址',
  'rule:field': '个人信息字段',
}

interface Chip { key: string; label: string; count: number; model: boolean }
function chips(counts?: RedactionCounts): Chip[] {
  return Object.entries(counts?.replacements || {})
    .map(([key, count]) => ({ key, count, model: key.startsWith('model:'), label: kindLabels[key] || key.replace(/^(model|rule):/, '') }))
    .sort((a, b) => b.count - a.count)
}
const replacementTotal = (counts?: RedactionCounts) => Object.values(counts?.replacements || {}).reduce((a, b) => a + b, 0)

function formatDuration(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (d) return `${d} 天 ${h} 小时`
  if (h) return `${h} 小时 ${m} 分`
  if (m) return `${m} 分 ${s % 60} 秒`
  return `${s} 秒`
}
const rate = (bytes: number, ms: number) => (ms > 0 ? (bytes * 1000) / ms : 0)

// Where an upload's time went. Network: a storage request in flight. Model:
// waiting for the detection service. Local: reading files and the rules.
interface Breakdown { total: number; network: number; model: number; local: number }
function breakdown(total: number, network: number, model: number): Breakdown {
  total = Math.max(total, 1)
  network = Math.min(network, total)
  model = Math.min(model, total - network)
  return { total, network, model, local: total - network - model }
}
const pct = (part: number, b: Breakdown) => Math.round((part / b.total) * 100)

function bottleneck(b: Breakdown, hasDetector: boolean): { tone: string; text: string } | null {
  if (b.total < 5000) return null
  const share = { network: b.network / b.total, model: b.model / b.total, local: b.local / b.total }
  if (share.model >= share.network && share.model >= share.local && share.model > 0.3) {
    return {
      tone: 'text-amber-700 dark:text-amber-400',
      text: `${pct(b.model, b)}% 的上传时间在等待模型脱敏服务，它是当前瓶颈。可以给脱敏服务更多 CPU 并调大 PRIVACY_DETECTOR_WORKERS，把脱敏范围改为「仅自然语言」减少送检量，或暂时关闭模型脱敏服务。`,
    }
  }
  if (share.network >= share.local) {
    return {
      tone: 'text-gray-600 dark:text-dark-300',
      text: `${pct(b.network, b)}% 的上传时间在等待存储桶${hasDetector ? `，模型脱敏只占 ${pct(b.model, b)}%` : ''}。瓶颈在到存储桶的网络或存储桶本身。`,
    }
  }
  return {
    tone: 'text-gray-600 dark:text-dark-300',
    text: `${pct(b.local, b)}% 的上传时间用于读取本地归档和内置规则处理，通常受磁盘读取速度或 CPU 限制。`,
  }
}

const stats = computed(() => props.status.upload_stats)
const hasDetector = computed(() => !!props.status.detector)

// Recent successful uploads give the current speed and time breakdown.
const recentOK = computed(() => (stats.value?.recent || []).filter(r => r.ok))
const recentSum = computed(() => recentOK.value.reduce(
  (s, r) => ({ bytes: s.bytes + r.bytes, ms: s.ms + r.duration_ms, network: s.network + r.network_ms, model: s.model + r.redaction.detector_ms }),
  { bytes: 0, ms: 0, network: 0, model: 0 },
))
const recentRate = computed(() => rate(recentSum.value.bytes, recentSum.value.ms))
const recentBreakdown = computed(() => breakdown(recentSum.value.ms, recentSum.value.network, recentSum.value.model))

const current = computed(() => {
  const up = props.status.uploading
  if (!up || up.network_ms == null || !up.started_at) return null
  const elapsed = Date.now() - Date.parse(up.started_at)
  return { up, b: breakdown(elapsed, up.network_ms, up.redaction?.detector_ms || 0) }
})

const backlog = computed(() => {
  const rows = (props.status.bundles || []).filter(b => ['sealed', 'uploading', 'failed'].includes(b.status))
  const count = rows.reduce((a, b) => a + b.count, 0)
  const bytes = rows.reduce((a, b) => a + b.bytes, 0)
  return { count, bytes, eta: recentRate.value > 0 && bytes > 0 ? (bytes / recentRate.value) * 1000 : null }
})
const diskTight = computed(() => props.status.disk_free_bytes >= 0 && backlog.value.bytes > 0 && props.status.disk_free_bytes < backlog.value.bytes)
</script>

<template>
  <section v-if="stats" class="mt-4 space-y-4 rounded-xl border border-gray-100 p-4 dark:border-dark-700" data-testid="upload-stats">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">上传与脱敏统计</h3>
      <span class="text-xs text-gray-500 dark:text-dark-400">当前节点，统计开始于 {{ formatTime(stats.totals.since) }}；节点重启后重新统计</span>
    </div>

    <div class="grid grid-cols-2 gap-x-6 gap-y-3 text-sm lg:grid-cols-4">
      <div>
        <p class="text-xs text-gray-500 dark:text-dark-400">待上传积压</p>
        <p class="font-mono text-gray-900 dark:text-white" data-testid="backlog">{{ backlog.count }} 个 · {{ formatBytes(backlog.bytes) }}</p>
        <p v-if="diskTight" class="text-xs text-red-600 dark:text-red-400">超过剩余磁盘空间 {{ formatBytes(status.disk_free_bytes) }}</p>
      </div>
      <div>
        <p class="text-xs text-gray-500 dark:text-dark-400">最近上传速度</p>
        <p class="font-mono text-gray-900 dark:text-white">{{ recentRate > 0 ? `${formatBytes(recentRate)}/s` : '—' }}</p>
        <p class="text-xs text-gray-500 dark:text-dark-400">按最近 {{ recentOK.length }} 个成功上传</p>
      </div>
      <div>
        <p class="text-xs text-gray-500 dark:text-dark-400">清空积压预计需要</p>
        <p class="font-mono text-gray-900 dark:text-white" data-testid="backlog-eta">{{ backlog.eta != null ? formatDuration(backlog.eta) : '—' }}</p>
        <p class="text-xs text-gray-500 dark:text-dark-400">不含期间新增的归档</p>
      </div>
      <div>
        <p class="text-xs text-gray-500 dark:text-dark-400">已完成上传</p>
        <p class="font-mono text-gray-900 dark:text-white">{{ stats.totals.uploads }} 个 · {{ formatBytes(stats.totals.bytes) }}</p>
        <p class="text-xs" :class="stats.totals.failed ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-dark-400'">失败 {{ stats.totals.failed }} 次</p>
      </div>
    </div>

    <div v-if="recentSum.ms > 0">
      <p class="text-xs text-gray-500 dark:text-dark-400">最近上传的耗时构成</p>
      <div class="mt-1 flex h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700" data-testid="time-breakdown">
        <div class="bg-sky-500" :style="{ width: pct(recentBreakdown.network, recentBreakdown) + '%' }" />
        <div class="bg-amber-500" :style="{ width: pct(recentBreakdown.model, recentBreakdown) + '%' }" />
        <div class="bg-gray-400" :style="{ width: pct(recentBreakdown.local, recentBreakdown) + '%' }" />
      </div>
      <p class="mt-1 flex flex-wrap gap-x-4 text-xs text-gray-600 dark:text-dark-300">
        <span><span class="mr-1 inline-block h-2 w-2 rounded-full bg-sky-500" />等待存储桶 {{ pct(recentBreakdown.network, recentBreakdown) }}%</span>
        <span><span class="mr-1 inline-block h-2 w-2 rounded-full bg-amber-500" />模型脱敏服务 {{ pct(recentBreakdown.model, recentBreakdown) }}%</span>
        <span><span class="mr-1 inline-block h-2 w-2 rounded-full bg-gray-400" />读取与规则处理 {{ pct(recentBreakdown.local, recentBreakdown) }}%</span>
      </p>
      <p v-if="bottleneck(recentBreakdown, hasDetector)" class="mt-1 text-xs" :class="bottleneck(recentBreakdown, hasDetector)!.tone" data-testid="bottleneck">
        {{ bottleneck(recentBreakdown, hasDetector)!.text }}
      </p>
    </div>

    <div v-if="current" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800" data-testid="current-redaction">
      <p class="text-xs text-gray-500 dark:text-dark-400">正在上传 {{ current.up.file_name }}<template v-if="!current.up.redacted">（此存储桶上传原文，不脱敏）</template></p>
      <div class="mt-2 grid grid-cols-2 gap-x-6 gap-y-2 text-sm lg:grid-cols-4">
        <div><p class="text-xs text-gray-500 dark:text-dark-400">已读取会话</p><p class="font-mono">{{ current.up.redaction?.records ?? 0 }}</p></div>
        <div><p class="text-xs text-gray-500 dark:text-dark-400">已修改会话</p><p class="font-mono">{{ current.up.redaction?.changed ?? 0 }}</p></div>
        <div><p class="text-xs text-gray-500 dark:text-dark-400">等待模型服务</p><p class="font-mono">{{ formatDuration(current.up.redaction?.detector_ms ?? 0) }}（{{ pct(current.b.model, current.b) }}%）</p></div>
        <div><p class="text-xs text-gray-500 dark:text-dark-400">等待存储桶</p><p class="font-mono">{{ formatDuration(current.up.network_ms ?? 0) }}（{{ pct(current.b.network, current.b) }}%）</p></div>
      </div>
      <ul v-if="chips(current.up.redaction).length" class="mt-2 flex flex-wrap gap-1.5">
        <li v-for="c in chips(current.up.redaction)" :key="c.key" class="rounded-full px-2 py-0.5 text-xs" :class="c.model ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'">
          {{ c.label }} × {{ c.count.toLocaleString() }}
        </li>
      </ul>
    </div>

    <div>
      <p class="text-xs text-gray-500 dark:text-dark-400">
        累计替换 {{ replacementTotal(stats.totals.redaction).toLocaleString() }} 处 · 读取 {{ stats.totals.redaction.records.toLocaleString() }} 个会话，修改 {{ stats.totals.redaction.changed.toLocaleString() }} 个
        <template v-if="stats.totals.redaction.tool_skipped"> · 跳过工具内容 {{ stats.totals.redaction.tool_skipped.toLocaleString() }} 段</template>
        <template v-if="stats.totals.redaction.detector_ms"> · 等待模型服务共 {{ formatDuration(stats.totals.redaction.detector_ms) }}</template>
      </p>
      <ul v-if="chips(stats.totals.redaction).length" class="mt-1 flex flex-wrap gap-1.5" data-testid="replacement-kinds">
        <li v-for="c in chips(stats.totals.redaction)" :key="c.key" class="rounded-full px-2 py-0.5 text-xs" :class="c.model ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'">
          {{ c.label }} × {{ c.count.toLocaleString() }}
        </li>
      </ul>
      <p v-if="chips(stats.totals.redaction).length" class="mt-1 text-xs text-gray-400">蓝色为模型识别，灰色为内置规则</p>
    </div>

    <div v-if="stats.recent.length" class="overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead class="text-gray-500 dark:text-dark-400">
          <tr>
            <th class="py-1 pr-3 font-medium">开始时间</th>
            <th class="py-1 pr-3 font-medium">文件</th>
            <th class="py-1 pr-3 text-right font-medium">大小</th>
            <th class="py-1 pr-3 text-right font-medium">用时</th>
            <th class="py-1 pr-3 text-right font-medium">速度</th>
            <th class="py-1 pr-3 text-right font-medium">模型服务</th>
            <th class="py-1 pr-3 text-right font-medium">存储桶</th>
            <th class="py-1 text-right font-medium">替换</th>
          </tr>
        </thead>
        <tbody class="font-mono text-gray-700 dark:text-gray-300">
          <tr v-for="(r, i) in stats.recent" :key="i" class="border-t border-gray-100 dark:border-dark-700">
            <td class="py-1 pr-3 whitespace-nowrap">{{ formatTime(r.started_at) }}</td>
            <td class="py-1 pr-3">
              {{ r.file_name }}
              <span v-if="!r.ok" class="ml-1 text-red-600 dark:text-red-400">失败</span>
              <span v-else-if="!r.redacted" class="ml-1 text-gray-400">原文</span>
            </td>
            <td class="py-1 pr-3 text-right">{{ formatBytes(r.bytes) }}</td>
            <td class="py-1 pr-3 text-right">{{ formatDuration(r.duration_ms) }}</td>
            <td class="py-1 pr-3 text-right">{{ r.ok ? `${formatBytes(rate(r.bytes, r.duration_ms))}/s` : '—' }}</td>
            <td class="py-1 pr-3 text-right">{{ pct(Math.min(r.redaction.detector_ms, r.duration_ms), breakdown(r.duration_ms, 0, 0)) }}%</td>
            <td class="py-1 pr-3 text-right">{{ pct(Math.min(r.network_ms, r.duration_ms), breakdown(r.duration_ms, 0, 0)) }}%</td>
            <td class="py-1 text-right">{{ replacementTotal(r.redaction).toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
