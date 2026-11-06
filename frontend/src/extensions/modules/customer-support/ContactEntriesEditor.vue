<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { MAX_CONTACT_ENTRIES, parseContactEntries, serializeContactEntries, type ContactEntry } from './contacts'

const props = defineProps<{
  modelValue: string
  kind: 'group' | 'link'
  valuePlaceholder?: string
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()

// Rows are local so a freshly added, still empty row survives until it is filled.
const rows = ref<ContactEntry[]>([])
let lastEmitted: string | null = null

watch(
  () => props.modelValue,
  value => {
    if (value === lastEmitted) return
    rows.value = parseContactEntries(value)
  },
  { immediate: true }
)

function sync() {
  lastEmitted = serializeContactEntries(rows.value)
  emit('update:modelValue', lastEmitted)
}

function add() {
  if (rows.value.length >= MAX_CONTACT_ENTRIES) return
  rows.value.push({ label: '', value: '' })
}

function remove(index: number) {
  rows.value.splice(index, 1)
  sync()
}

function invalidLink(row: ContactEntry) {
  return props.kind === 'link' && row.value.trim() !== '' && !sanitizeUrl(row.value)
}

const full = computed(() => rows.value.length >= MAX_CONTACT_ENTRIES)
</script>

<template>
  <div class="space-y-2" :data-kind="kind">
    <div
      v-for="(row, index) in rows"
      :key="index"
      class="flex flex-col gap-2 sm:flex-row sm:items-start"
      data-test="contact-entry"
    >
      <input
        v-model="row.label"
        type="text"
        class="input sm:w-48"
        maxlength="32"
        :placeholder="kind === 'group' ? t('customerSupport.groupLabelPlaceholder') : t('customerSupport.linkLabelPlaceholder')"
        :aria-label="`${t('customerSupport.entryLabel')} ${index + 1}`"
        @input="sync"
      />
      <div class="min-w-0 flex-1">
        <input
          v-model="row.value"
          :type="kind === 'link' ? 'url' : 'text'"
          class="input"
          :class="{ 'input-error': invalidLink(row) }"
          maxlength="500"
          :placeholder="valuePlaceholder"
          :aria-label="`${kind === 'group' ? t('customerSupport.afterSalesGroups') : t('customerSupport.onlineService')} ${index + 1}`"
          @input="sync"
        />
        <p v-if="invalidLink(row)" class="input-error-text">{{ t('customerSupport.invalidUrl') }}</p>
      </div>
      <button
        type="button"
        class="btn btn-ghost btn-icon shrink-0 self-end text-gray-400 hover:text-red-600 sm:self-auto"
        :aria-label="`${t('customerSupport.remove')} ${index + 1}`"
        :title="t('customerSupport.remove')"
        @click="remove(index)"
      >
        <Icon name="trash" size="sm" />
      </button>
    </div>
    <div class="flex flex-wrap items-center gap-3">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="full" data-test="contact-entry-add" @click="add">
        <Icon name="plus" size="sm" />{{ kind === 'group' ? t('customerSupport.addGroup') : t('customerSupport.addLink') }}
      </button>
      <span v-if="full" class="text-xs text-gray-500 dark:text-dark-400">{{ t('customerSupport.maxEntries', { max: MAX_CONTACT_ENTRIES }) }}</span>
    </div>
  </div>
</template>
