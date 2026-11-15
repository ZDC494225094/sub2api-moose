import { readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { gzipSync } from 'node:zlib'

// Run after: vite build --manifest --outDir <directory>
// Only static dependencies are counted; deferred interactions are intentionally excluded.
const directory = resolve(process.argv[2] || 'backend/internal/web/dist')
const manifest = JSON.parse(readFileSync(resolve(directory, '.vite/manifest.json'), 'utf8'))
const roots = ['index.html', 'src/features/premium-home/runtime/PremiumHomeView.vue',
  'src/i18n/locales/zh/public.ts']
const visited = new Set()
function visit(key) {
  if (visited.has(key)) return
  const entry = manifest[key]
  if (!entry) throw new Error(`Missing manifest entry: ${key}`)
  visited.add(key)
  for (const dependency of entry.imports || []) visit(dependency)
}
roots.forEach(visit)
if ([...visited].some(key => /src\/i18n\/locales\/(zh|en)\/index\.ts/.test(key))) {
  throw new Error('Homepage eagerly loads a complete console translation dictionary')
}
const files = [...new Set([...visited].flatMap(key => [manifest[key].file, ...manifest[key].css || []]))]
const forbidden = files.filter(file => /vendor-(three|xlsx|chart|airwallex|export|qrcode|tour|stripe)|premium-home-globe|AdminComplianceDialog|AnnouncementPanel|ModelPlazaContent|SupportWidget|\/custom-/.test(file))
if (forbidden.length) throw new Error(`Homepage eagerly loads optional features: ${forbidden.join(', ')}`)
const report = {
  scope: 'Chinese premium homepage static JS/CSS dependencies, excluding API responses, images and deferred features',
  files: files.map(file => ({ file, bytes: statSync(resolve(directory, file)).size,
    gzipBytes: gzipSync(readFileSync(resolve(directory, file))).length })),
}
report.requests = report.files.length
report.bytes = report.files.reduce((sum, file) => sum + file.bytes, 0)
report.gzipBytes = report.files.reduce((sum, file) => sum + file.gzipBytes, 0)
console.log(JSON.stringify(report, null, 2))
