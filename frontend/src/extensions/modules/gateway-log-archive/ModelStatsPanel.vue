<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { archiveAPI, type ArchiveBucket, type ModelStats } from './api'
import { errorMessage, formatBytes, formatTime } from './format'

const props = defineProps<{ buckets: ArchiveBucket[] }>()
const emit = defineEmits<{ viewBundles: [model: string] }>()

const bucketId = ref(0)
const stats = ref<ModelStats>({ items: [], total_bundles: 0, total_entries: 0, total_bytes: 0 })
const loading = ref(false)
const error = ref('')
const query = ref('')

const visible = computed(() => {
  const q = query.value.trim().toLowerCase()
  return q ? stats.value.items.filter(m => m.model_key.toLowerCase().includes(q)) : stats.value.items
})
const largest = computed(() => stats.value.items[0]?.size_bytes || 0)
const scopeLabel = computed(() => {
  const b = props.buckets.find(item => item.id === bucketId.value)
  return b ? b.name || b.bucket : '全部存储桶'
})

function share(bytes: number) {
  return stats.value.total_bytes > 0 ? (bytes / stats.value.total_bytes) * 100 : 0
}

function barWidth(bytes: number) {
  return largest.value > 0 ? Math.max(2, (bytes / largest.value) * 100) : 0
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    stats.value = await archiveAPI.modelStats(bucketId.value || undefined)
  } catch (e) {
    error.value = errorMessage(e, '加载统计失败')
  } finally {
    loading.value = false
  }
}

watch(bucketId, load)
onMounted(load)
defineExpose({ reload: load })
</script>

<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">按模型统计已上传</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          累计统计，不受后台索引保留期影响；点击模型可查看该模型的压缩包。
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <select v-model.number="bucketId" class="input w-48" aria-label="存储桶范围">
          <option :value="0">全部存储桶</option>
          <option v-for="b in buckets" :key="b.id" :value="b.id">{{ b.name || b.bucket }}</option>
        </select>
        <input v-model="query" class="input w-48" placeholder="筛选模型" aria-label="筛选模型" />
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <p v-if="error" role="alert" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>

    <dl class="grid gap-3 sm:grid-cols-3">
      <div class="card p-4">
        <dt class="text-xs font-medium text-gray-500 dark:text-dark-400">已上传总量 · {{ scopeLabel }}</dt>
        <dd class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ formatBytes(stats.total_bytes) }}</dd>
      </div>
      <div class="card p-4">
        <dt class="text-xs font-medium text-gray-500 dark:text-dark-400">压缩包</dt>
        <dd class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ stats.total_bundles.toLocaleString() }}</dd>
      </div>
      <div class="card p-4">
        <dt class="text-xs font-medium text-gray-500 dark:text-dark-400">会话 · 模型数</dt>
        <dd class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
          {{ stats.total_entries.toLocaleString() }}
          <span class="text-base font-medium text-gray-500 dark:text-dark-400">· {{ stats.items.length }} 个模型</span>
        </dd>
      </div>
    </dl>

    <div class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="table min-w-[760px]">
          <thead>
            <tr>
              <th>模型 / 压缩包名</th>
              <th class="w-[34%]">已上传大小</th>
              <th class="!text-right">压缩包</th>
              <th class="!text-right">会话</th>
              <th>最近上传</th>
              <th class="!text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!visible.length">
              <td colspan="6" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                {{ loading ? '正在加载…' : query ? '没有匹配的模型' : '还没有上传完成的压缩包' }}
              </td>
            </tr>
            <tr v-for="m in visible" :key="m.model_key">
              <td class="font-medium text-gray-900 dark:text-white">{{ m.model_key }}</td>
              <td>
                <div class="flex items-center gap-3">
                  <div class="h-2 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                    <div class="h-full rounded-full bg-primary-500" :style="{ width: barWidth(m.size_bytes) + '%' }" />
                  </div>
                  <span class="w-20 shrink-0 text-right text-sm font-semibold text-gray-900 dark:text-white">{{ formatBytes(m.size_bytes) }}</span>
                  <span class="w-12 shrink-0 text-right text-xs text-gray-500 dark:text-dark-400">{{ share(m.size_bytes).toFixed(1) }}%</span>
                </div>
              </td>
              <td class="text-right">{{ m.bundle_count.toLocaleString() }}</td>
              <td class="text-right">{{ m.entry_count.toLocaleString() }}</td>
              <td class="whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">{{ formatTime(m.last_uploaded_at) }}</td>
              <td class="text-right">
                <button type="button" class="btn btn-ghost btn-sm" @click="emit('viewBundles', m.model_key)">
                  查看压缩包<Icon name="chevronRight" size="sm" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>
