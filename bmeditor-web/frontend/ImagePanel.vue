<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { openFile, openPath, pick, viewer } from './api'

// The image viewer: what a click in the Explorer, the file picker, a drop
// or Preview SVG put in viewer.

const explorerDrag = 'application/x-bmeditor-entry'
const picker = ref<HTMLInputElement>()
const actual = ref(false)
const hover = ref(false)
const size = ref('')

const facts = computed(() => {
  const b = viewer.bytes
  const bytes = b < 0 ? '' : b < 1024 ? `${b} B` : b < 1 << 20 ? `${(b / 1024).toFixed(1)} KB` : `${(b / (1 << 20)).toFixed(1)} MB`
  return [size.value, bytes, viewer.tabId ? 'live' : ''].filter(Boolean).join(' · ')
})

watch(
  () => viewer.src,
  () => {
    size.value = ''
    if (!viewer.tabId) actual.value = false
  },
)

function onPicked(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (f) openFile(f)
  input.value = ''
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  hover.value = true
  // The Explorer allows a move only; anything else would refuse the drop.
  if (e.dataTransfer) e.dataTransfer.dropEffect = e.dataTransfer.types.includes(explorerDrag) ? 'move' : 'copy'
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  hover.value = false
  const path = e.dataTransfer?.getData(explorerDrag)
  if (path) return openPath(path)
  const f = e.dataTransfer?.files?.[0]
  if (f) openFile(f)
}

function onLoad(e: Event) {
  const img = e.target as HTMLImageElement
  size.value = img.naturalWidth ? `${img.naturalWidth} × ${img.naturalHeight}` : ''
}

function answer() {
  if (pick.asked === pick.answered) return
  pick.answered = pick.asked
  void nextTick(() => picker.value?.click())
}

watch(() => pick.asked, answer)
onMounted(answer)
</script>

<template>
  <div class="viewer" :class="{ hover }" @dragover="onDragOver" @dragleave="hover = false" @drop="onDrop">
    <div class="bar">
      <button @click="picker?.click()"><i class="codicon codicon-folder-opened" /> Choose Image…</button>
      <template v-if="viewer.src">
        <span class="name" :title="viewer.name">{{ viewer.name }}</span>
        <span class="facts">{{ facts }}</span>
        <button class="fit" @click="actual = !actual">{{ actual ? 'Fit' : 'Actual size' }}</button>
      </template>
      <input ref="picker" type="file" accept="image/*" hidden @change="onPicked" />
    </div>
    <div class="stage" :class="{ actual }">
      <p v-if="viewer.failed" class="empty">{{ viewer.name }} could not be shown.</p>
      <img v-else-if="viewer.src" :src="viewer.src" :alt="viewer.name" @load="onLoad" @error="viewer.failed = true" @click="actual = !actual" />
      <p v-else class="empty">Click an image in the Explorer, choose one, or drag one here.</p>
    </div>
  </div>
</template>

<style scoped>
.viewer {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.viewer.hover {
  box-shadow: inset 0 0 0 1px var(--accent);
}
.bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  font-size: 12px;
}
.bar button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 4px;
  color: var(--text-dim);
}
.bar button:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.name {
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
.facts {
  color: var(--text-faint);
  white-space: nowrap;
}
.fit {
  margin-left: auto;
}
.stage {
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: grid;
  place-items: center;
  padding: 12px;
  /* A checkerboard, so a transparent image shows where it is clear. */
  background: repeating-conic-gradient(#24211e 0% 25%, #1b1a17 0% 50%) 0 0 / 16px 16px;
}
.stage img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  cursor: zoom-in;
}
.stage.actual {
  place-items: start;
}
.stage.actual img {
  max-width: none;
  max-height: none;
  cursor: zoom-out;
}
.empty {
  color: var(--text-dim);
  font-size: 13px;
}
</style>
