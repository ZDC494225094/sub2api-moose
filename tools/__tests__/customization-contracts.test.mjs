import test from 'node:test'
import assert from 'node:assert/strict'
import { evaluateContracts, evaluateInventory, evaluateProviderReachability, evaluateRuntimeEvidence } from '../lib/customization-contracts.mjs'

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

const evidenceFixture = () => ({ version: 1, entries: [{ module: 'fork', checkpoint: 'submit',
 read: { file: 'gate.go', marker: 'states.Enabled(' }, call: { file: 'host.go', marker: 'gate.Check(' },
 test: { file: 'host_test.go', marker: 'TestDisabled' }, feature: { file: 'host.go', marker: 'ForkFeature' }, scope: 'New work only',
}] })
const evidenceFiles = { 'gate.go': 'states.Enabled(ctx)', 'host.go': 'gate.Check(ctx); ForkFeature()', 'host_test.go': 'func TestDisabled(t *testing.T) { require.False(t, enabled) }' }
const checkEvidence = (config, files = evidenceFiles, native = false) => evaluateRuntimeEvidence(config, [{ id: 'fork', managed: true, gates: ['submit'] }], { readLocal: file => files[file], featureInBaseline: () => native })
test('runtime evidence rejects missing managed rows, calls, tests and upstream feature claims', () => {
 assert.deepEqual(checkEvidence(evidenceFixture()), [])
 assert.ok(checkEvidence({version:1,entries:[]}).length)
 for (const path of Object.keys(evidenceFiles)) { const files = {...evidenceFiles}; delete files[path]; assert.ok(checkEvidence(evidenceFixture(),files).length) }
 assert.ok(checkEvidence(evidenceFixture(),evidenceFiles,true).length)
 assert.ok(checkEvidence(evidenceFixture(),{...evidenceFiles,'host_test.go':'func TestDisabled(t *testing.T) {}'}).length)
 assert.ok(checkEvidence(evidenceFixture(),{...evidenceFiles,'host.go':'// gate.Check(ctx)\nForkFeature()'}).length)
 const config = evidenceFixture(); config.entries[0].call.file = 'host_test.go'; assert.ok(checkEvidence(config).length)
})
test('reviewed artifact removals cannot excuse implementation loss or restored runtime images', () => {
 const config = { version: 1, files: [{path:'generated/result.png'},{path:'service.go'}], relocations:[],extractions:[],removed_artifacts:[{path:'generated/result.png',reason:'runtime output'}] }
 assert.deepEqual(evaluateInventory(config, file => file === 'service.go'), [])
 assert.ok(evaluateInventory(config, () => true).length)
 config.removed_artifacts.push({path:'service.go',reason:'pretend artifact'})
 assert.ok(evaluateInventory(config, () => false).length)
})
