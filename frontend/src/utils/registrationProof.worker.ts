type SolveRequest = {
  type: 'solve'
  jobId: number
  challenge: string
  difficulty: number
}

type CancelRequest = {
  type: 'cancel'
  jobId: number
}

const workerScope = self as unknown as Worker
const encoder = new TextEncoder()
const batchSize = 256
let activeJobId = 0

function hasLeadingZeroBits(hash: Uint8Array, difficulty: number): boolean {
  const fullBytes = Math.floor(difficulty / 8)
  const remainingBits = difficulty % 8
  for (let index = 0; index < fullBytes; index += 1) {
    if (hash[index] !== 0) return false
  }
  return remainingBits === 0 || (hash[fullBytes] ?? 0) >> (8 - remainingBits) === 0
}

async function solve(request: SolveRequest): Promise<void> {
  activeJobId = request.jobId
  let nonce = 0
  let lastProgressAt = 0

  while (activeJobId === request.jobId && nonce < Number.MAX_SAFE_INTEGER - batchSize) {
    const candidates = Array.from({ length: batchSize }, (_, offset) => nonce + offset)
    const hashes = await Promise.all(
      candidates.map((candidate) =>
        crypto.subtle.digest('SHA-256', encoder.encode(`${request.challenge}:${candidate}`))
      )
    )

    if (activeJobId !== request.jobId) return
    for (let index = 0; index < hashes.length; index += 1) {
      if (hasLeadingZeroBits(new Uint8Array(hashes[index]), request.difficulty)) {
        workerScope.postMessage({
          type: 'solved',
          jobId: request.jobId,
          solution: String(candidates[index]),
          attempts: nonce + index + 1
        })
        return
      }
    }

    nonce += batchSize
    if (nonce - lastProgressAt >= 4096) {
      lastProgressAt = nonce
      workerScope.postMessage({ type: 'progress', jobId: request.jobId, attempts: nonce })
    }
  }
}

workerScope.addEventListener('message', (event: MessageEvent<SolveRequest | CancelRequest>) => {
  const request = event.data
  if (request.type === 'cancel') {
    if (activeJobId === request.jobId) activeJobId = 0
    return
  }
  void solve(request).catch((error: unknown) => {
    workerScope.postMessage({
      type: 'error',
      jobId: request.jobId,
      message: error instanceof Error ? error.message : 'Failed to solve registration proof'
    })
  })
})

export {}
