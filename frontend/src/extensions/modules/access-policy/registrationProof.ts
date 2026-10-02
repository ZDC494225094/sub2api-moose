import { createRegistrationProofChallenge } from '@/api/auth'

export interface RegistrationProof {
  challenge: string
  solution: string
  expiresAt: number
}

type WorkerProgress = {
  type: 'progress'
  jobId: number
  attempts: number
}

type WorkerSolved = {
  type: 'solved'
  jobId: number
  solution: string
  attempts: number
}

type WorkerError = {
  type: 'error'
  jobId: number
  message: string
}

let nextJobId = 1

export async function createAndSolveRegistrationProof(
  email: string,
  onProgress?: (attempts: number) => void,
  signal?: AbortSignal
): Promise<RegistrationProof | null> {
  const challenge = await createRegistrationProofChallenge(email)
  if (!challenge.enabled) return null
  if (
    !challenge.challenge ||
    !challenge.difficulty ||
    !challenge.expires_at ||
    challenge.difficulty < 1
  ) {
    throw new Error('Invalid registration proof challenge')
  }

  const solution = await solveRegistrationProof(
    challenge.challenge,
    challenge.difficulty,
    onProgress,
    signal
  )
  return {
    challenge: challenge.challenge,
    solution,
    expiresAt: challenge.expires_at
  }
}

function solveRegistrationProof(
  challenge: string,
  difficulty: number,
  onProgress?: (attempts: number) => void,
  signal?: AbortSignal
): Promise<string> {
  if (signal?.aborted) return Promise.reject(new DOMException('Aborted', 'AbortError'))

  return new Promise((resolve, reject) => {
    const jobId = nextJobId++
    const worker = new Worker(new URL('./registrationProof.worker.ts', import.meta.url), {
      type: 'module'
    })

    const cleanup = (): void => {
      signal?.removeEventListener('abort', handleAbort)
      worker.terminate()
    }
    const handleAbort = (): void => {
      worker.postMessage({ type: 'cancel', jobId })
      cleanup()
      reject(new DOMException('Aborted', 'AbortError'))
    }

    worker.addEventListener('message', (event: MessageEvent<WorkerProgress | WorkerSolved | WorkerError>) => {
      const message = event.data
      if (message.jobId !== jobId) return
      if (message.type === 'progress') {
        onProgress?.(message.attempts)
        return
      }
      cleanup()
      if (message.type === 'solved') {
        onProgress?.(message.attempts)
        resolve(message.solution)
      } else {
        reject(new Error(message.message))
      }
    })
    worker.addEventListener('error', (event) => {
      cleanup()
      reject(new Error(event.message || 'Failed to solve registration proof'))
    })
    signal?.addEventListener('abort', handleAbort, { once: true })
    worker.postMessage({ type: 'solve', jobId, challenge, difficulty })
  })
}
