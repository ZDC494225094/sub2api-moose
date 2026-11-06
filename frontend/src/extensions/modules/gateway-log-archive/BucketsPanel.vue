<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import BucketDialog from './BucketDialog.vue'
import { archiveAPI, type ArchiveBucket, type BucketList, type TestResult } from './api'
import { errorMessage, formatBytes, providerLabels } from './format'

const props = defineProps<{ today: string }>()
const emit = defineEmits<{ changed: [] }>()

const list = ref<BucketList>({ items: [], unattributed: { bundle_count: 0, size_bytes: 0 } })
const loading = ref(false)
const error = ref('')
const notice = ref('')
const dialogOpen = ref(false)
const editing = ref<ArchiveBucket | null>(null)
const deleting = ref<ArchiveBucket | null>(null)
const busyId = ref(0)
const tests = ref<Record<number, TestResult & { error?: string }>>({})

const primaryId = computed(() => list.value.items.find(b => b.enabled)?.id ?? 0)
const deleteMessage = computed(() => {
  const b = deleting.value
  if (!b) return ''
  const used = b.usage.bundle_count
    ? `该存储桶已上传 ${b.usage.bundle_count} 个压缩包（${formatBytes(b.usage.size_bytes)}）。删除配置后，后台将无法再预览或下载这些压缩包；存储桶内的对象和统计数据不受影响。`
    : '该存储桶还没有上传过压缩包。'
  return `${used}如只是暂停使用，建议改为关闭“启用”。`
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    list.value = await archiveAPI.buckets()
  } catch (e) {
    error.value = errorMessage(e, '加载存储桶失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(b: ArchiveBucket) {
  editing.value = b
  dialogOpen.value = true
}

async function onSaved(b: ArchiveBucket) {
  notice.value = editing.value ? `已保存「${b.name || b.bucket}」，几秒内生效。` : `已添加「${b.name || b.bucket}」，几秒内生效。`
  dialogOpen.value = false
  await load()
  emit('changed')
}

async function setEnabled(b: ArchiveBucket, enabled: boolean) {
  busyId.value = b.id
  error.value = ''
  try {
    await archiveAPI.updateBucket(b.id, { ...b, enabled })
    notice.value = `「${b.name || b.bucket}」已${enabled ? '启用' : '停用'}。`
    await load()
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e, '保存失败')
  } finally {
    busyId.value = 0
  }
}

async function runTest(b: ArchiveBucket) {
  busyId.value = b.id
  try {
    tests.value = { ...tests.value, [b.id]: await archiveAPI.testBucket({ ...b }) }
  } catch (e) {
    tests.value = { ...tests.value, [b.id]: { ok: false, steps: [], error: errorMessage(e, '测试失败') } }
  } finally {
    busyId.value = 0
  }
}

async function confirmDelete() {
  const b = deleting.value
  deleting.value = null
  if (!b) return
  error.value = ''
  try {
    await archiveAPI.deleteBucket(b.id)
    notice.value = `已删除「${b.name || b.bucket}」。`
    await load()
    emit('changed')
  } catch (e) {
    error.value = errorMessage(e, '删除失败')
  }
}

function testSummary(id: number) {
  const t = tests.value[id]
  if (!t) return ''
  if (t.error) return t.error
  const failed = t.steps.find(s => !s.ok && !s.name.startsWith('删除'))
  return t.ok ? '连接正常' : failed ? `${failed.name}：${failed.error || '失败'}` : '未通过'
}

onMounted(load)
defineExpose({ reload: load })
</script>

<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0 max-w-3xl">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">存储桶</h2>
        <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-dark-400">
          压缩包按「优先级」从小到大选择启用的存储桶上传；上传失败会自动切换到下一个。每个压缩包记录所在存储桶，预览和下载会从对应存储桶读取。
        </p>
      </div>
      <div class="flex shrink-0 gap-2">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />刷新
        </button>
        <button type="button" class="btn btn-primary" @click="openCreate">
          <Icon name="plus" size="sm" />新增存储桶
        </button>
      </div>
    </div>

    <p v-if="error" role="alert" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    <p v-if="notice && !error" role="status" class="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">{{ notice }}</p>

    <div class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="table min-w-[960px]">
          <thead>
            <tr>
              <th class="w-28">优先级</th>
              <th>名称 / 存储桶</th>
              <th>服务商 / 地址</th>
              <th>凭证</th>
              <th class="!text-right">已上传</th>
              <th class="w-24">启用</th>
              <th class="w-48 !text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!list.items.length">
              <td colspan="7" class="py-12 text-center">
                <div class="mx-auto flex max-w-sm flex-col items-center gap-3">
                  <span class="grid h-11 w-11 place-items-center rounded-xl bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300">
                    <Icon name="database" size="md" />
                  </span>
                  <p class="text-sm text-gray-500 dark:text-dark-400">{{ loading ? '正在加载…' : '还没有存储桶，添加后才能开启采集。' }}</p>
                  <button v-if="!loading" type="button" class="btn btn-primary btn-sm" @click="openCreate">
                    <Icon name="plus" size="sm" />新增存储桶
                  </button>
                </div>
              </td>
            </tr>
            <tr v-for="b in list.items" :key="b.id" :class="{ 'opacity-60': !b.enabled }">
              <td class="whitespace-nowrap">
                <span class="font-mono text-sm text-gray-700 dark:text-gray-200">{{ b.priority }}</span>
                <span v-if="b.id === primaryId" class="badge badge-primary ml-2 whitespace-nowrap">当前</span>
              </td>
              <td class="min-w-[220px]">
                <p class="font-medium text-gray-900 dark:text-white">{{ b.name || b.bucket }}</p>
                <p class="mt-0.5 break-all font-mono text-xs text-gray-500 dark:text-dark-400">
                  s3://{{ b.bucket }}{{ b.prefix ? '/' + b.prefix : '' }}
                </p>
              </td>
              <td class="min-w-[180px]">
                <p class="text-sm text-gray-800 dark:text-gray-200">{{ providerLabels[b.provider] || b.provider }}</p>
                <p class="mt-0.5 break-all text-xs text-gray-500 dark:text-dark-400">
                  {{ b.endpoint || b.region || '默认地址' }}<span v-if="b.force_path_style"> · Path-Style</span>
                </p>
              </td>
              <td class="whitespace-nowrap text-xs">
                <span v-if="b.secret_access_key_set" class="font-mono text-gray-700 dark:text-gray-200">{{ b.access_key_id }}</span>
                <span v-else class="text-gray-500 dark:text-dark-400">服务器环境凭证</span>
                <span v-if="b.session_token_set" class="badge badge-gray ml-1">Token</span>
              </td>
              <td class="whitespace-nowrap text-right">
                <p class="font-semibold text-gray-900 dark:text-white">{{ formatBytes(b.usage.size_bytes) }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ b.usage.bundle_count }} 个 ZIP</p>
              </td>
              <td>
                <Toggle :model-value="b.enabled" :aria-label="`启用 ${b.name || b.bucket}`" @update:model-value="setEnabled(b, $event)" />
              </td>
              <td class="text-right">
                <div class="inline-flex flex-nowrap justify-end gap-1">
                  <button type="button" class="btn btn-ghost btn-sm" :disabled="busyId === b.id" @click="runTest(b)">测试</button>
                  <button type="button" class="btn btn-ghost btn-sm" @click="openEdit(b)">编辑</button>
                  <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="deleting = b">删除</button>
                </div>
                <p
                  v-if="tests[b.id]"
                  class="mt-1 max-w-[16rem] break-words text-right text-xs"
                  :class="tests[b.id].ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'"
                  :title="testSummary(b.id)"
                >
                  {{ testSummary(b.id) }}
                </p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p
        v-if="list.unattributed.bundle_count"
        class="border-t border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400"
      >
        另有 {{ list.unattributed.bundle_count }} 个压缩包（{{ formatBytes(list.unattributed.size_bytes) }}）上传到已删除的存储桶，计入总量统计。
      </p>
    </div>

    <BucketDialog :show="dialogOpen" :bucket="editing" :today="props.today" @close="dialogOpen = false" @saved="onSaved" />
    <ConfirmDialog
      :show="!!deleting"
      :title="deleting ? `删除存储桶「${deleting.name || deleting.bucket}」？` : ''"
      :message="deleteMessage"
      confirm-text="删除"
      danger
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </section>
</template>
