<script setup lang="ts">
import { computed } from 'vue'
import type { MenuItem } from '../bmext/bmeditor'
import { change, getReady, toggle } from './actions'
import { api, modelName, onGPU, setting, state } from './state'

// The top bar's button: what the model is doing, and a menu that picks the
// model, the GPU and pauses suggestions, so nothing is typed in Settings.

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
  return `${(bytes / 1e9).toFixed(1)} GB`
}

const items = computed<MenuItem[]>(() => {
  const st = state.status
  if (!st) return [{ id: 'settings', label: 'Settings', icon: 'settings-gear' }]
  const out: MenuItem[] = []
  if (st.provider === 'off') {
    out.push({ id: 'provider:local', label: 'Turn on', detail: 'A model on this machine', icon: 'sparkle' })
  } else if (st.provider === 'ollama') {
    out.push({ id: 'h', heading: true, label: 'Ollama' })
    out.push({ id: 'x', label: setting('ollamaModel', 'qwen2.5-coder:1.5b'), icon: 'check', disabled: true })
    out.push({ id: 'provider:local', label: 'Run a model here instead', icon: 'blank' })
  } else {
    const current = setting('localModel', '0.5b')
    out.push({ id: 'h', heading: true, label: 'Model' })
    for (const m of st.models) {
      out.push({
        id: 'model:' + m.id,
        label: `Qwen2.5-Coder ${m.id.toUpperCase()}`,
        detail: m.downloaded ? 'Downloaded' : `Downloads ${gb(m.size)}`,
        icon: m.id === current ? 'check' : 'blank',
        disabled: st.busy,
      })
    }
    if (current === 'custom') out.push({ id: 'settings', label: modelName(), detail: 'A .gguf of your own', icon: 'check' })
    out.push({ id: 's1', separator: true })
    out.push({ id: 'gpu', label: 'Run on the GPU', detail: 'Vulkan: NVIDIA, AMD or Intel', icon: onGPU() ? 'check' : 'blank', disabled: st.busy })
  }
  out.push({ id: 's2', separator: true })
  if (st.busy) out.push({ id: 'cancel', label: 'Cancel download', icon: 'debug-stop' })
  else if (!st.ready && st.provider !== 'off') out.push({ id: 'ready', label: 'Get model ready', icon: 'cloud-download' })
  if (st.ready) out.push({ id: 'pause', label: state.on ? 'Pause suggestions' : 'Resume suggestions', icon: state.on ? 'debug-pause' : 'debug-start' })
  if (st.provider !== 'off') out.push({ id: 'provider:off', label: 'Turn off', icon: 'circle-slash' })
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
  else if (what === 'pause') toggle()
  else if (what === 'output') api.ui.showOutput()
  else if (what === 'settings') api.ui.openSettings()
}

// An editor before 0.1.6 has no menu to lend: a click does what it did.
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
