import { existsSync } from 'node:fs'
import { resolve } from 'node:path'

export function parseVerificationArgs(args) {
  let mode = 'check'
  let upstream
  let selectedMode = false
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]
    if (['--check', '--full', '--build'].includes(arg)) {
      if (selectedMode) throw new Error('Choose one of --check, --full or --build')
      mode = arg.slice(2); selectedMode = true
    } else if (arg === '--upstream') {
      upstream = args[++i]
      if (!upstream || upstream.startsWith('-')) throw new Error('--upstream requires a local Git ref')
    } else throw new Error(`Unknown argument: ${arg}`)
  }
  return { mode, upstream }
}

// All commands are argument arrays, not shell strings. No fetching, checkout,
// dependency installation, generated-source rewrite, database or deployment.
export function verificationPlan({ root, node, platform, mode, upstream }) {
  if (!['check', 'full', 'build'].includes(mode)) throw new Error('Invalid verification mode')
  const command = (id, executable, args, cwd = root, extra = {}) => ({ id, executable, args, cwd, ...extra })
  const js = (id, script, args, cwd = root) => command(id, node, [resolve(root, script), ...args], cwd, { requires: [script] })
  const localJS = (id, script, args, cwd) => ({ ...js(id, script, args, cwd), nodeModules: resolve(cwd, 'node_modules') })
  const front = resolve(root, 'frontend'), back = resolve(root, 'backend')
  const steps = [
    command('guard-tests', node, ['--test', resolve(root, 'tools/__tests__/customization-contracts.test.mjs'), resolve(root, 'tools/__tests__/customization-verification.test.mjs'), resolve(root, 'tools/__tests__/customization-ownership.test.mjs')]),
    js('audit', 'tools/customization-audit.mjs', ['--upstream', upstream, '--output', '.tmp/customization-verification/audit.json', '--check']),
  ]
  if (mode === 'check') return steps
  steps.push(
    command('backend-tests', 'go', ['test', '-mod=readonly', './...'], back),
    command('backend-unit-tests', 'go', ['test', '-mod=readonly', '-tags=unit', './...'], back),
    localJS('frontend-lint', 'frontend/node_modules/eslint/bin/eslint.js', ['.', '--ext', '.vue,.js,.jsx,.cjs,.mjs,.ts,.tsx,.cts,.mts'], front),
    localJS('frontend-tests', 'frontend/node_modules/vitest/vitest.mjs', ['run'], front),
    localJS('frontend-types', 'frontend/node_modules/vue-tsc/bin/vue-tsc.js', ['-b'], front),
  )
  if (mode !== 'build') return steps
  steps.push(
    localJS('frontend-build', 'frontend/node_modules/vite/bin/vite.js', ['build'], front),
    localJS('canvas-build', 'canvas/web/node_modules/vite/bin/vite.js', ['build', '--base=/canvas/'], resolve(root, 'canvas/web')),
    { id: 'canvas-embed', copy: { from: 'canvas/web/dist', to: 'backend/internal/web/dist/canvas' }, requires: ['canvas/web/dist/index.html', 'backend/internal/web/dist/index.html'] },
    command('backend-embed', 'go', ['build', '-mod=readonly', '-tags=embed', '-trimpath', '-o', resolve(root, '.tmp/customization-release', platform === 'win32' ? 'sub2api.exe' : 'sub2api'), './cmd/server'], back, { env: { CGO_ENABLED: '0' } }),
  )
  return steps
}

// pnpm's bin shims expose the hidden hoisted dependencies through NODE_PATH.
// Calling a JS entry directly is shell-free, but must preserve that resolution
// context (for example, ESLint resolves parsers relative to the project config).
// Restrict the extra lookup to the command's installed workspace dependencies;
// npm installs need no adjustment and the caller's environment is never mutated.
export function verificationEnvironment(step, inherited, { platform = process.platform, exists = existsSync } = {}) {
  const env = { ...inherited, ...step.env }
  // Windows can inherit both PATH and Path from a desktop/PowerShell host.
  // Merge before spawning so Node does not discard a configured tool directory.
  if (platform === 'win32') {
    const keys = Object.keys(env).filter(key => key.toUpperCase() === 'PATH')
    if (keys.length > 1) {
      const paths = keys.flatMap(key => (env[key] ?? '').split(';')).filter(Boolean)
      for (const key of keys) delete env[key]
      env.PATH = [...new Set(paths)].join(';')
    }
  }
  if (!step.nodeModules) return env
  const hoisted = resolve(step.nodeModules, '.pnpm/node_modules')
  if (!exists(hoisted)) return env
  const key = platform === 'win32' ? Object.keys(env).find(key => key.toUpperCase() === 'NODE_PATH') : 'NODE_PATH'
  const previous = key ? env[key] : undefined
  if (key && key !== 'NODE_PATH') delete env[key]
  env.NODE_PATH = [hoisted, previous].filter(Boolean).join(platform === 'win32' ? ';' : ':')
  return env
}
