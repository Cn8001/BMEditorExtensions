import { reactive } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'

export let api: ExtensionAPI

export function setApi(a: ExtensionAPI) {
  api = a
}

export interface Model {
  id: string
  label: string
  size: number
  downloaded: boolean
}

export interface Status {
  provider: string
  ready: boolean
  why?: string
  busy: boolean
  models: Model[]
}

// What the status bar shows and the ghost text needs: the backend's status,
// whether a request is out, the settings, and the on/off switch.
export const state = reactive({
  status: null as Status | null,
  thinking: 0, // requests out
  error: '',
  on: true,
  settings: {} as Record<string, string>,
})

const offKey = 'bmeditor-autocomplete.off'

try {
  state.on = localStorage.getItem(offKey) !== '1'
} catch {
  // storage unavailable: on
}

export function setOn(on: boolean) {
  state.on = on
  try {
    if (on) localStorage.removeItem(offKey)
    else localStorage.setItem(offKey, '1')
  } catch {
    // only for this session then
  }
}

export async function refresh() {
  try {
    state.settings = (await api.settings()).values
  } catch {
    // keep the last
  }
  try {
    state.status = await api.call<Status>('status')
  } catch (err) {
    state.error = api.ui.failureOf(err).message
  }
}

export function setting(key: string, fallback = ''): string {
  return state.settings[key] || fallback
}

// The model, as the status bar names it.
export function modelName(): string {
  const st = state.status
  if (!st) return ''
  if (st.provider === 'ollama') return setting('ollamaModel', 'qwen2.5-coder:1.5b')
  const id = setting('localModel', '0.5b')
  if (id === 'custom') return setting('modelFile').split(/[\\/]/).pop() || 'Own model'
  const m = st.models.find((x) => x.id === id)
  return m ? `Qwen2.5-Coder ${id.toUpperCase()}` : id.split(/[\\/]/).pop() ?? id
}

export function onGPU(): boolean {
  return setting('gpu', 'true') !== 'false'
}
