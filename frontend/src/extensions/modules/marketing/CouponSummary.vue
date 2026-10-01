<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OrderType } from '@/types/payment'
import { useCouponCheckout } from './checkout'
const props = defineProps<{ orderType: OrderType }>()
const { t } = useI18n()
const controller = useCouponCheckout()
const selected = computed(() => controller.couponFor(props.orderType))
const discount = computed(() => controller.discountFor(props.orderType))
</script>

<template>
  <div v-if="selected" class="flex justify-between">
    <span class="text-gray-500 dark:text-gray-400">{{ t('payment.discountCoupon') }}</span>
    <span class="text-emerald-600 dark:text-emerald-400">-{{ controller.context.formatPaymentAmount(discount) }}</span>
  </div>
</template>
