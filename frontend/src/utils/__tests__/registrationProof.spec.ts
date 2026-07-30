import { afterEach, describe, expect, it, vi } from 'vitest'
import { createRegistrationProofChallenge } from '@/api/auth'
import { createAndSolveRegistrationProof } from '@/utils/registrationProof'

vi.mock('@/api/auth', () => ({
  createRegistrationProofChallenge: vi.fn()
}))

class WorkerStub {
  private messageListener?: (event: MessageEvent) => void

  addEventListener(type: string, listener: EventListener): void {
    if (type === 'message') {
      this.messageListener = listener as (event: MessageEvent) => void
    }
  }

  removeEventListener(): void {}
  terminate(): void {}

  postMessage(message: { type: string; jobId: number }): void {
    if (message.type !== 'solve') return
    queueMicrotask(() => {
      this.messageListener?.({
        data: { type: 'solved', jobId: message.jobId, solution: '42', attempts: 43 }
      } as MessageEvent)
    })
  }
}

describe('createAndSolveRegistrationProof', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('returns null without starting a worker when the protection is disabled', async () => {
    vi.mocked(createRegistrationProofChallenge).mockResolvedValue({ enabled: false })
    const worker = vi.fn()
    vi.stubGlobal('Worker', worker)

    await expect(createAndSolveRegistrationProof('user@example.com')).resolves.toBeNull()
    expect(worker).not.toHaveBeenCalled()
  })

  it('returns the server challenge with the locally solved nonce', async () => {
    vi.mocked(createRegistrationProofChallenge).mockResolvedValue({
      enabled: true,
      challenge: 'signed-challenge',
      difficulty: 18,
      expires_at: 1234567890
    })
    vi.stubGlobal('Worker', WorkerStub)

    await expect(createAndSolveRegistrationProof('user@example.com')).resolves.toEqual({
      challenge: 'signed-challenge',
      solution: '42',
      expiresAt: 1234567890
    })
  })
})
