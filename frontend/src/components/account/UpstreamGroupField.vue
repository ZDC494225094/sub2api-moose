<template>
  <div>
    <label :for="inputId" class="input-label">{{ t('admin.accounts.upstreamGroup') }}</label>
    <input
      :id="inputId"
      v-model="value"
      :list="listId"
      type="text"
      maxlength="100"
      class="input"
      :placeholder="t('admin.accounts.upstreamGroupPlaceholder')"
      :aria-describedby="hintId"
      autocomplete="off"
      data-testid="upstream-group-input"
    />
    <datalist :id="listId">
      <option v-for="group in groups" :key="group.key" :value="group.name">
        {{ t('admin.accounts.upstreamGroupOption', { count: group.account_count }) }}
      </option>
    </datalist>
    <p :id="hintId" class="input-hint">
      {{ loading ? t('admin.accounts.upstreamGroupsLoading') : t('admin.accounts.upstreamGroupHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'

import type { AccountUpstreamGroup } from '@/types'

const props = withDefaults(defineProps<{
  modelValue?: string
  groups?: AccountUpstreamGroup[]
  loading?: boolean
}>(), {
  modelValue: '',
  groups: () => [],
  loading: false
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const { t } = useI18n()
const uid = useId()
const inputId = `account-upstream-group-${uid}`
const listId = `${inputId}-options`
const hintId = `${inputId}-hint`

const value = computed({
  get: () => props.modelValue,
  set: (next: string) => emit('update:modelValue', next)
})
</script>
