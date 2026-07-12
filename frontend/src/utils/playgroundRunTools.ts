export interface PlaygroundRequestErrorLike {
  status?: unknown
  code?: unknown
  message?: unknown
}

const PLAYGROUND_RECOVERABLE_ERROR_KEY = '__playgroundRecoverable'

export function isRetryablePlaygroundRequestError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const candidate = error as PlaygroundRequestErrorLike
  const status = Number(candidate.status)
  if (status === 0 || status === 408 || status === 425 || status === 429 || status >= 500) return true

  const code = String(candidate.code || '').toUpperCase()
  if (['ERR_NETWORK', 'ECONNABORTED', 'ETIMEDOUT', 'ERR_BAD_RESPONSE'].includes(code)) return true

  const message = String(candidate.message || '').toLowerCase()
  return message.includes('network error') || message.includes('timeout') || message.includes('connection reset')
}

export function playgroundRetryDelayMs(attempt: number, baseMs = 900, maxMs = 10000): number {
  const normalizedAttempt = Math.max(1, Math.floor(attempt) || 1)
  return Math.min(maxMs, baseMs * 2 ** Math.min(normalizedAttempt - 1, 6))
}

export function markRecoverablePlaygroundError(error: unknown, fallbackMessage: string): Error {
  const candidateMessage = error && typeof error === 'object'
    ? String((error as PlaygroundRequestErrorLike).message || '')
    : ''
  const recoverableError = error instanceof Error
    ? error
    : new Error(candidateMessage || fallbackMessage)
  Object.defineProperty(recoverableError, PLAYGROUND_RECOVERABLE_ERROR_KEY, {
    value: true,
    configurable: true
  })
  return recoverableError
}

export function isRecoverablePlaygroundError(error: unknown): boolean {
  return Boolean(
    error &&
    typeof error === 'object' &&
    (error as Record<string, unknown>)[PLAYGROUND_RECOVERABLE_ERROR_KEY]
  )
}
