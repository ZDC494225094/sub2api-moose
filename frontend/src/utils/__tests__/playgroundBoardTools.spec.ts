import { describe, expect, it } from 'vitest'
import { buildImageBoardTasks, buildImageBoardTasksFromThreads, buildVideoBoardTasksFromThreads, imageBoardRectsIntersect } from '../playgroundBoardTools'

describe('playground board tools', () => {
  it('groups image runs with the preceding user prompt and shows newest first', () => {
    const config = { size: '1024x1024' }
    const tasks = buildImageBoardTasks([
      { id: 'user-1', role: 'user', content: 'First prompt', createdAt: 1 },
      { id: 'image-1', role: 'assistant', content: '', createdAt: 2, imageConfig: config, images: ['first'] },
      { id: 'chat-1', role: 'assistant', content: 'Not an image run', createdAt: 3 },
      { id: 'user-2', role: 'user', content: 'Second prompt', createdAt: 4 },
      { id: 'image-2', role: 'assistant', content: '', createdAt: 5, imageConfig: config, pending: true }
    ])

    expect(tasks).toHaveLength(2)
    expect(tasks.map((task) => task.message.id)).toEqual(['image-2', 'image-1'])
    expect(tasks.map((task) => task.prompt)).toEqual(['Second prompt', 'First prompt'])
  })

  it('keeps image failures visible as board tasks', () => {
    const config = { size: '1024x1024' }
    const tasks = buildImageBoardTasks([
      { id: 'user', role: 'user', content: 'A failed image', createdAt: 1 },
      { id: 'image', role: 'assistant', content: 'Generation failed', createdAt: 2, imageConfig: config, error: true }
    ])

    expect(tasks[0]).toMatchObject({
      prompt: 'A failed image',
      message: { id: 'image', error: true }
    })
  })

  it('collects image results from every image conversation', () => {
    const config = { size: '1024x1024' }
    const tasks = buildImageBoardTasksFromThreads([
      {
        id: 'image-thread-1',
        mode: 'image',
        messages: [
          { id: 'user-1', role: 'user' as const, content: 'First image', createdAt: 1 },
          { id: 'image-1', role: 'assistant' as const, content: '', createdAt: 2, imageConfig: config }
        ]
      },
      {
        id: 'chat-thread',
        mode: 'chat',
        messages: [
          { id: 'chat-user', role: 'user' as const, content: 'Chat only', createdAt: 3 },
          { id: 'chat-assistant', role: 'assistant' as const, content: 'Response', createdAt: 4, imageConfig: config }
        ]
      },
      {
        id: 'image-thread-2',
        mode: 'image',
        messages: [
          { id: 'user-2', role: 'user' as const, content: 'Latest image', createdAt: 5 },
          { id: 'image-2', role: 'assistant' as const, content: '', createdAt: 6, imageConfig: config }
        ]
      }
    ])

    expect(tasks.map((task) => [task.thread.id, task.message.id, task.prompt])).toEqual([
      ['image-thread-2', 'image-2', 'Latest image'],
      ['image-thread-1', 'image-1', 'First image']
    ])
  })

  it('collects video runs with their preceding prompts', () => {
    const tasks = buildVideoBoardTasksFromThreads([
      {
        id: 'video-thread',
        mode: 'video',
        messages: [
          { id: 'user-1', role: 'user' as const, content: 'First clip', createdAt: 1 },
          { id: 'video-1', role: 'assistant' as const, content: '', createdAt: 2, videos: ['first'] },
          { id: 'user-2', role: 'user' as const, content: 'Second clip', createdAt: 3 },
          { id: 'video-2', role: 'assistant' as const, content: '', createdAt: 4, pending: true }
        ]
      },
      {
        id: 'image-thread',
        mode: 'image',
        messages: [
          { id: 'user-3', role: 'user' as const, content: 'Ignore image', createdAt: 5 }
        ]
      }
    ])

    expect(tasks.map((task) => task.message.id)).toEqual(['video-2', 'video-1'])
    expect(tasks.map((task) => task.prompt)).toEqual(['Second clip', 'First clip'])
  })

  it('identifies cards touched by the box selection rectangle', () => {
    expect(imageBoardRectsIntersect(
      { left: 20, top: 20, right: 80, bottom: 80 },
      { left: 60, top: 60, right: 120, bottom: 120 }
    )).toBe(true)
    expect(imageBoardRectsIntersect(
      { left: 20, top: 20, right: 80, bottom: 80 },
      { left: 80, top: 80, right: 140, bottom: 140 }
    )).toBe(false)
  })
})
