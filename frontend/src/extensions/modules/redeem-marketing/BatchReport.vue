<script setup lang="ts">
import { ref } from 'vue'
import { redeemMarketingAPI, type MarketingBatch } from './api'
import { formatDateTime } from '@/utils/format'

const open = ref(false)
const loading = ref(false)
const error = ref('')
const search = ref('')
const appliedSearch = ref('')
const page = ref(1)
const items = ref<MarketingBatch[]>([])
let revision = 0

async function load(next = 1, query = appliedSearch.value) {
  const current = ++revision
  loading.value = true
  error.value = ''
  try {
    const result = await redeemMarketingAPI.batches(query.trim(), next)
    if (current !== revision) return
    items.value = result
    page.value = next
    appliedSearch.value = query.trim()
  } catch (err: unknown) {
    if (current !== revision) return
    items.value = []
    error.value = (err as { response?: { data?: { detail?: string } } }).response?.data?.detail || '批次统计加载失败，请重试。'
  } finally {
    if (current === revision) loading.value = false
  }
}

function show() {
  open.value = true
  void load()
}

const percent = (part: number, total: number) => total ? `${(part / total * 100).toFixed(1)}%` : '0.0%'
const money = (amounts: Record<string, number>) => Object.entries(amounts).map(([currency, amount]) => `${currency} ${amount.toFixed(2)}`).join(' / ') || '—'
</script>

<template>
  <button class="btn btn-secondary" type="button" @click="show">营销码批次转化</button>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-3" @click.self="open = false" @keydown.esc="open = false">
      <section role="dialog" aria-modal="true" aria-label="营销码批次转化" class="flex max-h-[90vh] w-full max-w-6xl flex-col gap-4 overflow-y-auto rounded-2xl bg-white p-5 shadow-xl dark:bg-dark-800">
        <header class="flex items-center justify-between gap-3">
          <h2 class="text-lg font-semibold">营销兑换码 · 批次转化与用户回购</h2>
          <button type="button" class="btn btn-secondary" @click="open = false">关闭</button>
        </header>
        <p class="text-sm text-gray-500">按批次独立统计：付费转化率 = 兑换后付费人数 / 已兑换人数。仅统计兑换后创建且支付成功的订单（PAID / RECHARGING / COMPLETED），排除退款订单，金额为实际支付金额。包含首次付费；同一用户参加不同批次会分别计入，不可跨批次直接求和。</p>
        <form class="flex flex-wrap gap-2" @submit.prevent="load(1, search)">
          <input v-model="search" class="input min-w-0 flex-1" placeholder="输入完整批次编号，留空查看全部" aria-label="批次编号" maxlength="64" />
          <button class="btn btn-primary" :disabled="loading">查询 / 刷新</button>
        </form>
        <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
        <p v-if="loading" class="text-sm text-gray-500">正在加载统计…</p>
        <div v-else class="overflow-x-auto">
          <table class="w-full whitespace-nowrap text-left text-sm">
            <thead class="bg-gray-50 dark:bg-dark-700"><tr>
              <th class="p-3">批次 / 生成时间</th><th class="p-3">适用用户</th><th class="p-3">发码 / 兑换</th>
              <th class="p-3">兑换率</th><th class="p-3">发放余额</th><th class="p-3">付费人数</th>
              <th class="p-3">付费转化率</th><th class="p-3">支付订单</th><th class="p-3">实付金额</th>
            </tr></thead>
            <tbody><tr v-for="batch in items" :key="batch.batch_id" class="border-b border-gray-100 dark:border-dark-700">
              <td class="p-3"><code class="text-xs">{{ batch.batch_id }}</code><div class="mt-1 text-xs text-gray-500">{{ formatDateTime(batch.created_at) }}</div></td>
              <td class="p-3" :title="batch.registered_after ? `注册时间不早于 ${formatDateTime(batch.registered_after)}` : ''">{{ batch.registered_after ? '仅新注册用户' : '不限注册时间' }}</td>
              <td class="p-3">{{ batch.code_count }} / {{ batch.redeemed_users }}</td>
              <td class="p-3">{{ percent(batch.redeemed_users, batch.code_count) }}</td>
              <td class="p-3">${{ batch.redeemed_value.toFixed(2) }}</td>
              <td class="p-3">{{ batch.paying_users }}</td>
              <td class="p-3 font-semibold text-primary-600">{{ percent(batch.paying_users, batch.redeemed_users) }}</td>
              <td class="p-3">{{ batch.order_count }}</td><td class="p-3">{{ money(batch.pay_amounts) }}</td>
            </tr></tbody>
          </table>
          <p v-if="!items.length && !error" class="p-6 text-center text-gray-500">暂无匹配的营销兑换码批次。</p>
        </div>
        <footer class="flex items-center justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="loading || page <= 1" @click="load(page - 1)">上一页</button>
          <span class="text-sm">第 {{ page }} 页</span>
          <button type="button" class="btn btn-secondary" :disabled="loading || items.length < 20" @click="load(page + 1)">下一页</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>
