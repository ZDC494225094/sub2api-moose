// Structural regression checks. They complement tests; they do not prove runtime
// compatibility or that every unmigrated customization has been identified.
export function evaluateContracts(config, { readLocal, readUpstream }) {
  if (config.version !== 1) throw new Error('Unsupported customization contract version')
  const violations = []
  const normalized = value => value?.replace(/\r\n/g, '\n')
  const report = (rule, file, detail) => violations.push({ rule, file, detail })
  for (const file of config.required_files) {
    if (readLocal(file) === undefined) report('required-file', file, 'Extension-owned implementation is missing')
  }
  for (const file of config.native_equivalent) {
    const local = readLocal(file)
    const upstream = readUpstream(file)
    if (local === undefined || upstream === undefined) report('native-equivalent', file, 'Cannot verify local and target upstream source')
    else if (normalized(local) !== normalized(upstream)) report('native-equivalent', file, 'Restored native file differs from the selected upstream target')
  }
  for (const { file, patterns } of config.forbidden_references) {
    const source = readLocal(file)
    if (source === undefined) { report('boundary', file, 'Guarded host source is missing'); continue }
    for (const pattern of patterns) {
      if (new RegExp(pattern).test(source)) report('boundary', file, `Custom implementation leaked back into the host: ${pattern}`)
    }
  }
  for (const { file, marker } of config.required_markers) {
    if (!readLocal(file)?.includes(marker)) report('host-seam', file, `Missing extension integration point: ${marker}`)
  }
  return violations
}

// The immutable initial path list makes accidental omission visible when files
// are moved. Explicit relocation destinations are also checked, not just names.
export function evaluateInventory(config, exists) {
  if (config.version !== 1) throw new Error('Unsupported customization inventory version')
  const violations = []
  const files = new Map(config.files.map(entry => [entry.path, entry]))
  const relocations = new Map()
  for (const entry of config.relocations) {
    if (!files.has(entry.from) || relocations.has(entry.from) || !entry.to?.length) {
      violations.push({ file: entry.from, detail: 'Invalid or duplicate relocation declaration' })
      continue
    }
    relocations.set(entry.from, entry.to)
  }
  for (const entry of config.files) {
    // D records an upstream file already intentionally deleted in the original
    // customized commit; it is not a deletion introduced by this migration.
    if (entry.original_change === 'D') continue
    const expected = relocations.get(entry.path) ?? [entry.path]
    for (const destination of expected) {
      if (!exists(destination)) violations.push({ file: entry.path, detail: `Missing preserved/relocated implementation: ${destination}` })
    }
  }
  for (const entry of config.extractions) {
    for (const destination of entry.to) {
      if (!exists(destination)) violations.push({ file: entry.from, detail: `Missing extracted implementation: ${destination}` })
    }
  }
  return violations
}

// Wire can silently prune providers that are registered but never consumed.
// This is static reachability evidence, not a substitute for behavioral tests.
export function evaluateProviderReachability(providers, generated) {
  const stripComments = source => source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '')
  const names = [...stripComments(providers).matchAll(/^\s*(Provide[A-Za-z0-9_]+),/gm)].map(match => match[1])
  const runtime = stripComments(generated)
  return names.filter(name => !runtime.includes(`wiring.${name}(`)).map(name => ({
    rule: 'provider-runtime-reachability', file: 'backend/cmd/server/wire_gen.go',
    detail: `Registered extension provider ${name} is absent from generated runtime wiring`,
  }))
}
