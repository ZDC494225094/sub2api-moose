// Kept separate from the WebGL renderer so the homepage can paint before Three.js loads.
export const globeProviderMarkers = [
  { id: 'openai', name: 'OpenAI', model: 'gpt' },
  { id: 'gemini', name: 'Gemini', model: 'gemini' },
  { id: 'deepseek', name: 'DeepSeek', model: 'deepseek' },
  { id: 'claude', name: 'Claude', model: 'claude' },
  { id: 'qwen', name: 'Qwen', model: 'qwen' },
  { id: 'grok', name: 'Grok', model: 'grok' },
] as const
