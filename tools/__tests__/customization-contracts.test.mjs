import test from 'node:test'
import assert from 'node:assert/strict'
import { evaluateContracts, evaluateInventory, evaluateProviderReachability } from '../lib/customization-contracts.mjs'

const config = {
  version: 1,
  native_equivalent: ['native.vue'],
  required_files: ['extension.ts'],
  forbidden_references: [{ file: 'dashboard.go', patterns: ['GetOperations'] }],
  required_markers: [{ file: 'app.vue', marker: 'ExtensionSlot' }],
}
const baseline = { 'native.vue': '<main>upstream</main>\n' }
const local = () => ({ ...baseline, 'extension.ts': 'export {}', 'dashboard.go': 'package dashboard', 'app.vue': '<ExtensionSlot />' })
const check = files => evaluateContracts(config, { readLocal: file => files[file], readUpstream: file => baseline[file] })

test('accepts intact boundaries and platform line-ending differences', () => {
  const files = local(); files['native.vue'] = files['native.vue'].replace(/\n/g, '\r\n')
  assert.deepEqual(check(files), [])
})
test('rejects copied native overrides, missing modules, leaked business logic and lost seams', () => {
  const files = local()
  files['native.vue'] += '<CustomWidget />'
  delete files['extension.ts']
  files['dashboard.go'] += ' GetOperations'
  files['app.vue'] = '<main />'
  assert.deepEqual(check(files).map(failure => failure.rule).sort(), ['boundary', 'host-seam', 'native-equivalent', 'required-file'])
})
test('does not report deleted core files as successfully isolated', () => {
  const files = local(); delete files['dashboard.go']; delete files['native.vue']
  assert.deepEqual(check(files).map(failure => failure.rule).sort(), ['boundary', 'native-equivalent'])
})
test('rejects unknown schema versions', () => {
  assert.throws(() => evaluateContracts({ ...config, version: 2 }, {}), /Unsupported/)
})

const inventory = {
  version: 1,
  files: [{ path: 'original.ts', original_change: 'A' }, { path: 'restored.vue', original_change: 'M' }, { path: 'already-deleted.ts', original_change: 'D' }],
  relocations: [{ from: 'original.ts', to: ['extension/original.ts'] }],
  extractions: [{ from: 'restored.vue', to: ['extension/widget.vue'] }],
}
test('path ledger accepts explicit moves and existing core files', () => {
  const files = new Set(['extension/original.ts', 'restored.vue', 'extension/widget.vue'])
  assert.deepEqual(evaluateInventory(inventory, file => files.has(file)), [])
})
test('path ledger does not conceal loss behind an old filename or move declaration', () => {
  const files = new Set(['original.ts', 'restored.vue'])
  assert.equal(evaluateInventory(inventory, file => files.has(file)).length, 2)
})
test('path ledger rejects duplicate or unknown move declarations', () => {
  const value = { ...inventory, relocations: [...inventory.relocations, ...inventory.relocations, { from: 'unknown.ts', to: [] }] }
  assert.equal(evaluateInventory(value, () => true).length, 2)
})

test('rejects extension providers silently pruned by Wire', () => {
  const providers = 'wire.NewSet(\n ProvideBillingSchedulingAdmission,\n)'
  assert.equal(evaluateProviderReachability(providers, '// nothing wired').length, 1)
  assert.equal(evaluateProviderReachability(providers, '// wiring.ProvideBillingSchedulingAdmission(settings)').length, 1)
  assert.deepEqual(evaluateProviderReachability(providers, 'wiring.ProvideBillingSchedulingAdmission(settings)'), [])
})
