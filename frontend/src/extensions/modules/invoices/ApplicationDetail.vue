<script setup lang="ts">
import type { InvoiceApplication } from './api'
import { formatMoney, formatTime, invoiceTypeLabels, orderTypeLabels, statusBadge, statusLabels, titleTypeLabels } from './format'

// Read-only body shared by the user detail dialog and the review dialog.
defineProps<{ app: InvoiceApplication; admin?: boolean }>()
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center gap-2">
      <span :class="statusBadge[app.status]">{{ statusLabels[app.status] }}</span>
      <span class="badge badge-gray">{{ invoiceTypeLabels[app.invoice_type] }}</span>
      <span v-if="admin && app.refund_alert" class="badge badge-danger">含已退款订单</span>
      <span class="ml-auto text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatMoney(app.amount) }}</span>
    </div>

    <p v-if="app.status === 'rejected' || app.status === 'voided'" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
      {{ app.status === 'rejected' ? '驳回原因' : '作废原因' }}：{{ app.reject_reason || '—' }}
    </p>
    <p v-if="admin && app.refund_alert" class="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-300">
      申请中有订单在提交后发生了退款。若发票已开具，请按规定红冲后将本申请作废，订单会重新变为可开票。
    </p>

    <dl class="grid gap-x-6 gap-y-3 rounded-2xl bg-gray-50 p-4 text-sm sm:grid-cols-2 dark:bg-dark-800/60">
      <div class="sm:col-span-2"><dt class="text-xs text-gray-500 dark:text-dark-400">抬头（{{ titleTypeLabels[app.title_type] }}）</dt><dd class="mt-0.5 break-words font-medium text-gray-900 dark:text-white">{{ app.title_name }}</dd></div>
      <div v-if="app.tax_no"><dt class="text-xs text-gray-500 dark:text-dark-400">纳税人识别号</dt><dd class="mt-0.5 font-mono text-gray-800 dark:text-gray-200">{{ app.tax_no }}</dd></div>
      <div v-if="app.email"><dt class="text-xs text-gray-500 dark:text-dark-400">接收邮箱</dt><dd class="mt-0.5 break-all text-gray-800 dark:text-gray-200">{{ app.email }}</dd></div>
      <template v-if="app.invoice_type === 'special' || app.address">
        <div class="sm:col-span-2"><dt class="text-xs text-gray-500 dark:text-dark-400">地址、电话</dt><dd class="mt-0.5 text-gray-800 dark:text-gray-200">{{ app.address || '—' }} {{ app.phone }}</dd></div>
        <div class="sm:col-span-2"><dt class="text-xs text-gray-500 dark:text-dark-400">开户行及账号</dt><dd class="mt-0.5 text-gray-800 dark:text-gray-200">{{ app.bank_name || '—' }} <span class="font-mono">{{ app.bank_account }}</span></dd></div>
      </template>
      <div v-if="app.item_name"><dt class="text-xs text-gray-500 dark:text-dark-400">开票内容</dt><dd class="mt-0.5 text-gray-800 dark:text-gray-200">{{ app.item_name }}</dd></div>
      <div v-if="app.invoice_no"><dt class="text-xs text-gray-500 dark:text-dark-400">发票号码</dt><dd class="mt-0.5 font-mono text-gray-800 dark:text-gray-200">{{ app.invoice_no }}</dd></div>
      <div><dt class="text-xs text-gray-500 dark:text-dark-400">申请时间</dt><dd class="mt-0.5 text-gray-800 dark:text-gray-200">{{ formatTime(app.created_at) }}</dd></div>
      <div v-if="app.issued_at"><dt class="text-xs text-gray-500 dark:text-dark-400">开具时间</dt><dd class="mt-0.5 text-gray-800 dark:text-gray-200">{{ formatTime(app.issued_at) }}</dd></div>
      <div v-if="app.remark" class="sm:col-span-2"><dt class="text-xs text-gray-500 dark:text-dark-400">用户备注</dt><dd class="mt-0.5 break-words text-gray-800 dark:text-gray-200">{{ app.remark }}</dd></div>
      <div v-if="admin && app.admin_note" class="sm:col-span-2"><dt class="text-xs text-gray-500 dark:text-dark-400">内部备注</dt><dd class="mt-0.5 break-words text-gray-800 dark:text-gray-200">{{ app.admin_note }}</dd></div>
    </dl>

    <div v-if="app.orders?.length">
      <h4 class="mb-2 text-sm font-medium text-gray-900 dark:text-white">包含订单（{{ app.order_count }} 笔）</h4>
      <div class="table-container max-h-64 overflow-y-auto">
        <table class="table">
          <thead><tr><th>订单号</th><th>类型</th><th class="hidden sm:table-cell">支付时间</th><th class="!text-right">金额</th></tr></thead>
          <tbody>
            <tr v-for="o in app.orders" :key="o.order_id">
              <td class="font-mono text-xs">
                {{ o.out_trade_no || `#${o.order_id}` }}
                <span v-if="o.refunded" class="badge badge-danger ml-1">已退款</span>
              </td>
              <td>{{ orderTypeLabels[o.order_type] || o.order_type }}</td>
              <td class="hidden whitespace-nowrap sm:table-cell">{{ formatTime(o.paid_at) }}</td>
              <td class="text-right tabular-nums">{{ formatMoney(o.amount) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
