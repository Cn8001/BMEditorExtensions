<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowReactive, watch } from 'vue'
import type { Tab } from '../bmext/bmeditor'
import { api, editsOf, fromBase64, set, undo } from './state'

// A file that is not text, as Bless shows one: offsets, sixteen bytes a row
// in hex, the same bytes as characters, and what the selected bytes read
// as. Only the rows on screen are read, a page at a time, so a file of any
// size opens at once. Typing overwrites the selected byte, two hex digits
// in the hex column or a character in the other; the file keeps its size.

const props = defineProps<{ tab: Tab }>()
const edits = editsOf(props.tab)
const column = ref<'hex' | 'text'>('hex')
const high = ref<number | null>(null) // the first hex digit typed into the selected byte

const WIDTH = 16
const LINE = 20
const PAGE = 64 * 1024
// Past this a scrollbar of one pixel a row would be taller than the
// browser draws; the bar is scaled down instead.
const MAX_SCROLL = 8_000_000

const size = ref(-1)
const error = ref('')
const pages = shallowReactive(new Map<number, Uint8Array>())
const asked = new Set<number>()
const scroller = ref<HTMLDivElement>()
const scrollTop = ref(0)
const viewHeight = ref(400)
const selected = ref(-1)
const hovered = ref(-1)
const target = ref('')

const rows = computed(() => Math.max(1, Math.ceil(Math.max(0, size.value) / WIDTH)))
const shown = computed(() => Math.ceil(viewHeight.value / LINE) + 1)
const fullHeight = computed(() => rows.value * LINE)
const barHeight = computed(() => Math.min(fullHeight.value, MAX_SCROLL))
const firstRow = computed(() => {
  const room = barHeight.value - viewHeight.value
  if (room <= 0) return 0
  const fraction = Math.min(1, scrollTop.value / room)
  return Math.max(0, Math.min(rows.value - shown.value + 1, Math.round(fraction * Math.max(0, rows.value - shown.value + 1))))
})

async function load(page: number) {
  if (asked.has(page)) return
  asked.add(page)
  try {
    const b = await api.call<{ data: string | null; size: number }>('read', { path: props.tab.path, offset: page * PAGE, size: PAGE })
    size.value = b.size
    pages.set(page, fromBase64(b.data))
    error.value = ''
  } catch (err) {
    asked.delete(page)
    error.value = api.ui.failureOf(err).message
  }
}

// byteAt is the byte as edited: a change not saved yet, else the file's.
function byteAt(offset: number): number | undefined {
  if (offset < 0 || offset >= size.value) return undefined
  const changed = edits.changed.get(offset)
  if (changed !== undefined) return changed
  const page = pages.get(Math.floor(offset / PAGE))
  if (!page) {
    void load(Math.floor(offset / PAGE))
    return undefined
  }
  return page[offset % PAGE]
}

const hex = (n: number, width = 2) => n.toString(16).toUpperCase().padStart(width, '0')
const printable = (b: number) => (b >= 0x20 && b < 0x7f ? String.fromCharCode(b) : '.')

const visible = computed(() => {
  void pages.size // read again as pages come in, and as bytes change
  void edits.changed.size
  void edits.undo.length
  const out: { offset: number; bytes: (number | undefined)[] }[] = []
  for (let r = firstRow.value; r < Math.min(rows.value, firstRow.value + shown.value); r++) {
    const offset = r * WIDTH
    const bytes: (number | undefined)[] = []
    for (let i = 0; i < WIDTH && offset + i < size.value; i++) bytes.push(byteAt(offset + i))
    out.push({ offset, bytes })
  }
  return out
})

const offsetWidth = computed(() => Math.max(8, hex(Math.max(0, size.value)).length))

// What the selected bytes read as, Bless's conversion table.
const reading = computed(() => {
  const at = selected.value
  if (at < 0) return null
  const raw: number[] = []
  for (let i = 0; i < 8; i++) {
    const b = byteAt(at + i)
    if (b === undefined) break
    raw.push(b)
  }
  if (!raw.length) return null
  const view = new DataView(new Uint8Array([...raw, 0, 0, 0, 0, 0, 0, 0, 0].slice(0, 8)).buffer)
  const has = (n: number) => raw.length >= n
  const b = raw[0]
  return [
    ['Offset', `0x${hex(at, offsetWidth.value)} (${at})`],
    ['Byte', `0x${hex(b)} · ${b} · ${b.toString(2).padStart(8, '0')} · '${printable(b)}'`],
    ['Int8 / UInt8', `${view.getInt8(0)} / ${b}`],
    ...(has(2) ? [['Int16 LE / BE', `${view.getInt16(0, true)} / ${view.getInt16(0, false)}`], ['UInt16 LE / BE', `${view.getUint16(0, true)} / ${view.getUint16(0, false)}`]] : []),
    ...(has(4) ? [['Int32 LE / BE', `${view.getInt32(0, true)} / ${view.getInt32(0, false)}`], ['UInt32 LE / BE', `${view.getUint32(0, true)} / ${view.getUint32(0, false)}`], ['Float32 LE', `${view.getFloat32(0, true)}`]] : []),
    ...(has(8) ? [['Int64 LE', `${view.getBigInt64(0, true)}`], ['Float64 LE', `${view.getFloat64(0, true)}`]] : []),
  ] as [string, string][]
})

