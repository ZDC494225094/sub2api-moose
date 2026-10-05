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
  const removedArtifacts = new Set()
  for (const entry of config.removed_artifacts ?? []) {
    if (!files.has(entry.path) || !/^generated\/[^/]+\.png$/.test(entry.path) || !entry.reason || removedArtifacts.has(entry.path)) {
      violations.push({ file: entry.path, detail: 'Invalid reviewed artifact removal' })
      continue
    }
    removedArtifacts.add(entry.path)
    if (exists(entry.path)) violations.push({ file: entry.path, detail: 'Removed runtime artifact reintroduced' })
  }
  for (const entry of config.files) {
    if (removedArtifacts.has(entry.path)) continue
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

// Evidence is deliberately explicit: catalog coverage, a live call site, an
// executable regression test and a feature delta against the frozen baseline.
// This is NOT semantic call-graph analysis or proof that a test was executed.
export function evaluateRuntimeEvidence(config, catalog, { readLocal, featureInBaseline }) {
  if (config.version !== 1) throw new Error('Unsupported runtime evidence version')
  const failures = []
  const fail = (file, detail) => failures.push({ rule: 'runtime-evidence', file, detail })
  const seen = new Set()
  for (const entry of config.entries) {
    const key = `${entry.module}:${entry.checkpoint}`
    if (seen.has(key)) fail('customizations/runtime-evidence.json', `Duplicate ${key}`)
    seen.add(key)
    if (!catalog.some(m => m.id === entry.module && m.managed)) fail('customizations/runtime-evidence.json', `Not managed: ${key}`)
    for (const kind of ['read', 'call', 'test', 'feature']) {
      const ref = entry[kind]
      const source = ref && readLocal(ref.file)
      if (!source || !ref.marker || !source.includes(ref.marker)) fail(ref?.file, `${key}: missing ${kind} marker`)
      if (kind !== 'test' && /(?:_test\.go|\.spec\.|__tests__)/.test(ref?.file ?? '')) fail(ref.file, `${key}: production evidence points to test`)
      if (kind === 'test' && !/(?:_test\.go|\.spec\.)/.test(ref?.file ?? '')) fail(ref.file, `${key}: not an executable test file`)
      if (kind === 'test' && source && ref.marker) {
        const declaration = ref.file.endsWith('_test.go')
          ? `func ${ref.marker}(t *testing.T)` : `it('${ref.marker}'`
        const index = source.indexOf(declaration)
        if (index < 0) fail(ref.file, `${key}: test marker is not a test declaration`)
        else {
          const rest = source.slice(index)
          const next = rest.slice(1).search(ref.file.endsWith('_test.go') ? /^func /m : /\n\s*(?:it|test)\(/)
          const body = next < 0 ? rest : rest.slice(0, next + 1)
          if (!/(?:require\.|assert\.|expect\(|t\.(?:Fatal|Error))/.test(body)) fail(ref.file, `${key}: test has no regression assertion`)
        }
      }
      if ((kind === 'call' || kind === 'read') && source) {
        const code = source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^[ \t]*\/\/.*$/gm, '')
        if (!code.includes(ref.marker)) fail(ref.file, `${key}: marker exists only in comments`)
      }
    }
    if (!entry.scope) fail(entry.call.file, `${key}: missing boundary explanation`)
    if (featureInBaseline(entry.feature.marker)) fail(entry.feature.file, `${key}: claimed fork feature marker exists in upstream baseline`)
  }
  for (const module of catalog.filter(m => m.managed)) {
    const entries = config.entries.filter(e => e.module === module.id)
    if (!entries.length) fail('customizations/runtime-evidence.json', `Missing managed module ${module.id}`)
    for (const gate of module.gates ?? []) if (!entries.some(e => e.checkpoint === gate)) fail('customizations/runtime-evidence.json', `Missing checkpoint ${module.id}:${gate}`)
  }
  return failures
}

export function renderRuntimeEvidence(config, readLocal) {
  const location = ref => {
    const source = readLocal(ref.file) ?? ''
    const index = source.indexOf(ref.marker)
    return `${ref.file}:${index < 0 ? '?' : source.slice(0, index).split('\n').length}`
  }
  return ['# 扩展开关可达性检查表', '', `固定上游基线：${config.baseline}`, '',
    '本表由 runtime-evidence.json 生成；行号随工作区源码更新。结构门禁检查实际标记、目录覆盖和上游特征缺失，不冒充 Go 调用图分析，也不代表测试已经执行或生产验收。测试结果以 verification 日志为准。', '',
    '| 模块 / 检查点 | 读开关 | 非测试调用 | 关闭分支回归测试 | 二开特征（固定基线全文检索不存在） | 范围 |',
    '|---|---|---|---|---|---|',
    ...config.entries.map(e => `| ${e.module} / ${e.checkpoint} | ${location(e.read)} | ${location(e.call)} | ${location(e.test)} — ${e.test.marker} | ${location(e.feature)} — ${e.feature.marker} | ${e.scope} |`), '',
    '上游保护：组合调度文件保持与上游一致；分时定价、高峰倍率不读扩展开关。宿主历史 500 号迁移不删除、不改校验和。', ''].join('\n')
}
