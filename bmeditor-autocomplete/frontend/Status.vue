<script setup lang="ts">
import { computed } from 'vue'
import type { MenuItem } from '../bmext/bmeditor'
import { change, getReady, removeModel, removeUnused, toggle } from './actions'
import { api, modelName, onGPU, setting, state, type Model } from './state'

// The top bar's button: what the model is doing, and a menu that picks the
// model, where it runs and the GPU, so nothing is typed in Settings.

defineProps<{ compact?: number }>()

const view = computed(() => {
  const st = state.status
  if (!st) return { icon: 'sparkle', text: '', title: 'Autocomplete' }
  if (st.provider === 'off') return { icon: 'circle-slash', text: 'Off', title: 'Autocomplete is off' }
  if (st.busy) return { icon: 'loading codicon-modifier-spin', text: 'Getting ready', title: 'Downloading or loading the model: see Output' }
  if (!st.ready) return { icon: 'cloud-download', text: 'Get model', title: st.why ?? '' }
  if (!state.on) return { icon: 'circle-slash', text: 'Paused', title: `${modelName()}: suggestions paused` }
  if (state.error) return { icon: 'warning', text: modelName(), title: state.error }
  return {
    icon: state.thinking ? 'loading codicon-modifier-spin' : 'sparkle',
    text: modelName(),
    title: `${modelName()}${st.provider === 'local' && onGPU() ? ' on the GPU' : ''}: Tab takes a suggestion, Ctrl+Right a word, Esc puts it away, Alt+\\ asks`,
  }
})

function gb(bytes: number) {
  return bytes >= 1e9 ? `${(bytes / 1e9).toFixed(1)} GB` : `${Math.max(1, Math.round(bytes / 1e6))} MB`
}

// The backend's label, "Qwen2.5-Coder 0.5B: fast on any CPU, 0.5 GB", as
// a name and what it is good for.
function described(m: Model) {
  const [name, rest = ''] = m.label.split(': ')
  const what = rest.replace(/,\s*[\d.]+ GB$/, '')
  return { name, what: what.charAt(0).toUpperCase() + what.slice(1) }
}

const items = computed<MenuItem[]>(() => {
  const st = state.status
  if (!st) return [{ id: 'settings', label: 'Settings', icon: 'settings-gear' }]
  const out: MenuItem[] = []
  if (st.provider === 'local') {
    const current = setting('localModel', '0.5b')
    out.push({ id: 'h1', heading: true, label: 'Model' })
    for (const m of st.models) {
      const d = described(m)
      out.push({
        id: 'model:' + m.id,
        label: d.name,
        detail: `${d.what} · ${m.downloaded ? 'downloaded' : gb(m.size) + ' to download'}`,
        icon: m.id === current ? 'check' : 'blank',
        disabled: st.busy,
        // Downloads stay when BMEditor closes; this makes room.
        action: m.downloaded ? { id: 'delete:' + m.id, icon: 'trash', title: `Delete the download (${gb(m.size)})` } : undefined,
      })
    }
    out.push({
      id: 'custom',
      label: current === 'custom' ? modelName() : 'A .gguf of my own...',
      detail: current === 'custom' ? 'Your own model' : 'Set its path in Settings',
      icon: current === 'custom' ? 'check' : 'blank',
    })
    out.push({ id: 'gpu', label: 'Run on the GPU', detail: 'Vulkan: NVIDIA, AMD or Intel', icon: onGPU() ? 'check' : 'blank', disabled: st.busy })
  }
  out.push({ id: 'h2', heading: true, label: 'Runs on' })
  out.push({ id: 'provider:local', label: 'This machine', detail: 'llama.cpp, nothing leaves it', icon: st.provider === 'local' ? 'check' : 'blank' })
  out.push({
    id: 'provider:ollama',
    label: 'Ollama',
    detail: st.provider === 'ollama' ? setting('ollamaModel', 'qwen2.5-coder:1.5b') : 'A model you pulled',
    icon: st.provider === 'ollama' ? 'check' : 'blank',
  })
  out.push({ id: 'provider:off', label: 'Off', icon: st.provider === 'off' ? 'check' : 'blank' })
  out.push({ id: 's1', separator: true })
  if (st.busy) out.push({ id: 'cancel', label: 'Cancel download', icon: 'debug-stop' })
  else if (!st.ready && st.provider !== 'off') out.push({ id: 'ready', label: 'Get model ready', icon: 'cloud-download' })
  if (st.ready) out.push({ id: 'pause', label: state.on ? 'Pause suggestions' : 'Resume suggestions', icon: state.on ? 'debug-pause' : 'debug-start' })
  if (st.unused > 0 && !st.busy) {
    out.push({ id: 'unused', label: 'Remove unused downloads', detail: `Frees ${gb(st.unused)}`, icon: 'trash' })
  }
  out.push({ id: 'output', label: 'Show Output', icon: 'output' })
  out.push({ id: 'settings', label: 'Settings', icon: 'settings-gear' })
  return out
})

function select(id: string) {
  const [what, value] = id.split(':')
  if (what === 'model') void change({ provider: 'local', localModel: value })
  else if (what === 'provider') void change({ provider: value })
  else if (what === 'gpu') void change({ gpu: onGPU() ? 'false' : 'true' })
  else if (what === 'ready') void getReady()
  else if (what === 'cancel') void api.call('cancel')
  else if (what === 'unused') void removeUnused()
  else if (what === 'delete') {
    const m = state.status?.models.find((x) => x.id === value)
    if (m) void removeModel(m.id, described(m).name)
  }
  else if (what === 'pause') toggle()
  else if (what === 'output') api.ui.showOutput()
  else if (what === 'settings' || what === 'custom') api.ui.openSettings()
}

// An editor before 0.1.5 has no menu to lend: a click does what it did.
const DropMenu = api.components.DropMenu
function click() {
  const st = state.status
  if (st && !st.ready && !st.busy && st.provider !== 'off') void getReady()
  else if (st?.ready) toggle()
}
</script>

<template>
  <component :is="DropMenu" v-if="DropMenu" :items="items" align="right" :width="260" @select="select">
    <button class="ac" :title="view.title">
      <i class="codicon" :class="'codicon-' + view.icon" />
      <span v-if="view.text && !compact">{{ view.text }}</span>
    </button>
  </component>
  <button v-else class="ac" :title="view.title" @click="click">
    <i class="codicon" :class="'codicon-' + view.icon" />
    <span v-if="view.text && !compact">{{ view.text }}</span>
  </button>
</template>

<style scoped>
.ac {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 4px;
  color: var(--text-dim);
  font-size: 12px;
  white-space: nowrap;
}
.ac:hover {
  background: var(--bg-hover);
  color: var(--text);
}
</style>
