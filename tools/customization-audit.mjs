#!/usr/bin/env node
// Read-only Git audit. Never fetches, merges, resets, checks out, or deletes files.
import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs'
import { dirname, resolve, relative, isAbsolute } from 'node:path'
import { fileURLToPath } from 'node:url'
import { evaluateContracts, evaluateInventory, evaluateProviderReachability } from './lib/customization-contracts.mjs'
import { evaluateOwnership, ownershipCheckFailed } from './lib/customization-ownership.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const args = process.argv.slice(2)
if (args.includes('--help')) {
  console.log('node tools/customization-audit.mjs [--upstream origin/main] [--output .tmp/customization-audit.json] [--check]')
  process.exit(0)
}
const allowed = new Set(['--upstream', '--output', '--check'])
for (let i = 0; i < args.length; i++) {
  if (!allowed.has(args[i])) throw new Error(`Unknown argument: ${args[i]}`)
  if (args[i] !== '--check') {
    if (!args[i + 1] || args[i + 1].startsWith('--')) throw new Error(`Missing value: ${args[i]}`)
    i++
  }
}
const option = (name, fallback) => args.includes(name) ? args[args.indexOf(name) + 1] : fallback
const git = (...gitArgs) => execFileSync('git', ['-c', 'core.quotepath=false', ...gitArgs], { cwd: root, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 })
const paths = (...gitArgs) => git(...gitArgs).split('\0').filter(Boolean)
const config = JSON.parse(readFileSync(resolve(root, 'customizations/upstream.json'), 'utf8'))
const ownership = JSON.parse(readFileSync(resolve(root, 'customizations/ownership.json'), 'utf8'))
const reviews = JSON.parse(readFileSync(resolve(root, 'customizations/ownership-reviewed.json'), 'utf8'))
const inventoryConfig = JSON.parse(readFileSync(resolve(root, 'customizations/inventory-baseline.json'), 'utf8'))
const base = git('rev-parse', '--verify', '--end-of-options', `${config.merged_upstream}^{commit}`).trim()
const targetRef = option('--upstream', `${config.upstream_remote}/main`)
const target = git('rev-parse', '--verify', '--end-of-options', `${targetRef}^{commit}`).trim()
const upstreamFiles = new Set(paths('ls-tree', '-r', '--name-only', '-z', base))
const localChanges = [...new Set([...paths('diff', '--name-only', '-z', base, '--'), ...paths('ls-files', '--others', '--exclude-standard', '-z')])].sort()
const incoming = new Set(paths('diff', '--name-only', '-z', base, target, '--'))
// Historical entries stay valid when their file is restored to upstream equivalence
// or explicitly relocated. A typo must not silently create a phantom owner.
const ownershipResult = evaluateOwnership(ownership, reviews, {
  paths: localChanges,
  knownPaths: [...localChanges, ...inventoryConfig.files.map(file => file.path)],
  baseline: base,
})
const modules = ownershipResult.modules
const files = ownershipResult.files.map(file => ({
  ...file, modifies_upstream_file: upstreamFiles.has(file.path), also_changed_upstream: incoming.has(file.path),
}))
const seams = [
  ['backend/internal/repository/ent.go', 'return ApplyMigrations(ctx, drv.DB())'],
  ['backend/internal/repository/migrations_runner.go', 'return applyMigrationsFS(ctx, db, migrations.FS, builtInExtensionMigrations())'],
  ['backend/internal/repository/migrations_runner.go', 'extensions[0].Prepare(ctx, lockConn)'],
  ['backend/internal/repository/migrations_runner.go', 'extensions[0].Apply(ctx, lockConn)'],
  ['backend/internal/setup/setup.go', 'return repository.ApplyMigrations(migrationCtx, db)'],
  ['backend/internal/server/router.go', 'r.Use(settingService.CustomExtensions().PageMiddleware())'],
  ['backend/internal/server/router.go', 'settingService.CustomExtensions().PublicState'],
  ['backend/internal/server/routes/admin.go', 'settingService.CustomExtensions().AdminUpdate'],
  ['backend/internal/server/routes/admin.go', 'registerCustomDashboardRoutes(dashboard, h, settingService.CustomExtensions())'],
  ['backend/internal/server/routes/user.go', 'registerCustomUserRoutes(authenticated, h, settingService.CustomExtensions())'],
  ['backend/internal/server/routes/payment.go', 'registerCustomPaymentUserRoutes(authenticated, extensions, settingService.CustomExtensions())'],
  ['backend/internal/server/routes/payment.go', 'registerCustomPaymentPublicRoutes(public, extensions, settingService.CustomExtensions(), panelRateLimiter.PublicIP())'],
  ['backend/internal/server/routes/payment.go', 'registerCustomPaymentAdminRoutes(adminGroup, extensions, settingService.CustomExtensions())'],
  ['backend/internal/service/recharge_campaign.go', 's.rechargeCampaignExtensionEnabled(ctx)'],
  ['frontend/src/router/index.ts', '...customExtensionRoutes'],
  ['frontend/src/router/index.ts', 'installCustomExtensionGuard(router)'],
  ['frontend/src/App.vue', 'useCustomExtensionRuntime(router)'],
  ['frontend/src/components/layout/AppSidebar.vue', 'extensions.pathEnabled(item.path)'],
  ['frontend/src/components/layout/AppSidebar.vue', "path: '/admin/custom-extensions'"],
  ['frontend/src/views/user/PaymentView.vue', 'useCheckoutExtensions({'],
  ['frontend/src/extensions/modules/recharge-campaigns/RechargeCampaignTicker.vue', "extensions.enabled('recharge-campaigns')"],
].map(([file, marker]) => ({ file, marker, present: existsSync(resolve(root, file)) && readFileSync(resolve(root, file), 'utf8').includes(marker) }))
const contractConfig = JSON.parse(readFileSync(resolve(root, 'customizations/contracts.json'), 'utf8'))
const readLocal = file => {
  const absolute = resolve(root, file)
  const rel = relative(root, absolute)
  if (!rel || rel.startsWith('..') || isAbsolute(rel)) throw new Error(`Contract path escapes repository: ${file}`)
  return existsSync(absolute) ? readFileSync(absolute) : undefined
}

