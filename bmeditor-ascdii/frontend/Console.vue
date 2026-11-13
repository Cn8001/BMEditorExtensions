<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { absolute, api, clearConsole, store, type Level, type LogLine } from './api'

// The game's Console, as Unity's: what the engine logged, by level, with
// counts that filter, Collapse for repeats, Clear on Play, and a click on
// a script error that opens the script at its line.

const prefsKey = 'bmeditor-ascdii.console'
const prefs = reactive({ collapse: false, clearOnPlay: true, info: true, warn: true, error: true })
try {
  Object.assign(prefs, JSON.parse(localStorage.getItem(prefsKey) ?? '{}'))
} catch {
  // the defaults then
}
watch(prefs, () => {
  try {
    localStorage.setItem(prefsKey, JSON.stringify(prefs))
  } catch {
    // only for this session
  }
})

const search = ref('')
const list = ref<HTMLElement>()
const selected = ref(0)

const counts = computed(() => {
  const c = { info: 0, warn: 0, error: 0 }
  for (const l of store.console) if (l.level !== 'out') c[l.level]++
  return c
})

const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  const keep = (l: LogLine) => (l.level === 'out' ? prefs.info : prefs[l.level]) && (!q || l.text.toLowerCase().includes(q) || l.source.toLowerCase().includes(q))
  const lines = store.console.filter(keep)
  if (!prefs.collapse) return lines.map((l) => ({ l, n: 1 }))
  const by = new Map<string, { l: LogLine; n: number }>()
  for (const l of lines) {
    const k = l.level + '\n' + l.source + '\n' + l.text
    const e = by.get(k)
    if (e) e.n++
    else by.set(k, { l, n: 1 })
  }
  return [...by.values()]
})

// Follow new lines while the list is at its end, as a console does.
let atEnd = true
function onScroll() {
  const el = list.value
  if (el) atEnd = el.scrollTop + el.clientHeight >= el.scrollHeight - 8
}
watch(
  () => store.console.length,
  async () => {
    if (!atEnd) return
    await nextTick()
    if (list.value) list.value.scrollTop = list.value.scrollHeight
  },
)

const icons: Record<Level, string> = { info: 'info', warn: 'warning', error: 'error', out: 'terminal' }

function open(l: LogLine) {
  selected.value = l.id
  if (l.file) void api.session.openFile(absolute(l.file), { line: l.line ?? 1, col: 1 })
}

function toggle(level: 'info' | 'warn' | 'error') {
  prefs[level] = !prefs[level]
}
</script>

<template>
  <div class="console">
    <div class="bar">
      <button title="Clear the Console" @click="clearConsole"><i class="codicon codicon-clear-all" />Clear</button>
      <button :class="{ on: prefs.collapse }" title="One line for messages that repeat, with how many" @click="prefs.collapse = !prefs.collapse">Collapse</button>
      <button :class="{ on: prefs.clearOnPlay }" title="Clear the Console when Play starts the game" @click="prefs.clearOnPlay = !prefs.clearOnPlay">Clear on Play</button>
      <input v-model="search" class="search" placeholder="Search" spellcheck="false" />
      <span class="grow" />
      <button class="level" :class="{ on: prefs.info }" title="Messages" @click="toggle('info')"><i class="codicon codicon-info" />{{ counts.info }}</button>
      <button class="level warn" :class="{ on: prefs.warn }" title="Warnings" @click="toggle('warn')"><i class="codicon codicon-warning" />{{ counts.warn }}</button>
      <button class="level error" :class="{ on: prefs.error }" title="Errors" @click="toggle('error')"><i class="codicon codicon-error" />{{ counts.error }}</button>
    </div>
    <div ref="list" class="lines" @scroll="onScroll">
      <p v-if="!store.console.length" class="empty">{{ store.running ? 'Nothing logged yet.' : 'Play the game: what it logs shows here.' }}</p>
      <div
        v-for="{ l, n } in shown"
        :key="l.id"
        class="line"
        :class="[l.level, { selected: selected === l.id, link: !!l.file }]"
        :title="l.file ? `${l.file}:${l.line}: click to open` : ''"
        @click="open(l)"
      >
        <i class="codicon" :class="'codicon-' + icons[l.level]" />
        <span class="time">{{ l.time }}</span>
        <span v-if="l.source" class="source">{{ l.source }}</span>
        <span class="text">{{ l.text }}</span>
        <span v-if="n > 1" class="count">{{ n }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.console {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  font-size: 12px;
}
.bar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border);
}
.bar button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 22px;
  padding: 0 8px;
  border-radius: 4px;
  color: var(--text-dim);
}
.bar button:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.bar button.on {
  background: var(--accent-dim);
  color: var(--text);
}
.level:not(.on) {
  opacity: 0.5;
}
.level.warn .codicon {
  color: var(--warning, #e5c07b);
}
.level.error .codicon {
  color: var(--error);
}
.search {
  width: 180px;
  margin-left: 6px;
  padding: 2px 8px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg);
  outline: none;
  font-size: 12px;
  user-select: text;
}
.search:focus {
  border-color: var(--accent);
}
.grow {
  flex: 1;
}
.lines {
  flex: 1;
  min-height: 0;
  overflow: auto;
  font-family: var(--font-mono);
}
.line {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 2px 10px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
  color: var(--text);
  user-select: text;
}
.line:hover {
  background: var(--bg-hover);
}
.line.selected {
  background: var(--bg-active);
}
.line.link {
  cursor: pointer;
}
.line .codicon {
  flex: none;
  align-self: center;
  color: var(--text-dim);
}
.line.warn .codicon {
  color: var(--warning, #e5c07b);
}
.line.error .codicon {
  color: var(--error);
}
.line.error .text {
  color: color-mix(in srgb, var(--error) 70%, var(--text));
}
.line.out {
  color: var(--text-dim);
}
.time,
.source {
  flex: none;
  color: var(--text-faint);
}
.source {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.text {
  flex: 1;
  white-space: pre-wrap;
  word-break: break-word;
}
.count {
  flex: none;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--bg-active);
  color: var(--text-dim);
}
.empty {
  padding: 10px 12px;
  font-family: var(--font-ui);
  color: var(--text-faint);
}
</style>
