export interface ImageBoardMessage<TImage, TConfig> {
  id: string
  role: 'user' | 'assistant'
  content: string
  createdAt: number
  images?: TImage[]
  imageConfig?: TConfig
  pending?: boolean
  progress?: string
  durationMs?: number
  error?: boolean
}

export interface ImageBoardTask<TMessage> {
  message: TMessage
  prompt: string
}

export interface ImageBoardThread<TMessage> {
  mode: string
  messages: TMessage[]
}

export interface ImageBoardThreadTask<TMessage, TThread> extends ImageBoardTask<TMessage> {
  thread: TThread
}

export interface VideoBoardMessage<TVideo> {
  id: string
  role: 'user' | 'assistant'
  content: string
  createdAt: number
  videos?: TVideo[]
  pending?: boolean
  progress?: string
  durationMs?: number
  error?: boolean
}

export interface VideoBoardThread<TMessage> {
  mode: string
  messages: TMessage[]
}

export interface VideoBoardThreadTask<TMessage, TThread> {
  message: TMessage
  prompt: string
  thread: TThread
}

export interface ImageBoardRect {
  left: number
  top: number
  right: number
  bottom: number
}

export function buildImageBoardTasks<
  TImage,
  TConfig,
  TMessage extends ImageBoardMessage<TImage, TConfig>
>(messages: TMessage[]): ImageBoardTask<TMessage>[] {
  let latestPrompt = ''
  const tasks: ImageBoardTask<TMessage>[] = []

  for (const message of messages) {
    if (message.role === 'user') {
      latestPrompt = message.content.trim()
      continue
    }
    if (!message.imageConfig) continue
    tasks.push({ message, prompt: latestPrompt })
  }

  return tasks.sort((left, right) => right.message.createdAt - left.message.createdAt)
}

export function buildImageBoardTasksFromThreads<
  TImage,
  TConfig,
  TMessage extends ImageBoardMessage<TImage, TConfig>,
  TThread extends ImageBoardThread<TMessage>
>(threads: TThread[]): ImageBoardThreadTask<TMessage, TThread>[] {
  return threads
    .filter((thread) => thread.mode === 'image')
    .flatMap((thread) => buildImageBoardTasks(thread.messages).map((task) => ({ ...task, thread })))
    .sort((left, right) => right.message.createdAt - left.message.createdAt)
}

export function buildVideoBoardTasksFromThreads<
  TVideo,
  TMessage extends VideoBoardMessage<TVideo>,
  TThread extends VideoBoardThread<TMessage>
>(threads: TThread[]): VideoBoardThreadTask<TMessage, TThread>[] {
  return threads
    .filter((thread) => thread.mode === 'video')
    .flatMap((thread) => {
      let latestPrompt = ''
      const tasks: VideoBoardThreadTask<TMessage, TThread>[] = []
      for (const message of thread.messages) {
        if (message.role === 'user') {
          latestPrompt = message.content.trim()
          continue
        }
        tasks.push({ message, prompt: latestPrompt, thread })
      }
      return tasks
    })
    .sort((left, right) => right.message.createdAt - left.message.createdAt)
}

export function imageBoardRectsIntersect(left: ImageBoardRect, right: ImageBoardRect): boolean {
  return left.left < right.right && left.right > right.left && left.top < right.bottom && left.bottom > right.top
}
