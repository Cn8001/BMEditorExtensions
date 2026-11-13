<script setup lang="ts">
import { computed } from 'vue'
import type { Tab } from '../bmext/bmeditor'
import { api, store, type Kind } from './api'

// The .ascdiiproj as Unity's Project Settings show a project: the game's
// name and version, the scene it starts in, the screen, mods and modules.
// Changes go into the file's tab, so undo and Ctrl+S work as on the text.

const props = defineProps<{ tab: Tab }>()

const parsed = computed<{ data: any; error: string }>(() => {
  void props.tab.version
  try {
    return { data: JSON.parse(api.session.textOf(props.tab.id).replace(/^﻿/, '')), error: '' }
  } catch (err) {
    return { data: null, error: (err as Error).message }
  }
})
const d = computed(() => parsed.value.data ?? {})
const display = computed(() => d.value.display ?? {})
const windowed = computed(() => display.value.mode === 'window')

function indentOf(src: string): string | number {
  const m = /\n([ \t]+)"/.exec(src)
  if (!m) return 2
  return m[1].startsWith('\t') ? '\t' : m[1].length
}

function write(mutate: (data: any) => void) {
  const state = api.session.stateOf(props.tab.id)
  if (!state) return
  const src = state.doc.toString()
  let data: any
  try {
    data = JSON.parse(src.replace(/^﻿/, ''))
  } catch {
    return
  }
  mutate(data)
  const nl = src.includes('\r\n') ? '\r\n' : '\n'
  let out = JSON.stringify(data, null, indentOf(src)).replace(/\n/g, nl)
  if (/\n$/.test(src)) out += nl
  if (src.startsWith('﻿')) out = '﻿' + out
  if (out !== src) api.session.dispatchTo(props.tab.id, { changes: { from: 0, to: state.doc.length, insert: out } })
}

// set writes a field of the manifest, or of one of its objects; an empty
// optional one is left out, as the engine writes it.
function set(path: string, value: unknown) {
  write((data) => {
    const keys = path.split('.')
    let at = data
    for (const k of keys.slice(0, -1)) at = at[k] ??= {}
    const last = keys[keys.length - 1]
    if (value === '' || value === false || value === null || (typeof value === 'number' && (Number.isNaN(value) || value === 0))) delete at[last]
    else at[last] = value
    if (keys.length > 1 && !Object.keys(data[keys[0]]).length) delete data[keys[0]]
  })
}

const text = (e: Event) => (e.target as HTMLInputElement).value.trim()
const num = (e: Event) => Number.parseInt((e.target as HTMLInputElement).value, 10)
const checked = (e: Event) => (e.target as HTMLInputElement).checked

const assets = (kind: Kind) => (store.index?.assets ?? []).filter((a) => a.kind === kind)
// pathOf shows a reference by its path, whether the file holds a GUID or a path.
const pathOf = (ref: string | undefined) => (ref ? (store.index?.assets.find((a) => a.guid === ref)?.path ?? ref) : '')

const installed = computed(() => /(\d+\.\d+\.\d+)/.exec(store.status?.version ?? '')?.[1] ?? '')
</script>

