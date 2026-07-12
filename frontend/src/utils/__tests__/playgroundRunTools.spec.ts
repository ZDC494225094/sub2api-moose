import { describe, expect, it } from 'vitest'
import {
  isRecoverablePlaygroundError,
  isRetryablePlaygroundRequestError,
  markRecoverablePlaygroundError,
  playgroundRetryDelayMs
} from '../playgroundRunTools'

describe('playground run tools', () => {
  it('retries connection failures and temporary HTTP errors', () => {
    expect(isRetryablePlaygroundRequestError({ status: 0 })).toBe(true)
    expect(isRetryablePlaygroundRequestError({ status: 502 })).toBe(true)
    expect(isRetryablePlaygroundRequestError({ code: 'ECONNABORTED' })).toBe(true)
  })

  it('does not retry permanent request errors or cancellation', () => {
    expect(isRetryablePlaygroundRequestError({ status: 400 })).toBe(false)
    expect(isRetryablePlaygroundRequestError({ status: 401 })).toBe(false)
    expect(isRetryablePlaygroundRequestError({ code: 'ERR_CANCELED' })).toBe(false)
  })

  it('uses capped exponential retry delays', () => {
    expect(playgroundRetryDelayMs(1)).toBe(900)
    expect(playgroundRetryDelayMs(2)).toBe(1800)
    expect(playgroundRetryDelayMs(6)).toBe(10000)
    expect(playgroundRetryDelayMs(20)).toBe(10000)
  })

  it('marks both Error instances and structured request failures as recoverable', () => {
    const original = new Error('poll timeout')
    expect(markRecoverablePlaygroundError(original, 'fallback')).toBe(original)
    expect(isRecoverablePlaygroundError(original)).toBe(true)

    const structured = markRecoverablePlaygroundError({ status: 0, message: 'Network error' }, 'fallback')
    expect(structured.message).toBe('Network error')
    expect(isRecoverablePlaygroundError(structured)).toBe(true)
    expect(isRecoverablePlaygroundError(new Error('permanent'))).toBe(false)
  })
})
