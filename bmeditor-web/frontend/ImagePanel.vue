<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { openFile, openPath, pick, reload, viewer } from './api'

// The image viewer: what a click in the Explorer, the file picker, a drop
// or Preview SVG put in viewer.

const explorerDrag = 'application/x-bmeditor-entry'
const picker = ref<HTMLInputElement>()
const hover = ref(false)
// zoom is 0 to fit the panel, else the scale of the image's own size.
const zoom = ref(0)
const natural = ref({ width: 0, height: 0 })
// A light background for a dark image, such as an icon drawn in black.
const light = ref(false)

const steps = [0.1, 0.25, 0.5, 0.75, 1, 1.5, 2, 3, 4, 6, 8, 12, 16, 24, 32]

// The image's own size: an SVG's from its width, height or viewBox, else
// what the browser read.
const own = computed(() => (viewer.svg && viewer.width && viewer.height ? { width: viewer.width, height: viewer.height } : natural.value))

const facts = computed(() => {
  const b = viewer.bytes
  const bytes = b < 0 ? '' : b < 1024 ? `${b} B` : b < 1 << 20 ? `${(b / 1024).toFixed(1)} KB` : `${(b / (1 << 20)).toFixed(1)} MB`
  const { width, height } = own.value
  const size = width && height ? `${width} × ${height}` : ''
  const box = viewer.svg && viewer.viewBox ? `viewBox ${viewer.viewBox}` : ''
  return [viewer.svg ? 'SVG' : '', size, box, bytes, viewer.tabId ? 'live' : ''].filter(Boolean).join(' · ')
})

const imgStyle = computed(() => {
  if (!zoom.value) return {}
  const { width, height } = own.value
  if (!width || !height) return { transform: `scale(${zoom.value})`, transformOrigin: '0 0' }
  return { width: `${width * zoom.value}px`, height: `${height * zoom.value}px` }
})

watch(
  () => viewer.src,
  () => {
    if (!viewer.tabId) zoom.value = 0
  },
)
watch(
  () => viewer.name,
  () => {
    natural.value = { width: 0, height: 0 }
    zoom.value = 0
  },
)

function zoomBy(dir: 1 | -1) {
  const stage = document.querySelector('.web-image-stage') as HTMLElement | null
  const img = stage?.querySelector('img') as HTMLImageElement | null
  // From fit, start at the scale the image is drawn at.
  const now = zoom.value || (img && own.value.width ? img.clientWidth / own.value.width : 1)
  const next = dir > 0 ? steps.find((s) => s > now + 1e-6) : [...steps].reverse().find((s) => s < now - 1e-6)
  zoom.value = next ?? now
}

function onWheel(e: WheelEvent) {
  if (!e.ctrlKey || !viewer.src) return
  e.preventDefault()
  zoomBy(e.deltaY < 0 ? 1 : -1)
}

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
  natural.value = { width: img.naturalWidth, height: img.naturalHeight }
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
        <span class="facts" :title="facts">{{ facts }}</span>
        <span class="tools">
          <button title="Read it again" @click="reload()"><i class="codicon codicon-refresh" /></button>
          <button title="Zoom out (Ctrl+wheel)" @click="zoomBy(-1)"><i class="codicon codicon-zoom-out" /></button>
          <button class="pct" :class="{ on: !zoom }" title="Fit the panel" @click="zoom = 0">{{ zoom ? Math.round(zoom * 100) + '%' : 'Fit' }}</button>
          <button title="Zoom in (Ctrl+wheel)" @click="zoomBy(1)"><i class="codicon codicon-zoom-in" /></button>
          <button :class="{ on: zoom === 1 }" title="Actual size" @click="zoom = 1">1:1</button>
          <button :title="light ? 'Dark background' : 'Light background'" @click="light = !light"><i class="codicon" :class="light ? 'codicon-color-mode' : 'codicon-lightbulb'" /></button>
        </span>
      </template>
      <input ref="picker" type="file" accept="image/*,.svg" hidden @change="onPicked" />
    </div>
    <div class="stage web-image-stage" :class="{ zoomed: !!zoom, light, svg: viewer.svg }" @wheel="onWheel">
      <p v-if="viewer.failed" class="empty">{{ viewer.name }} could not be shown: it is gone, or {{ viewer.svg ? 'not a well-formed SVG' : 'not an image' }}.</p>
      <img
        v-else-if="viewer.src"
        :src="viewer.src"
        :alt="viewer.name"
        :style="imgStyle"
        @load="onLoad"
        @error="viewer.failed = true"
        @click="zoom = zoom ? 0 : 1"
      />
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
.tools {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 2px;
}
.tools button {
  padding: 3px 6px;
}
.tools .pct {
  min-width: 46px;
  justify-content: center;
  font-variant-numeric: tabular-nums;
}
.tools button.on {
  color: var(--text);
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
.stage.light {
  background: repeating-conic-gradient(#ffffff 0% 25%, #e6e6e6 0% 50%) 0 0 / 16px 16px;
}
.stage img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  cursor: zoom-in;
}
/* A vector image fills the panel when fitted, as large as it fits. */
.stage.svg:not(.zoomed) img {
  width: 100%;
  height: 100%;
}
.stage.zoomed {
  place-items: start;
}
.stage.zoomed img {
  max-width: none;
  max-height: none;
  cursor: zoom-out;
}
.empty {
  color: var(--text-dim);
  font-size: 13px;
}
</style>