<template>
  <div class="settings">
    <p v-if="parsed.error" class="error">The file is not valid JSON: {{ parsed.error }}</p>
    <template v-else>
      <h4>General</h4>
      <label>Name<input :value="d.name ?? ''" spellcheck="false" @change="set('name', text($event))" /></label>
      <label>Version<input :value="d.version ?? ''" placeholder="0.0.1" spellcheck="false" @change="set('version', text($event))" /></label>
      <label
        >Start scene
        <select :value="pathOf(d.start)" @change="set('start', text($event))">
          <option v-if="!assets('scene').some((a) => a.path === pathOf(d.start))" :value="pathOf(d.start)">{{ pathOf(d.start) || '(none)' }}</option>
          <option v-for="a in assets('scene')" :key="a.path" :value="a.path">{{ a.path }}</option>
        </select>
      </label>
      <label
        >Icon
        <select :value="pathOf(d.icon)" @change="set('icon', text($event))">
          <option value="">(the default icon)</option>
          <option v-if="d.icon && !assets('texture').some((a) => a.path === pathOf(d.icon))" :value="pathOf(d.icon)">{{ pathOf(d.icon) }}</option>
          <option v-for="a in assets('texture')" :key="a.path" :value="a.path">{{ a.path }}</option>
        </select>
      </label>
      <p class="note">
        Saved with ASCDII Engine {{ d.engine || '?' }}
        <span v-if="installed && d.engine && installed !== d.engine" class="warn">; the installed engine is {{ installed }}</span>
      </p>

      <h4>Display</h4>
      <div class="seg" role="radiogroup" aria-label="Display">
        <button :class="{ on: !windowed }" @click="set('display.mode', '')">Terminal</button>
        <button :class="{ on: windowed }" @click="set('display.mode', 'window')">Window</button>
      </div>
      <div class="row3">
        <label>Columns<input type="number" min="20" max="400" :value="d.cols ?? ''" placeholder="120" @change="set('cols', num($event))" /></label>
        <label>Rows<input type="number" min="8" max="200" :value="d.rows ?? ''" placeholder="40" @change="set('rows', num($event))" /></label>
        <label>FPS<input type="number" min="1" max="240" :value="d.fps ?? ''" placeholder="30" @change="set('fps', num($event))" /></label>
      </div>
      <template v-if="windowed">
        <div class="row3">
          <label>Width<input type="number" min="0" :value="display.width ?? ''" placeholder="1280" @change="set('display.width', num($event))" /></label>
          <label>Height<input type="number" min="0" :value="display.height ?? ''" placeholder="720" @change="set('display.height', num($event))" /></label>
          <label title="Screen pixels per dot; empty fits about 480 dots across">Scale<input type="number" min="0" :value="display.scale ?? ''" placeholder="auto" @change="set('display.scale', num($event))" /></label>
        </div>
        <label class="check"><input type="checkbox" :checked="!!display.fullscreen" @change="set('display.fullscreen', checked($event))" />Start in full screen</label>
      </template>

      <h4>Mods</h4>
      <label class="check"><input type="checkbox" :checked="!!d.mods?.enabled" @change="set('mods.enabled', checked($event))" />Players' mods load</label>
      <label v-if="d.mods?.enabled"
        >API script
        <select :value="pathOf(d.mods?.api)" @change="set('mods.api', text($event))">
          <option value="">(none)</option>
          <option v-for="a in assets('script')" :key="a.path" :value="a.path">{{ a.path }}</option>
        </select>
      </label>

      <template v-if="d.modules?.length">
        <h4>Modules</h4>
        <div class="chips">
          <span v-for="m in d.modules" :key="m" class="chip">{{ m }}</span>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.settings {
  padding: 0 12px 10px;
  font-size: 12px;
}
h4 {
  margin: 10px 0 4px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-dim);
}
label {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-bottom: 6px;
  color: var(--text-dim);
}
input,
select {
  min-width: 0;
  padding: 3px 6px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
  outline: none;
  font-size: 12px;
  user-select: text;
}
input:focus,
select:focus {
  border-color: var(--accent);
}
.row3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
}
.check {
  flex-direction: row;
  align-items: center;
  gap: 6px;
  color: var(--text);
}
.seg {
  display: inline-flex;
  margin-bottom: 8px;
  border: 1px solid var(--border-strong);
  border-radius: 5px;
  overflow: hidden;
}
.seg button {
  padding: 3px 12px;
  color: var(--text-dim);
}
.seg button.on {
  background: var(--accent-dim);
  color: var(--accent);
}
.note {
  margin: 2px 0 0;
  color: var(--text-faint);
}
.warn {
  color: var(--warning, #e5c07b);
}
.error {
  color: var(--error);
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.chip {
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg-hover);
  color: var(--text-dim);
}
</style>