function onScroll() {
  scrollTop.value = scroller.value?.scrollTop ?? 0
}

// reveal scrolls so a byte's row is in view.
async function reveal(offset: number) {
  const row = Math.floor(offset / WIDTH)
  if (row >= firstRow.value && row < firstRow.value + shown.value - 2) return
  const el = scroller.value
  if (!el) return
  const room = barHeight.value - viewHeight.value
  const span = Math.max(1, rows.value - shown.value + 1)
  el.scrollTop = room > 0 ? (Math.max(0, row - 2) / span) * room : 0
  await nextTick()
  onScroll()
}

function select(offset: number, where?: 'hex' | 'text') {
  if (size.value <= 0) return
  selected.value = Math.max(0, Math.min(size.value - 1, offset))
  if (where) column.value = where
  high.value = null
  void reveal(selected.value)
}

// type overwrites the selected byte: two hex digits in the hex column, a
// character in the other, then moves on.
function type(key: string): boolean {
  const at = selected.value
  const current = byteAt(at)
  if (at < 0 || current === undefined) return false
  if (column.value === 'text') {
    const code = key.charCodeAt(0)
    if (key.length !== 1 || code > 0xff) return false
    set(props.tab, at, code)
    select(at + 1)
    return true
  }
  if (!/^[0-9a-f]$/i.test(key)) return false
  const digit = parseInt(key, 16)
  if (high.value === null) {
    high.value = digit
    set(props.tab, at, (digit << 4) | (current & 0x0f))
  } else {
    set(props.tab, at, (high.value << 4) | digit)
    select(at + 1)
  }
  return true
}

function go() {
  const t = target.value.trim().toLowerCase()
  const n = t.startsWith('0x') ? parseInt(t.slice(2), 16) : /^[0-9a-f]+h$/.test(t) ? parseInt(t.slice(0, -1), 16) : parseInt(t, 10)
  if (!Number.isNaN(n)) select(n)
}

function onKey(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z') {
    const at = undo(props.tab)
    if (at !== undefined) select(at)
    e.preventDefault()
    return
  }
  if (e.key === 'Tab') {
    column.value = column.value === 'hex' ? 'text' : 'hex'
    high.value = null
    e.preventDefault()
    return
  }
  if (!e.ctrlKey && !e.metaKey && !e.altKey && type(e.key)) {
    e.preventDefault()
    return
  }
  const step: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -WIDTH, ArrowDown: WIDTH, PageUp: -WIDTH * (shown.value - 2), PageDown: WIDTH * (shown.value - 2) }
  if (e.key in step) select(Math.max(0, selected.value) + step[e.key])
  else if (e.key === 'Home') select(e.ctrlKey ? 0 : selected.value - (selected.value % WIDTH))
  else if (e.key === 'End') select(e.ctrlKey ? size.value - 1 : selected.value - (selected.value % WIDTH) + WIDTH - 1)
  else return
  e.preventDefault()
}

let ro: ResizeObserver | undefined
onMounted(() => {
  void load(0)
  ro = new ResizeObserver(([e]) => (viewHeight.value = e.contentRect.height))
  if (scroller.value) ro.observe(scroller.value)
})
onBeforeUnmount(() => ro?.disconnect())

// The file changed on disk: read what is on screen again.
watch(
  () => props.tab.version,
  () => {
    pages.clear()
    asked.clear()
    void load(Math.floor((firstRow.value * WIDTH) / PAGE))
  },
)

function human(n: number) {
  if (n < 1024) return `${n} bytes`
  const units = ['KB', 'MB', 'GB', 'TB']
  let v = n / 1024
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  return `${v.toFixed(1)} ${units[u]} (${n.toLocaleString()} bytes)`
}
</script>

