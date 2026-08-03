// 测试模型识别
function playgroundVideoModelKind(model) {
  const normalized = model.trim().toLowerCase()
  if (/(^|\/)grok-video-10(?:$|[-_:])/.test(normalized)) return 'grok-video-10'
  if (/(^|\/)grok-video-r(?:$|[-_:])/.test(normalized)) return 'grok-video-r'
  if (/(^|\/)(?:kling|keling)[-_:]?omni[-_:]?video(?:$|[-_:])/.test(normalized)) return 'kling-omni-video'
  if (/(^|\/)(?:kling|keling)(?:[-_:]?video)?(?:$|[-_:])/.test(normalized)) return 'kling-video'
  // Seedance 2.5: 支持 seedance-2.5 / doubao-seedance-2.5 / sd-2-5 / sd-2.5
  if (/(^|\/)(?:(?:doubao-)?seedance|sd)[-_:]?2(?:[._-]?5)(?:$|[-_:])/.test(normalized)) return 'seedance-2.5'
  // Seedance 2.0: 支持 seedance-2.0 / doubao-seedance-2.0 / sd-2-0 / sd-2.0
  if (/(^|\/)(?:(?:doubao-)?seedance|sd)[-_:]?2(?:[._-]?0)(?:$|[-_:])/.test(normalized)) return 'seedance-2.0'
  if (/(^|\/)(?:seedance|doubao-seedance)(?:$|[-_:])/.test(normalized)) return 'seedance'
  if (/(^|\/)gemini-omni-flash(?:$|[-_:])/.test(normalized)) return 'gemini-omni-flash'
  return 'default'
}

const testCases = [
  'doubao-seedance-2-0-260128-1080p',
  'sd-2-5',
  'sd-2-0',
  'sd-2.5',
  'sd-2.0',
  'sd_2_5',
  'seedance-2.5',
  'doubao-seedance-2.5',
  'kling-v1-6',
  'kling-omni-video',
  'sd-xl',      // 这应该不被识别（Stable Diffusion）
  'sdxl',       // 这应该不被识别
  'sd-1.5'      // 这应该不被识别
]

console.log('模型识别测试结果：\n')
testCases.forEach(model => {
  console.log(`${model.padEnd(35)} → ${playgroundVideoModelKind(model)}`)
})
