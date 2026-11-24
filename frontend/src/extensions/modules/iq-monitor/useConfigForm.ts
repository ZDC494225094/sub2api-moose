import { ref, watch, type Ref } from 'vue'
import { useAppStore } from '@/stores/app'
import { iqMonitorAdminAPI, type Config, type ConfigView } from './api'
import { errorMessage } from './format'

function plain(view: ConfigView): Config {
  return {
    probe_user_id: view.probe_user_id,
    interval_minutes: view.interval_minutes,
    timeout_seconds: view.timeout_seconds,
    request_retries: view.request_retries,
    max_output_tokens: view.max_output_tokens,
    retention_days: view.retention_days,
    logic: { ...view.logic },
    drawing: { ...view.drawing },
    thresholds: { ...view.thresholds },
    notify: { ...view.notify, bot_ids: [...(view.notify.bot_ids ?? [])] },
    submissions: { ...view.submissions },
    visibility: view.visibility,
  }
}

// Each settings panel edits a copy of the whole config and saves it whole;
// only one panel is on screen at a time, and each starts from the latest copy.
export function useConfigForm(source: Ref<ConfigView>, onSaved: (cfg: ConfigView) => void) {
  const appStore = useAppStore()
  const form = ref<Config>(plain(source.value))
  const saving = ref(false)
  const error = ref('')

  watch(source, (v) => { form.value = plain(v) })

  async function save(check?: (cfg: Config) => string) {
    error.value = check?.(form.value) ?? ''
    if (error.value) return
    saving.value = true
    try {
      const saved = await iqMonitorAdminAPI.saveConfig(form.value)
      appStore.showSuccess('设置已保存')
      onSaved(saved)
    } catch (e) {
      error.value = errorMessage(e, '保存失败')
    } finally {
      saving.value = false
    }
  }

  return { form, saving, error, save }
}
