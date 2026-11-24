import { extractApiErrorMessage } from '@/utils/apiError'
import type { Health, ResultStatus, TopicStatus } from './api'

export function errorMessage(e: unknown, fallback: string): string {
  return extractApiErrorMessage(e, fallback)
}

export const healthLabels: Record<Health, string> = {
  normal: '正常',
  unstable: '偶有失败',
  degraded: '疑似降智',
  no_data: '暂无数据',
}

export const healthClasses: Record<Health, string> = {
  normal: 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-900/20 dark:text-emerald-300 dark:ring-emerald-800/50',
  unstable: 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-900/20 dark:text-amber-300 dark:ring-amber-800/50',
  degraded: 'bg-rose-50 text-rose-700 ring-rose-200 dark:bg-rose-900/20 dark:text-rose-300 dark:ring-rose-800/50',
  no_data: 'bg-gray-50 text-gray-500 ring-gray-200 dark:bg-dark-800 dark:text-dark-400 dark:ring-dark-700',
}

export const statusLabels: Record<ResultStatus, string> = {
  pending: '等待中',
  running: '进行中',
  pass: '通过',
  retry_pass: '重试后正确',
  fail: '失败',
  error: '请求出错',
  interrupted: '中断',
  skipped: '未执行',
}

/** Timeline cell colors. */
export const cellClasses: Record<ResultStatus, string> = {
  pending: 'bg-sky-200 dark:bg-sky-800 animate-pulse',
  running: 'bg-sky-300 dark:bg-sky-700 animate-pulse',
  pass: 'bg-emerald-500',
  retry_pass: 'bg-lime-400 dark:bg-lime-500',
  fail: 'bg-rose-500',
  error: 'bg-orange-400',
  interrupted: 'bg-gray-300 dark:bg-dark-600',
  skipped: 'bg-gray-200 dark:bg-dark-700',
}

export const topicLabels: Record<TopicStatus, string> = {
  pending: '待审核',
  reviewing: '审核中',
  approved: '已通过，排队中',
  rejected: '未通过',
  failed: '审核失败',
  used: '已使用',
}

export const topicClasses: Record<TopicStatus, string> = {
  pending: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300',
  reviewing: 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300',
  approved: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  rejected: 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300',
  failed: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  used: 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300',
}

export function formatRate(rate: number, total: number): string {
  if (!total) return '—'
  return `${rate >= 99.95 || rate === 0 ? rate.toFixed(0) : rate.toFixed(1)}%`
}

export function formatMs(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms < 1000) return `${ms} ms`
  const s = ms / 1000
  return s < 60 ? `${s.toFixed(1)} s` : `${Math.floor(s / 60)} 分 ${Math.round(s % 60)} 秒`
}

export function formatElapsed(fromISO: string | null, nowMs: number): string {
  if (!fromISO) return '—'
  const sec = Math.max(0, Math.floor((nowMs - new Date(fromISO).getTime()) / 1000))
  const m = Math.floor(sec / 60)
  return m > 0 ? `${m}:${String(sec % 60).padStart(2, '0')}` : `${sec} 秒`
}

export function formatTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`
  if (n >= 10_000) return `${(n / 1000).toFixed(1)}K`
  return String(Math.round(n))
}

export function formatUSD(n: number | null | undefined): string {
  if (n == null) return '—'
  if (n === 0) return '$0'
  return n < 0.01 ? `$${n.toFixed(4)}` : `$${n.toFixed(2)}`
}

export function formatMultiplier(n: number): string {
  return `${Number(n.toFixed(3))}x`
}
