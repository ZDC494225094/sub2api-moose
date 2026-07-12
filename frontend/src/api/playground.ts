import { apiClient } from './client'

export interface PlaygroundModel {
  id: string
  label: string
  owned_by?: string
}

export type PlaygroundChatContent =
  | string
  | Array<
    | { type: 'text'; text: string }
    | { type: 'image_url'; image_url: { url: string } }
  >

export interface PlaygroundChatMessage {
  role: 'system' | 'user' | 'assistant'
  content: PlaygroundChatContent
}

export interface PlaygroundChatRequest {
  apiKey: string
  endpointBase?: string
  displayEndpoint?: string
  model: string
  messages: PlaygroundChatMessage[]
  temperature?: number
  maxTokens?: number | null
  topP?: number | null
  presencePenalty?: number | null
  frequencyPenalty?: number | null
  signal?: AbortSignal
}

export interface PlaygroundChatResponse {
  content: string
  raw: unknown
}

export interface PlaygroundChatStreamRequest extends PlaygroundChatRequest {
  onDelta?: (delta: string, event: unknown) => void
  onEvent?: (event: unknown) => void
}

export interface PlaygroundImageRequest {
  apiKey: string
  endpointBase?: string
  displayEndpoint?: string
  model: string
  prompt: string
  size: string
  n: number
  quality?: string
  background?: string
  outputFormat?: string
  signal?: AbortSignal
}

export interface PlaygroundImageInput {
  name: string
  type?: string
  dataUrl: string
  storageId?: string
}

export interface PlaygroundImageResult {
  url: string
  revisedPrompt?: string
  assetIndex?: number
  mimeType?: string
}

export interface PlaygroundImageResponse {
  images: PlaygroundImageResult[]
  raw: unknown
}

export type PlaygroundRunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled'

export interface PlaygroundRunRequest {
  id?: string
  mode: 'chat' | 'image'
  apiKey: string
  endpointBase?: string
  displayEndpoint?: string
  model: string
  messages?: PlaygroundChatMessage[]
  temperature?: number
  maxTokens?: number | null
  topP?: number | null
  presencePenalty?: number | null
  frequencyPenalty?: number | null
  prompt?: string
  size?: string
  n?: number
  quality?: string
  background?: string
  outputFormat?: string
  images?: PlaygroundImageInput[]
}

export interface PlaygroundRun {
  id: string
  mode: 'chat' | 'image'
  status: PlaygroundRunStatus
  model?: string
  content?: string
  images?: PlaygroundImageResult[]
  error?: string
  raw?: unknown
  createdAt?: string
  updatedAt?: string
  completedAt?: string
  durationMs?: number
}

function normalizeImageGenerationSize(size: string): string {
  const trimmed = size.trim().toLowerCase()
  if (!trimmed || trimmed === 'auto') return 'auto'

  const supported = new Set(['1024x1024', '1536x1024', '1024x1536'])
  if (supported.has(trimmed)) return trimmed

  const match = trimmed.match(/^(\d+)x(\d+)$/)
  if (!match) return 'auto'

  const width = Number(match[1])
  const height = Number(match[2])
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return 'auto'
  }
  if (width === height) return '1024x1024'
  return width > height ? '1536x1024' : '1024x1536'
}

function authHeaders(apiKey: string): HeadersInit {
  return {
    Authorization: `Bearer ${apiKey}`,
    'Content-Type': 'application/json'
  }
}

export function resolvePlaygroundRequestBase(displayEndpoint?: string, fallback = '/v1'): string {
  const raw = (displayEndpoint || '').trim()
  if (!raw) return fallback

  try {
    const parsed = new URL(raw, window.location.origin)
    const path = parsed.pathname.replace(/\/+$/, '')
    if (/\/antigravity\/v1beta$/i.test(path)) return '/antigravity/v1beta'
    if (/\/antigravity\/v1$/i.test(path)) return '/antigravity/v1'
    if (/\/v1beta$/i.test(path)) return '/v1beta'
    if (/\/v1$/i.test(path)) return '/v1'
    return fallback
  } catch {
    return fallback
  }
}