<template>
  <div class="hex-view">
    <div class="bar">
      <i class="codicon codicon-file-binary" />
      <span class="what">{{ size >= 0 ? human(size) : 'Reading…' }}</span>
      <span v-if="edits.changed.size" class="changed">{{ edits.changed.size }} byte{{ edits.changed.size === 1 ? '' : 's' }} changed · Ctrl+S saves, Ctrl+Z undoes</span>
      <span v-else class="hint">Type over a byte: hex digits, or a character in the right column (Tab switches)</span>
      <div class="spacer" />
      <form class="goto" @submit.prevent="go">
        <input v-model="target" placeholder="Go to offset (0x1F0 or 496)" spellcheck="false" />
      </form>
    </div>
    <p v-if="error" class="error">{{ error }}</p>
    <div ref="scroller" class="scroller" tabindex="0" @scroll="onScroll" @keydown="onKey" @mouseleave="hovered = -1">
      <div :style="{ height: barHeight + 'px' }">
        <div class="rows" :style="{ transform: `translateY(${scrollTop}px)` }">
          <div v-for="r in visible" :key="r.offset" class="row">
            <span class="offset">{{ hex(r.offset, offsetWidth) }}</span>
            <span class="bytes">
              <span
                v-for="(b, i) in r.bytes"
                :key="i"
                class="b"
                :class="{ sel: r.offset + i === selected, active: column === 'hex', hov: r.offset + i === hovered, gap: i === 8, zero: b === 0, edited: edits.changed.has(r.offset + i) }"
                @mousedown.prevent="select(r.offset + i, 'hex')"
                @mouseenter="hovered = r.offset + i"
                >{{ b === undefined ? '··' : hex(b) }}</span
              >
            </span>
            <span class="ascii">
              <span
                v-for="(b, i) in r.bytes"
                :key="i"
                class="c"
                :class="{ sel: r.offset + i === selected, active: column === 'text', hov: r.offset + i === hovered, dot: b === undefined || b < 0x20 || b >= 0x7f, edited: edits.changed.has(r.offset + i) }"
                @mousedown.prevent="select(r.offset + i, 'text')"
                @mouseenter="hovered = r.offset + i"
                >{{ b === undefined ? ' ' : printable(b) }}</span
              >
            </span>
          </div>
        </div>
      </div>
    </div>
    <dl v-if="reading" class="reading">
      <template v-for="[k, v] in reading" :key="k">
        <dt>{{ k }}</dt>
        <dd>{{ v }}</dd>
      </template>
    </dl>
  </div>
</template>

<style scoped>
.hex-view {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg);
  font-family: var(--font-mono);
  font-size: 13px;
}
.bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border);
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--text-dim);
}
.spacer {
  flex: 1;
}
.goto input {
  width: 220px;
  padding: 3px 8px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg-side);
  font-family: var(--font-mono);
  outline: none;
  user-select: text;
}
.goto input:focus {
  border-color: var(--accent);
}
.error {
  margin: 0;
  padding: 6px 12px;
  color: var(--error);
  font-family: var(--font-ui);
  font-size: 12px;
}
.scroller {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: auto;
  outline: none;
}
.rows {
  position: absolute;
  left: 0;
  top: 0;
  padding-left: 12px;
  will-change: transform;
}
.row {
  display: flex;
  gap: 18px;
  height: 20px;
  line-height: 20px;
  white-space: pre;
}
.offset {
  color: var(--text-faint);
}
.bytes {
  display: flex;
}
.b {
  width: 2.6ch;
  text-align: center;
  color: var(--text);
  cursor: default;
}
.b.gap {
  margin-left: 1ch;
}
.b.zero {
  color: var(--text-faint);
}
.ascii {
  display: flex;
  padding-left: 10px;
  border-left: 1px solid var(--border);
}
.c {
  width: 1ch;
  text-align: center;
  color: var(--text-dim);
  cursor: default;
}
.c.dot {
  color: var(--text-faint);
}
.hov {
  background: var(--bg-hover);
}
.edited {
  color: var(--warning, #e5c07b);
}
.sel {
  background: var(--accent-dim);
  border-radius: 2px;
}
/* The column typing goes to shows the selection strongly. */
.sel.active {
  outline: 1px solid var(--accent);
  color: var(--accent);
}
.changed {
  color: var(--warning, #e5c07b);
}
.hint {
  color: var(--text-faint);
}
.reading {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, max-content) minmax(170px, 1fr));
  gap: 4px 14px;
  margin: 0;
  padding: 8px 12px;
  border-top: 1px solid var(--border);
  background: var(--bg-side);
  font-size: 12px;
}
.reading dt {
  color: var(--text-faint);
  font-family: var(--font-ui);
}
.reading dd {
  margin: 0;
  color: var(--text);
  user-select: text;
}
</style>