const inventoryViolations = evaluateInventory(inventoryConfig, file => readLocal(file) !== undefined)
const contractViolations = evaluateContracts(contractConfig, {
  readLocal: file => readLocal(file)?.toString('utf8'),
  readUpstream: file => {
    try { return git('show', `${target}:${file}`) } catch { return undefined }
  },
})
contractViolations.push(...evaluateProviderReachability(readLocal('backend/internal/customize/wiring/providers.go').toString('utf8'), readLocal('backend/cmd/server/wire_gen.go').toString('utf8')))

const report = {
  generated_at: new Date().toISOString(),
  baseline: { ref: config.merged_upstream_label, sha: base },
  target: { ref: targetRef, sha: target, date: git('show', '-s', '--format=%cI', target).trim(), note: 'Local Git ref only; run git fetch yourself to refresh. File overlap indicates review risk, not a proven merge conflict.' },
  totals: { changed_files: files.length, modified_upstream_files: files.filter(f => f.modifies_upstream_file).length, incoming_overlap: files.filter(f => f.also_changed_upstream).length, ...ownershipResult.totals, missing_host_seams: seams.filter(s => !s.present).length, contract_violations: contractViolations.length, missing_inventory_paths: inventoryViolations.length },
  modules, seams, ownership_violations: ownershipResult.violations, contract_violations: contractViolations, inventory: { head: inventoryConfig.customized_head, original_paths: inventoryConfig.files.length, violations: inventoryViolations }, files,
}
const output = option('--output', '')
if (output) {
  const absolute = resolve(root, output)
  const rel = relative(root, absolute)
  if (rel === '' || rel.startsWith('..') || isAbsolute(rel)) throw new Error('Report output must stay inside the repository')
  mkdirSync(dirname(absolute), { recursive: true })
  writeFileSync(absolute, `${JSON.stringify(report, null, 2)}\n`)
  console.log(`Report: ${absolute}`)
}
console.log(JSON.stringify({ baseline: report.baseline, target: report.target, totals: report.totals, modules }, null, 2))
for (const seam of seams.filter(s => !s.present)) console.error(`MISSING HOST SEAM: ${seam.file}: ${seam.marker}`)
for (const violation of inventoryViolations) console.error(`INVENTORY LOSS: ${violation.file}: ${violation.detail}`)
for (const violation of contractViolations) console.error(`CONTRACT ${violation.rule}: ${violation.file}: ${violation.detail}`)
for (const violation of ownershipResult.violations) console.error(`OWNERSHIP: ${violation.file}: ${violation.detail}`)
for (const file of files.filter(file => file.ownership_source === 'unclassified')) console.error('UNCLASSIFIED: ' + file.path)
if (args.includes('--check') && (ownershipCheckFailed(ownershipResult) || report.totals.missing_host_seams || report.totals.incoming_overlap || contractViolations.length || inventoryViolations.length)) process.exitCode = 2