function hasVersionSuffix(raw: string): boolean {
  const trimmed = raw.trim().replace(/\/+$/, '')
  if (!trimmed) return false

  let pathValue = ''
  try {
    const parsed = new URL(trimmed, window.location.origin)
    pathValue = parsed.pathname
  } catch {
    const slash = trimmed.indexOf('/')
    pathValue = slash >= 0 ? trimmed.slice(slash) : ''
  }

  const segment = pathValue.replace(/\/+$/, '').split('/').filter(Boolean).pop() || ''
  return /^v\d+(?:(?:\.\d+)|(?:alpha.*|beta.*|preview.*))?$/i.test(segment)
}

export function buildPlaygroundEndpointURL(endpointBase: string | undefined, endpoint: string): string {
  const base = (endpointBase || '/v1').trim().replace(/\/+$/, '') || '/v1'
  const endpointPath = `/${endpoint.trim().replace(/^\/+/, '')}`
  const relativePath = endpointPath.replace(/^\/v1(?=\/|$)/, '')

  if (base.endsWith(endpointPath) || (relativePath && base.endsWith(relativePath))) {
    return base
  }
  if (hasVersionSuffix(base)) {
    return `${base}${relativePath}`
  }
  return `${base}${endpointPath}`
}

async function parseError(response: Response): Promise<Error> {
  const requestUrl = response.url || ''
  const contentType = response.headers.get('content-type') || ''
  const isHTML = contentType.includes('text/html')
  const body = isHTML ? null : await response.json().catch(() => null)
  const message =
    body?.error?.message ||
    body?.message ||
    body?.detail ||
    `${response.status} ${response.statusText}`.trim()
  const detailParts: string[] = []
  if (requestUrl) detailParts.push(`URL: ${requestUrl}`)
  if (response.status) detailParts.push(`HTTP ${response.status}`)

  if (isHTML) {
    return new Error(`当前请求返回了前端 HTML，通常是开发代理没有指向 Sub2API 后端。${detailParts.join(' · ')}`)
  }

  if (response.status === 503 && /temporarily unavailable/i.test(message)) {
    return new Error(`${message}。后端已收到请求，但并发槽位、账号调度或上游可用性检查失败；请确认所选 API Key 的分组有可用账号，Redis/数据库连接正常。${detailParts.join(' · ')}`)
  }

  return new Error(detailParts.length ? `${message} (${detailParts.join(' · ')})` : message)
}

async function parseJSONResponse(response: Response): Promise<unknown> {
  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('text/html')) {
    throw new Error(`当前请求返回了前端 HTML，通常是开发代理没有指向 Sub2API 后端。URL: ${response.url}`)
  }
  return response.json()
}

function readModelID(entry: unknown): string {
  if (typeof entry === 'string') return entry
  if (!entry || typeof entry !== 'object') return ''
  const record = entry as Record<string, unknown>
  const value = record.id || record.name || record.model || record.display_name
  return typeof value === 'string' ? value.replace(/^models\//, '').trim() : ''
}

function readModelOwner(entry: unknown): string | undefined {
  if (!entry || typeof entry !== 'object') return undefined
  const record = entry as Record<string, unknown>
  const owner = record.owned_by || record.owner || record.provider
  return typeof owner === 'string' && owner.trim() ? owner.trim() : undefined
}

function extractModelEntries(payload: unknown): unknown[] {
  if (Array.isArray(payload)) return payload
  if (!payload || typeof payload !== 'object') return []
  const record = payload as Record<string, unknown>
  if (Array.isArray(record.data)) return record.data
  if (Array.isArray(record.models)) return record.models
  if (record.data && typeof record.data === 'object') {
    const data = record.data as Record<string, unknown>
    if (Array.isArray(data.models)) return data.models
  }
  return []
}

export async function fetchModels(apiKey: string, endpointBase?: string, signal?: AbortSignal): Promise<PlaygroundModel[]> {
  const response = await fetch(buildPlaygroundEndpointURL(endpointBase, '/v1/models'), {
    method: 'GET',
    headers: { Authorization: `Bearer ${apiKey}` },
    signal
  })
  if (!response.ok) {
    throw await parseError(response)
  }

  const payload = await parseJSONResponse(response)
  const entries = extractModelEntries(payload)
  const seen = new Set<string>()
  const models: PlaygroundModel[] = []

  for (const entry of entries) {
    const id = readModelID(entry)
    if (!id || seen.has(id)) continue
    seen.add(id)
    models.push({
      id,
      label: id,
      owned_by: readModelOwner(entry)
    })
  }

  return models
}

function extractChatText(payload: unknown): string {
  const record = payload as Record<string, any>
  const message = record?.choices?.[0]?.message
  const content = message?.content
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    return content
      .map((item) => {
        if (typeof item === 'string') return item
        if (typeof item?.text === 'string') return item.text
        if (typeof item?.content === 'string') return item.content
        return ''
      })
      .filter(Boolean)
      .join('\n')
  }
  if (typeof record?.output_text === 'string') return record.output_text
  return ''
}

