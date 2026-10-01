import { test } from 'node:test'
import assert from 'node:assert/strict'
import { resolve } from 'node:path'
import { parseVerificationArgs, verificationPlan, verificationEnvironment } from '../lib/customization-verification.mjs'

const plan = (mode, platform = 'linux') => verificationPlan({ root: '/workspace', node: '/node', platform, mode, upstream: 'verified-sha' })
test('defaults to read-only checks and rejects ambiguous modes and malformed refs', () => {
  assert.deepEqual(parseVerificationArgs([]), { mode: 'check', upstream: undefined })
  assert.deepEqual(parseVerificationArgs(['--build', '--upstream', 'origin/main']), { mode: 'build', upstream: 'origin/main' })
  for (const args of [['--build', '--full'], ['--upstream'], ['--upstream', '--all'], ['--force']]) assert.throws(() => parseVerificationArgs(args))
})
test('check does not run builds, databases, installers or destructive Git commands', () => {
  assert.deepEqual(plan('check').map(x => x.id), ['guard-tests', 'audit'])
  for (const step of plan('build')) assert.ok(!step.args?.some(x => ['fetch', 'merge', 'reset', 'checkout', 'install', 'generate', 'migrate'].includes(x)))
})
test('build cannot bypass full regressions, types or final embedded assets', () => {
  const steps = plan('build'), ids = steps.map(x => x.id)
  for (const step of plan('full')) assert.ok(ids.includes(step.id))
  assert.ok(ids.indexOf('frontend-build') > ids.indexOf('frontend-types'))
  assert.ok(ids.indexOf('canvas-embed') > ids.indexOf('canvas-build'))
  assert.ok(ids.indexOf('canvas-embed') > ids.indexOf('frontend-build'))
  assert.equal(ids.at(-1), 'backend-embed')
  assert.deepEqual(steps.find(x => x.id === 'canvas-embed').copy, { from: 'canvas/web/dist', to: 'backend/internal/web/dist/canvas' })
})
test('uses argument arrays and the Windows executable suffix without shell interpolation', () => {
  const step = plan('build', 'win32').at(-1)
  assert.ok(step.args.find(x => x.endsWith('sub2api.exe')))
  assert.equal(step.executable, 'go'); assert.equal(step.env.CGO_ENABLED, '0')
  assert.throws(() => plan('publish'))
})

test('every installed JS tool gets its own workspace dependency resolution context', () => {
  const steps = plan('build')
  for (const id of ['frontend-lint', 'frontend-tests', 'frontend-types', 'frontend-build']) {
    assert.equal(steps.find(step => step.id === id).nodeModules, resolve('/workspace/frontend/node_modules'))
  }
  assert.equal(steps.find(step => step.id === 'canvas-build').nodeModules, resolve('/workspace/canvas/web/node_modules'))
  for (const id of ['guard-tests', 'audit', 'backend-tests', 'backend-unit-tests', 'backend-embed']) {
    assert.equal(steps.find(step => step.id === id).nodeModules, undefined)
  }
})
test('pnpm hidden hoist is prepended without mutating or dropping inherited environment', () => {
  const step = plan('full').find(step => step.id === 'frontend-lint')
  const inherited = { PATH: '/bin', NODE_PATH: '/existing' }
  const env = verificationEnvironment(step, inherited, { platform: 'linux', exists: path => path === resolve(step.nodeModules, '.pnpm/node_modules') })
  assert.deepEqual(env, { PATH: '/bin', NODE_PATH: `${resolve(step.nodeModules, '.pnpm/node_modules')}:/existing` })
  assert.deepEqual(inherited, { PATH: '/bin', NODE_PATH: '/existing' })
})
test('Windows uses a semicolon and canonicalizes case-insensitive NODE_PATH', () => {
  const step = plan('full', 'win32').find(step => step.id === 'frontend-lint')
  const env = verificationEnvironment(step, { Node_Path: 'C:\\existing' }, { platform: 'win32', exists: () => true })
  assert.equal(env.NODE_PATH, `${resolve(step.nodeModules, '.pnpm/node_modules')};C:\\existing`)
  assert.equal(env.Node_Path, undefined)
})
test('npm installs and non-JS commands do not receive pnpm lookup paths', () => {
  const inherited = { PATH: '/bin', NODE_PATH: '/existing' }
  const step = plan('full').find(step => step.id === 'frontend-lint')
  assert.deepEqual(verificationEnvironment(step, inherited, { exists: () => false }), inherited)
  const backend = plan('build').at(-1)
  assert.deepEqual(verificationEnvironment(backend, inherited, { exists: () => true }), { ...inherited, CGO_ENABLED: '0' })
})


test('all verification modes run exact ownership and artifact guards before audit', () => {
  for (const mode of ['check', 'full', 'build']) {
    const steps = plan(mode)
    assert.equal(steps[0].id, 'guard-tests')
    assert.equal(steps[1].id, 'audit')
    for (const name of ['contracts', 'verification', 'ownership']) assert.ok(steps[0].args.includes(resolve('/workspace/tools/__tests__/customization-' + name + '.test.mjs')))
  }
})

test('Windows backend spawning preserves tools from duplicate PATH casing without mutating the caller', () => {
  const inherited = { PATH: 'C:/git/bin;C:/go/bin', Path: 'C:/go/bin;C:/windows' }
  const env = verificationEnvironment(plan('full', 'win32')[2], inherited, { platform: 'win32' })
  assert.equal(env.PATH, 'C:/git/bin;C:/go/bin;C:/windows')
  assert.equal(env.Path, undefined)
  assert.equal(inherited.Path, 'C:/go/bin;C:/windows')
})
test('POSIX PATH casing remains distinct and unchanged', () => {
  const inherited = { PATH: '/bin', Path: '/other' }
  assert.deepEqual(verificationEnvironment(plan('full')[2], inherited, { platform: 'linux' }), inherited)
})
