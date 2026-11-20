<script setup lang="ts">
import { computed } from 'vue'
import { show, store } from './api'

// Mentor's button in the top bar: it opens Mentor wherever it lives, or
// closes its window, and its dot says whether the agent is connected.
defineProps<{ compact?: number }>()

const title = computed(() => {
  if (store.agent?.waiting === 'plan') return 'Mentor: the agent waits for you to approve its plan'
  if (store.agent?.waiting === 'checkpoint') return 'Mentor: the agent waits at a checkpoint'
  switch (store.agent?.level) {
    case 'enforced':
      return 'Mentor: the agent is connected, and checkpoints hold'
    case 'cooperative':
      return 'Mentor: the agent is connected without its hooks, so checkpoints do not hold'
    case 'off':
      return 'Mentor is off'
  }
  return 'Mentor: the agent is not connected'
})

function click() {
  if (store.place === 'window' && store.windowOpen) store.windowOpen = false
  else show()
}
</script>

<template>
  <button class="mb" :class="{ on: store.place === 'window' && store.windowOpen }" :title="title" @click="click">
    <i class="codicon codicon-mortar-board" />
    <span v-if="!compact">Mentor</span>
    <span v-if="store.agent?.waiting" class="wait">!</span>
    <span v-else class="dot" :class="store.agent?.level || 'observe'" />
  </button>
</template>

<style scoped>
.mb {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 4px;
  color: var(--text-dim);
  font-size: 12px;
  white-space: nowrap;
}
.mb:hover,
.mb.on {
  background: var(--bg-hover);
  color: var(--text);
}
.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--text-faint);
}
.dot.enforced {
  background: var(--success, #b8d27a);
}
.dot.cooperative {
  background: var(--warning);
}
.wait {
  padding: 0 5px;
  border-radius: 8px;
  background: var(--accent);
  color: var(--bg);
  font-size: 10px;
  font-weight: 600;
}
</style>