function messageHasContent(message: PlaygroundChatMessage): boolean {
  if (typeof message.content === 'string') return Boolean(message.content.trim())
  return message.content.some((item) => {
    if (item.type === 'text') return Boolean(item.text.trim())
    if (item.type === 'image_url') return Boolean(item.image_url.url.trim())
    return false
  })
}

function buildChatPayload(request: PlaygroundChatRequest, stream: boolean): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    model: request.model,
    messages: request.messages.filter(messageHasContent),
    stream
  }

  if (typeof request.temperature === 'number') {
    payload.temperature = request.temperature
  }
  if (request.maxTokens && request.maxTokens > 0) {
    payload.max_tokens = request.maxTokens
  }
  if (typeof request.topP === 'number') {
    payload.top_p = request.topP
  }
  if (typeof request.presencePenalty === 'number') {
    payload.presence_penalty = request.presencePenalty
  }
  if (typeof request.frequencyPenalty === 'number') {
    payload.frequency_penalty = request.frequencyPenalty
  }

  return payload
}

function extractStreamText(payload: unknown): string {
  const record = payload as Record<string, any>
  const delta = record?.choices?.[0]?.delta
  const content = delta?.content
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    return content
      .map((item) => {
        if (typeof item === 'string') return item
        if (typeof item?.text === 'string') return item.text
        if (typeof item?.content === 'string') return item.content
        return ''
      })
      .filter(Boolean)
      .join('')
  }
  if (typeof record?.delta === 'string') return record.delta
  if (typeof record?.text === 'string') return record.text
  return ''
}

function parseSSEBlock(block: string): string {
  return block
    .split(/\r?\n/)
    .filter((line) => line.startsWith('data:'))
    .map((line) => line.slice(5).trim())
    .join('\n')
}

function parseJSONOrNull(value: string): unknown | null {
  try {
    return JSON.parse(value)
  } catch {
    return null
  }
}

export async function runChatCompletion(request: PlaygroundChatRequest): Promise<PlaygroundChatResponse> {
  const payload = buildChatPayload(request, false)

  const response = await fetch(buildPlaygroundEndpointURL(request.endpointBase, '/v1/chat/completions'), {
    method: 'POST',
    headers: authHeaders(request.apiKey),
    body: JSON.stringify(payload),
    signal: request.signal
  })
  if (!response.ok) {
    throw await parseError(response)
  }

  const raw = await parseJSONResponse(response)
  return {
    content: extractChatText(raw),
    raw
  }
}

