<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, watch } from 'vue'
import { store } from './api'
import Sidebar from './Sidebar.vue'

// Mentor in a window of its own over the editor, when Open Mentor in says
// so: its title bar moves it, its corner sizes it, and it opens where it
// was left.
const key = 'mentor.window'

const connection = computed(() => {
  switch (store.agent?.level) {
    case 'enforced':
      return 'Agent connected: checkpoints hold'
    case 'cooperative':
      return 'Agent connected without its hooks: checkpoints do not hold'
    case 'off':
      return 'Mentor is off'
  }
  return 'Agent not connected'
})
const box = reactive({ x: 0, y: 0, w: 380, h: 560 })

function load() {
  try {
    const saved = JSON.parse(localStorage.getItem(key) || 'null')
    if (saved && typeof saved.x === 'number') Object.assign(box, saved)
    else Object.assign(box, { x: window.innerWidth - box.w - 24, y: 48 })
  } catch {
    Object.assign(box, { x: window.innerWidth - box.w - 24, y: 48 })
  }
  fit()
}

// fit keeps the title bar on screen, whatever size the editor's window has now.
function fit() {
  box.w = Math.max(260, Math.min(box.w, window.innerWidth - 16))
  box.h = Math.max(200, Math.min(box.h, window.innerHeight - 16))
  box.x = Math.max(0, Math.min(box.x, window.innerWidth - box.w))
  box.y = Math.max(0, Math.min(box.y, window.innerHeight - 32))
}

function save() {
  try {
    localStorage.setItem(key, JSON.stringify(box))
  } catch {
    // The place is only remembered where the webview keeps storage.
  }
}

function drag(e: PointerEvent, how: 'move' | 'size') {
  if ((e.target as HTMLElement).closest('button')) return
  e.preventDefault()
  const start = { px: e.clientX, py: e.clientY, ...box }
  const moveTo = (m: PointerEvent) => {
    const dx = m.clientX - start.px
    const dy = m.clientY - start.py
    if (how === 'move') Object.assign(box, { x: start.x + dx, y: start.y + dy })
    else Object.assign(box, { w: start.w + dx, h: start.h + dy })
    fit()
  }
  const end = () => {
    window.removeEventListener('pointermove', moveTo)
    window.removeEventListener('pointerup', end)
    save()
  }
  window.addEventListener('pointermove', moveTo)
  window.addEventListener('pointerup', end)
}

watch(
  () => store.windowOpen,
  (open) => open && load(),
  { immediate: true },
)
window.addEventListener('resize', fit)
onBeforeUnmount(() => window.removeEventListener('resize', fit))
</script>

<template>
  <div v-if="store.windowOpen && store.place === 'window'" class="mentor-window" :style="{ left: box.x + 'px', top: box.y + 'px', width: box.w + 'px', height: box.h + 'px' }">
    <div class="bar" @pointerdown="drag($event, 'move')">
      <i class="codicon codicon-mortar-board" />
      <span>Mentor</span>
      <span class="dot" :class="store.agent?.level || 'observe'" :title="connection" />
      <button title="Close" @click="store.windowOpen = false"><i class="codicon codicon-close" /></button>
    </div>
    <div class="content"><Sidebar /></div>
    <div class="grip" title="Size" @pointerdown="drag($event, 'size')" />
  </div>
</template>

<style scoped>
.mentor-window {
  position: fixed;
  z-index: 40;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  background: var(--bg-raised);
  color: var(--text);
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.35);
  overflow: hidden;
}
.bar {
  display: flex;
  flex: none;
  align-items: center;
  gap: 6px;
  padding: 4px 6px 4px 10px;
  border-bottom: 1px solid var(--border);
  font-size: 12px;
  color: var(--text-dim);
  cursor: move;
  user-select: none;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-faint);
}
.dot.enforced {
  background: var(--success, #b8d27a);
}
.dot.cooperative {
  background: var(--warning);
}
.bar button {
  margin-left: auto;
  color: var(--text-dim);
}
.bar button:hover {
  color: var(--text);
}
.content {
  flex: 1;
  min-height: 0;
}
.grip {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 14px;
  height: 14px;
  cursor: nwse-resize;
}
</style>
