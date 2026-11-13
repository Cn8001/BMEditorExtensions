<script setup lang="ts">
import { computed } from 'vue'
import type { MenuItem } from '../bmext/bmeditor'
import { api, build, isGame, play, setTarget, stop, store, target } from './api'

// The top bar's Play, as Unity's: it plays the game where the menu beside
// it says, in its own window or in the ASCDII editor, and stops it.

defineProps<{ compact?: number }>()

const DropMenu = api.components.DropMenu

const label = computed(() => (target() === 'editor' ? 'ASCDII Editor' : 'Play'))

const items = computed<MenuItem[]>(() => [
  { id: 'h', heading: true, label: 'Play in' },
  { id: 'target:window', label: 'Window', detail: 'The game in its own window', icon: target() === 'window' ? 'check' : 'blank' },
  { id: 'target:editor', label: 'ASCDII Editor', detail: 'The engine\'s editor, on this project', icon: target() === 'editor' ? 'check' : 'blank' },
  { id: 's', separator: true },
  { id: 'build', label: 'Build', detail: 'ascdii build', icon: 'package', disabled: store.building },
  { id: 'output', label: 'Show Output', icon: 'output' },
  { id: 'settings', label: 'Settings', icon: 'settings-gear' },
])

function select(id: string) {
  const [what, value] = id.split(':')
  if (what === 'target') void setTarget(value as 'window' | 'editor')
  else if (what === 'build') void build()
  else if (what === 'output') api.ui.showOutput()
  else if (what === 'settings') api.ui.openSettings()
}
</script>

<template>
  <div v-if="isGame()" class="play">
    <button v-if="store.running" class="main stop" :title="`Stop ${store.running}`" @click="stop()">
      <i class="codicon codicon-debug-stop" />
      <span v-if="!compact">Stop</span>
    </button>
    <button v-else class="main" :title="target() === 'editor' ? 'Open the project in the ASCDII editor' : 'Play the game in its window'" @click="play()">
      <i class="codicon" :class="target() === 'editor' ? 'codicon-game' : 'codicon-play'" />
      <span v-if="!compact">{{ label }}</span>
    </button>
    <component :is="DropMenu" v-if="DropMenu" :items="items" align="right" :width="240" @select="select">
      <button class="more" title="Where Play opens the game, and Build">
        <i class="codicon codicon-chevron-down" />
      </button>
    </component>
  </div>
</template>

<style scoped>
.play {
  display: inline-flex;
  align-items: center;
}
button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 24px;
  padding: 0 8px;
  border-radius: 4px;
  color: var(--text-dim);
  white-space: nowrap;
}
button:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.main .codicon-play,
.main .codicon-game {
  color: var(--accent);
}
.stop .codicon {
  color: var(--error);
}
.more {
  padding: 0 4px;
}
</style>