export async function streamChatCompletion(request: PlaygroundChatStreamRequest): Promise<PlaygroundChatResponse> {
  const payload = buildChatPayload(request, true)
  const response = await fetch(buildPlaygroundEndpointURL(request.endpointBase, '/v1/chat/completions'), {
    method: 'POST',
    headers: authHeaders(request.apiKey),
    body: JSON.stringify(payload),
    signal: request.signal
  })
  if (!response.ok) {
    throw await parseError(response)
  }

  if (!response.body) {
    const raw = await parseJSONResponse(response)
    return {
      content: extractChatText(raw),
      raw
    }
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  const rawEvents: unknown[] = []
  let buffer = ''
  let content = ''
  let rawText = ''
  let parsedStreamEvent = false
  const contentType = response.headers.get('content-type') || ''
  const looksLikeSSE = () => contentType.includes('event-stream') || /^data:/m.test(buffer)

  const processStreamData = (data: string) => {
    const trimmed = data.trim()
    if (!trimmed || trimmed === '[DONE]') return
    const event = parseJSONOrNull(trimmed)
    if (!event) return
    parsedStreamEvent = true
    rawEvents.push(event)
    request.onEvent?.(event)
    const delta = extractStreamText(event) || extractChatText(event)
    if (delta) {
      content += delta
      request.onDelta?.(delta, event)
    }
  }

  const processStreamBlock = (block: string) => {
    const trimmed = block.trim()
    if (!trimmed) return
    processStreamData(/^data:/m.test(trimmed) ? parseSSEBlock(trimmed) : trimmed)
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    const chunk = decoder.decode(value, { stream: true })
    rawText += chunk
    buffer += chunk

    if (looksLikeSSE()) {
      const blocks = buffer.split(/\r?\n\r?\n/)
      buffer = blocks.pop() || ''
      for (const block of blocks) {
        processStreamBlock(block)
      }
    } else {
      const lines = buffer.split(/\r?\n/)
      buffer = lines.pop() || ''
      for (const line of lines) {
        processStreamBlock(line)
      }
    }
  }

  const trailingChunk = decoder.decode()
  if (trailingChunk) {
    rawText += trailingChunk
    buffer += trailingChunk
  }

  processStreamBlock(buffer)

  if (!parsedStreamEvent) {
    const raw = parseJSONOrNull(rawText)
    const finalContent = raw ? extractChatText(raw) : content
    if (finalContent && !content) {
      request.onDelta?.(finalContent, raw)
    }
    return {
      content: finalContent,
      raw
    }
  }

  return {
    content,
    raw: rawEvents
  }
}

export async function generateImage(request: PlaygroundImageRequest): Promise<PlaygroundImageResponse> {
  const payload: Record<string, unknown> = {
    model: request.model,
    prompt: request.prompt,
    size: normalizeImageGenerationSize(request.size),
    n: request.n,
    response_format: 'b64_json'
  }

  if (request.quality && request.quality !== 'auto') {
    payload.quality = request.quality
  }
  if (request.background && request.background !== 'auto') {
    payload.background = request.background
  }
  if (request.outputFormat) {
    payload.output_format = request.outputFormat
  }

  const response = await fetch(buildPlaygroundEndpointURL(request.endpointBase, '/v1/images/generations'), {
    method: 'POST',
    headers: authHeaders(request.apiKey),
    body: JSON.stringify(payload),
    signal: request.signal
  })
  if (!response.ok) {
    throw await parseError(response)
  }

  const raw = await parseJSONResponse(response)
  const rawRecord = raw as Record<string, unknown>
  const data = Array.isArray(rawRecord.data) ? rawRecord.data : []
  const images = data
    .map((item: Record<string, unknown>) => {
      const b64 = typeof item.b64_json === 'string' ? item.b64_json : ''
      const url = typeof item.url === 'string'
        ? item.url
        : b64
          ? `data:image/${request.outputFormat || 'png'};base64,${b64}`
          : ''
      return {
        url,
        revisedPrompt: typeof item.revised_prompt === 'string' ? item.revised_prompt : undefined
      }
    })
    .filter((item: PlaygroundImageResult) => item.url)

  return { images, raw }
}

export async function startPlaygroundRun(request: PlaygroundRunRequest, signal?: AbortSignal): Promise<PlaygroundRun> {
  const { data } = await apiClient.post<PlaygroundRun>('/playground/runs', request, {
    timeout: 120000,
    signal
  })
  return data
}

export async function getPlaygroundRun(id: string, signal?: AbortSignal): Promise<PlaygroundRun> {
  const { data } = await apiClient.get<PlaygroundRun>(`/playground/runs/${encodeURIComponent(id)}`, {
    timeout: 30000,
    signal
  })
  return data
}

export async function getPlaygroundRunImage(id: string, index: number, signal?: AbortSignal): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/playground/runs/${encodeURIComponent(id)}/images/${index}`, {
    responseType: 'blob',
    timeout: 300000,
    signal
  })
  return data
}

export async function cancelPlaygroundRun(id: string): Promise<PlaygroundRun> {
  const { data } = await apiClient.delete<PlaygroundRun>(`/playground/runs/${encodeURIComponent(id)}`, {
    timeout: 60000
  })
  return data
}
