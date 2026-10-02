// Legacy settings tests exercise enabled custom controls; defaults stay fail-closed.
import { beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useExtensionStore } from '../store'
beforeEach(() => {
  setActivePinia(createPinia())
  useExtensionStore().flags = { 'access-policy': true }
})
