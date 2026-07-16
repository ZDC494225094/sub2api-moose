export function toCloneablePlaygroundState<T>(state: T): T {
  // Playground state is JSON-compatible; this also unwraps nested Vue proxies before IndexedDB clones it.
  return JSON.parse(JSON.stringify(state)) as T
}
