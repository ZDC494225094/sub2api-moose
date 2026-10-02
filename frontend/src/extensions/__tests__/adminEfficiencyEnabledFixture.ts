import { beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useExtensionStore } from '../store'

// Legacy behavior tests run with this newly isolated feature explicitly enabled.
beforeEach(() => {
  setActivePinia(createPinia())
  useExtensionStore().flags = { 'admin-efficiency': true }
})
