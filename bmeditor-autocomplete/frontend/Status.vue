<script setup lang="ts">
import { computed } from 'vue'
import { getReady, toggle } from './actions'
import { modelName, state } from './state'

// The top bar's switch: what the model is doing, and a click to get it
// ready or to turn suggestions on and off.

defineProps<{ compact?: number }>()

const view = computed(() => {
  const st = state.status
  if (!st) return { icon: 'sparkle', text: '', title: 'Autocomplete' }
  if (st.provider === 'off') return { icon: 'circle-slash', text: 'Off', title: 'Autocomplete is off: Settings › Autocomplete › Model server' }
  if (st.busy) return { icon: 'loading codicon-modifier-spin', text: 'Getting ready', title: 'Downloading or loading the model: see Output' }
  if (!st.ready) return { icon: 'cloud-download', text: 'Get model', title: `${st.why ?? ''}\nClick to download and start it.` }
  if (!state.on) return { icon: 'circle-slash', text: 'Paused', title: `${modelName()}: suggestions paused. Click to turn them on.` }
  if (state.error) return { icon: 'warning', text: modelName(), title: `${state.error}\nClick to pause suggestions.` }
  return {
    icon: state.thinking ? 'loading codicon-modifier-spin' : 'sparkle',
    text: modelName(),
    title: `${modelName()}: suggestions as you type. Tab takes one, Ctrl+Right a word, Esc puts it away, Alt+\\ asks. Click to pause.`,
  }
})

function click() {
  const st = state.status
  if (st && !st.ready && !st.busy && st.provider !== 'off') void getReady()
  else if (st?.ready) toggle()
}
</script>

<template>
  <button class="ac" :title="view.title" @click="click">
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
