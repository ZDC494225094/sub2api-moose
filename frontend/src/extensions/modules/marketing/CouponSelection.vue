<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { OrderType } from '@/types/payment'
import { useCouponCheckout } from './checkout'
const props = defineProps<{ orderType: OrderType }>()
const { t } = useI18n()
const { admission, coupons, selectedId, context } = useCouponCheckout()
const visible = computed(() => admission.value && coupons.value.length > 0 && !context.excludesAdjustments(props.orderType))
const options = computed(() => {
  const result: Array<{ value: number | null; label: string; disabled?: boolean }> = [{ value: null, label: t('userLottery.noCoupon') }]
  const currentAmount = context.orderAmount(props.orderType)
  coupons.value.filter(coupon => coupon.status === 'unused' && (coupon.scope === 'universal' || coupon.scope === props.orderType)).forEach(coupon => {
    const thresholdMet = coupon.threshold_amount <= 0 || currentAmount >= coupon.threshold_amount
    const thresholdLabel = coupon.threshold_amount > 0
      ? `${t('userLottery.thresholdPrefix')}¥${coupon.threshold_amount.toFixed(2)}` : t('userLottery.couponDirectDiscount')
    const pendingLabel = !thresholdMet && coupon.threshold_amount > 0
      ? ` · ${t('userLottery.couponThresholdPending', { amount: coupon.threshold_amount.toFixed(2) })}` : ''
    result.push({ value: coupon.id, label: `${thresholdLabel} -¥${coupon.discount_amount.toFixed(2)} (${coupon.coupon_code})${pendingLabel}`, disabled: !thresholdMet })
  })
  return result
})
</script>

<template>
  <div v-if="visible" class="card p-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userLottery.availableCoupons') }}</p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('userLottery.couponHint') }}</p>
      </div>
      <Select v-model="selectedId" :options="options" class="w-72" />
    </div>
  </div>
</template>
