<template>
  <Teleport to="body">
    <div class="premium-home" style="display: contents">
      <div class="notice-backdrop" :class="{ 'is-open': open }" @click="requestClose"></div>
      <aside class="notice-card notice-card--dialog" :class="{ 'is-open': open }" aria-label="公告弹窗">
        <div class="notice-head">
          <div class="notice-title">
            <Icon name="menu" size="sm" />
            公告
          </div>
          <button class="notice-close" type="button" aria-label="关闭公告弹窗" @click="requestClose">×</button>
        </div>
        <div ref="listRef" class="notice-list">
          <button
            v-for="notice in orderedAnnouncements"
            :key="notice.id"
            class="notice-item"
            :class="{ 'is-highlighted': isHighlighted(notice.id) }"
            :aria-current="isHighlighted(notice.id) ? 'true' : undefined"
            type="button"
            @click="openAnnouncement(notice)"
          >
            <div class="notice-date">
              {{ announcementPublishedDate(notice) }}
              <span v-if="isRecentAnnouncement(notice)" class="new-tag">NEW</span>
              <span v-if="isHighlighted(notice.id)" class="notice-unread-tag">待阅读</span>
            </div>
            <h3>{{ notice.title }}</h3>
            <p>{{ announcementExcerpt(notice.content) }}</p>
          </button>
        </div>
        <div class="notice-footer">
          <button v-if="showDismissToday" class="notice-footer-btn is-muted" type="button" @click="emit('dismissToday')">今日关闭</button>
          <button
            class="notice-footer-btn"
            :class="{ 'notice-all-btn': !showDismissToday }"
            type="button"
            @click="requestClose"
          >
            关闭
          </button>
        </div>
      </aside>

      <div class="notice-reader-backdrop" :class="{ 'is-open': !!selectedAnnouncement }" @click="closeAnnouncement"></div>
      <section class="notice-reader" :class="{ 'is-open': !!selectedAnnouncement }" aria-label="公告全文">
        <div class="notice-reader-head">
          <div>
            <span>{{ announcementPublishedDate(selectedAnnouncement) }}</span>
            <h2>{{ selectedAnnouncement?.title }}</h2>
          </div>
          <button
            type="button"
            aria-label="关闭公告全文"
            data-testid="announcement-popup-dismiss"
            @click="closeAnnouncement"
          >×</button>
        </div>
        <div class="notice-reader-body markdown-body" v-html="selectedAnnouncementHtml"></div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import type { UserAnnouncement } from '@/types'
import {
  announcementPublishedDate,
  isRecentAnnouncement,
  sortAnnouncementsByPublishedAt,
} from '@/utils/announcementOrder'
import './premium-home.css'
import '@/styles/announcement-markdown.css'

const props = withDefaults(defineProps<{
  announcements: UserAnnouncement[]
  open: boolean
  showDismissToday?: boolean
  initialSelectedAnnouncement?: UserAnnouncement | null
  closeOnReaderClose?: boolean
  /** Announcements that triggered an automatic popup; marked in the list only. */
  highlightIds?: readonly number[]
}>(), {
  showDismissToday: true,
  initialSelectedAnnouncement: null,
  closeOnReaderClose: false,
  highlightIds: () => [],
})

const emit = defineEmits<{
  close: []
  dismissToday: []
  select: [notice: UserAnnouncement]
}>()

const selectedAnnouncement = ref<UserAnnouncement | null>(null)
const listRef = ref<HTMLElement | null>(null)
const orderedAnnouncements = computed(() => sortAnnouncementsByPublishedAt(props.announcements))
const highlighted = computed(() => new Set(props.highlightIds))

function isHighlighted(id: number) {
  return highlighted.value.has(id)
}

marked.setOptions({
  breaks: true,
  gfm: true,
})

const selectedAnnouncementHtml = computed(() => {
  if (!selectedAnnouncement.value?.content) return ''
  const html = marked.parse(selectedAnnouncement.value.content) as string
  return DOMPurify.sanitize(html)
})

function stripMarkdown(content: string) {
  return content
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)]\([^)]*\)/g, '$1')
    .replace(/[#>*_\-~|]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function announcementExcerpt(content: string) {
  const plain = stripMarkdown(content)
  return plain.length > 64 ? `${plain.slice(0, 64)}...` : plain
}

function requestClose() {
  emit('close')
}

function openAnnouncement(notice?: UserAnnouncement) {
  if (!notice) return
  selectedAnnouncement.value = notice
  emit('select', notice)
}

function closeAnnouncement() {
  selectedAnnouncement.value = null
  if (props.closeOnReaderClose) requestClose()
}

function resetSelectedAnnouncement() {
  selectedAnnouncement.value = null
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (selectedAnnouncement.value) {
    closeAnnouncement()
  } else if (props.open) {
    requestClose()
  }
}

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) resetSelectedAnnouncement()
  }
)

// Bring the first highlighted notice into view when the popup opens on a long list.
watch(
  () => [props.open, props.highlightIds] as const,
  async ([isOpen]) => {
    if (!isOpen || !props.highlightIds.length) return
    await nextTick()
    listRef.value?.querySelector<HTMLElement>('.notice-item.is-highlighted')?.scrollIntoView?.({ block: 'nearest' })
  },
  { immediate: true }
)

watch(
  () => props.initialSelectedAnnouncement,
  (announcement) => {
    selectedAnnouncement.value = props.open ? announcement ?? null : null
  },
  { immediate: true }
)

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
})

defineExpose({
  closeAnnouncement,
  openAnnouncement,
})
</script>
