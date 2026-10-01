#!/usr/bin/env node
import { spawnSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, isAbsolute, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parseVerificationArgs, verificationPlan, verificationEnvironment } from './lib/customization-verification.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
if (process.argv.includes('--help')) {
  console.log('node tools/customization-verify.mjs [--check | --full | --build] [--upstream <local-ref>]')
  console.log('check: boundary/inventory audit; full: all Go/frontend tests, lint and types; build: full plus frontend/canvas/embedded binary. No fetch, merge, install, database or deployment.')
  process.exit(0)
}
const report = { started_at: new Date().toISOString(), mode: '', upstream: '', complete: false, steps: [] }
const reportDir = resolve(root, '.tmp/customization-verification')
function inside(path) {
  const target = resolve(root, path), rel = relative(root, target)
  if (!rel || rel.startsWith('..') || isAbsolute(rel)) throw new Error(`Path escapes workspace: ${path}`)
  return target
}
try {
  const options = parseVerificationArgs(process.argv.slice(2))
  const config = JSON.parse(readFileSync(resolve(root, 'customizations/upstream.json'), 'utf8'))
  report.mode = options.mode; report.upstream = options.upstream ?? config.merged_upstream
  mkdirSync(reportDir, { recursive: true })
  const steps = verificationPlan({ root, node: process.execPath, platform: process.platform, mode: options.mode, upstream: report.upstream })
  for (const step of steps) {
    const started = Date.now()
    const entry = { id: step.id, ok: false, duration_ms: 0 }
    report.steps.push(entry)
    console.log(`\n[customizations] ${step.id}`)
    for (const path of step.requires ?? []) if (!existsSync(inside(path))) throw new Error(`Missing prerequisite ${path}; install the locked dependencies or fix the preceding build, then retry.`)
    if (step.copy) {
      // Copy only our generated canvas assets after Vite empties the host dist.
      // Never move/delete source or user data as part of verification.
      cpSync(inside(step.copy.from), inside(step.copy.to), { recursive: true })
    } else {
      if (step.id === 'backend-embed') mkdirSync(inside('.tmp/customization-release'), { recursive: true })
      const result = spawnSync(step.executable, step.args, { cwd: step.cwd, stdio: 'inherit', shell: false, env: verificationEnvironment(step, process.env) })
      if (result.error) throw result.error
      if (result.status !== 0) { entry.exit_code = result.status; process.exitCode = result.status || 1; throw new Error(`${step.id} failed; later steps were not run`) }
    }
    entry.ok = true; entry.duration_ms = Date.now() - started
  }
  report.complete = true
  console.log(options.mode === 'build' ? '\nVerified local combined build: .tmp/customization-release/ (not deployed).' : '\nRequested checks passed. This does not certify unmigrated modules or future upstream compatibility.')
} catch (error) {
  report.error = error.message
  console.error(`[customizations] ${error.message}`)
  process.exitCode ||= 1
} finally {
  report.finished_at = new Date().toISOString()
  mkdirSync(reportDir, { recursive: true })
  writeFileSync(resolve(reportDir, 'result.json'), JSON.stringify(report, null, 2) + '\n')
}
