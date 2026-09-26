<template>
  <section class="mb-5 space-y-4" aria-label="付款与到账构成">
    <div class="rounded-xl border border-indigo-100 bg-indigo-50/60 p-4 dark:border-indigo-900 dark:bg-indigo-950/30">
      <p class="text-xs font-medium text-indigo-600 dark:text-indigo-300">实际付款 · 按原币种分别统计</p>
      <div v-for="item in totals.currencies" :key="item.currency" class="mt-3 grid gap-4 sm:grid-cols-3">
        <div><span class="text-xs text-gray-500">充值付款 · {{ item.currency }}</span><strong class="mt-1 block text-xl tabular-nums">{{ cashMoney(item.recharge_paid, item.currency) }}</strong></div>
        <div><span class="text-xs text-gray-500">订阅付款 · {{ item.currency }}</span><strong class="mt-1 block text-xl tabular-nums">{{ cashMoney(item.subscription_paid, item.currency) }}</strong></div>
        <div><span class="text-xs text-gray-500">实际付款合计 · {{ item.currency }}</span><strong class="mt-1 block text-xl text-indigo-600 dark:text-indigo-300 tabular-nums">{{ cashMoney(item.recharge_paid + item.subscription_paid, item.currency) }}</strong></div>
      </div>
      <p v-if="!totals.currencies.length" class="mt-3 text-sm text-gray-500">该范围暂无成功付款</p>
    </div>
    <div class="grid gap-3 sm:grid-cols-2">
      <div class="rounded-xl border border-emerald-100 bg-emerald-50/50 p-4 dark:border-emerald-900 dark:bg-emerald-950/20"><p class="text-xs text-gray-500">实际到账额度 · USD</p><strong class="mt-1 block text-xl tabular-nums">{{ credits(totals.credited) }}</strong><p class="mt-2 text-xs text-gray-500">仅已完成的余额充值，包含充值赠送；不含订阅价格。</p></div>
      <div class="rounded-xl border border-amber-100 bg-amber-50/50 p-4 dark:border-amber-900 dark:bg-amber-950/20"><p class="text-xs text-gray-500">已支付待到账额度 · USD</p><strong class="mt-1 block text-xl tabular-nums">{{ credits(totals.pending) }}</strong><p class="mt-2 text-xs text-gray-500">余额订单为已支付或充值中，不计入已到账。</p></div>
    </div>
    <p class="text-xs leading-5 text-gray-500">按订单创建日期归属，取当前成功状态的实际支付金额（含支付手续费、已扣优惠）。不同币种不合计，现金付款与 USD 计费额度不相加。以下汇总覆盖整个选定范围，不是当前分页小计。</p>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { PaymentDay } from '@/api/admin/operationsFinance'
import { cashMoney, paymentTotals } from '../operationsFinanceMetrics'
const props = defineProps<{ payments: PaymentDay[]; start?: string; end?: string }>()
const totals = computed(() => paymentTotals(props.payments, props.start, props.end))
const credits = (n: number) => '$' + n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 4 })
</script>
