<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { loadAssemblies, openPath, rel, store, type Assembly } from './api'

// The assembly definitions and their references, in layers: an assembly
// sits above those it references. A click focuses one: what it needs and
// what needs it stay lit. Loops are red; references to nothing are amber.

const showExternal = ref(false)
const showTests = ref(true)
const find = ref('')
const focus = ref(-1)

interface GNode {
  i: number // index in store.assemblies
  a: Assembly
  x: number
  y: number
  w: number
  layer: number
  missing: number
}

const H = 22
const ROW = 70
const GAP = 14
// A layer wider than this wraps onto more rows, SUBROW apart: a project
// with dozens of assemblies that reference nothing would otherwise be one
// line, shrunk to unreadable to fit the panel.
const MAX_ROW = 1200
const SUBROW = 34

const graph = computed(() => {
  const as = store.assemblies
  if (!as) return { nodes: [] as GNode[], edges: [] as [number, number][], cyclic: new Set<string>() }
  const byName = new Map(as.assemblies.map((a, i) => [a.name, i]))
  const keep = new Set<number>()
  as.assemblies.forEach((a, i) => {
    if (!a.external && (showTests.value || !a.test)) keep.add(i)
  })
  const edges: [number, number][] = as.edges.filter(([f, t]) => keep.has(f) && keep.has(t))
  if (showExternal.value) {
    for (const i of [...keep]) {
      for (const r of as.assemblies[i].references) {
        const t = r.status === 'external' && r.name ? byName.get(r.name) : undefined
        if (t !== undefined && as.assemblies[t].external) {
          keep.add(t)
          edges.push([i, t])
        }
      }
    }
  }
  const out = new Map<number, number[]>()
  for (const [f, t] of edges) out.set(f, [...(out.get(f) ?? []), t])
  // layer: 0 for an assembly that references none here, else one above
  // the highest it references; a loop is cut where it closes.
  const layer = new Map<number, number>()
  const onPath = new Set<number>()
  const depth = (i: number): number => {
    if (layer.has(i)) return layer.get(i)!
    if (onPath.has(i)) return 0
    onPath.add(i)
    let d = 0
    for (const t of out.get(i) ?? []) d = Math.max(d, depth(t) + 1)
    onPath.delete(i)
    layer.set(i, d)
    return d
  }
  for (const i of keep) depth(i)
  const top = Math.max(0, ...layer.values())
  const layers: number[][] = Array.from({ length: top + 1 }, () => [])
  for (const i of keep) layers[top - layer.get(i)!].push(i)
  for (const l of layers) l.sort((a, b) => as.assemblies[a].name.localeCompare(as.assemblies[b].name))

  const nodes = new Map<number, GNode>()
  const width = (a: Assembly) => Math.max(70, a.name.length * 6.4 + 22)
  const place = () => {
    let top = 0
    layers.forEach((l, layerIndex) => {
      const rows: number[][] = [[]]
      let used = -GAP
      for (const i of l) {
        const w = width(as.assemblies[i])
        if (rows[rows.length - 1].length && used + GAP + w > MAX_ROW) {
          rows.push([])
          used = -GAP
        }
        rows[rows.length - 1].push(i)
        used += GAP + w
      }
      rows.forEach((row, r) => {
        const total = row.reduce((s, i) => s + width(as.assemblies[i]) + GAP, -GAP)
        let x = -total / 2
        for (const i of row) {
          const a = as.assemblies[i]
          const w = width(a)
          nodes.set(i, { i, a, x, y: top + r * SUBROW, w, layer: layerIndex, missing: a.references.filter((ref) => ref.status === 'unresolved').length })
          x += w + GAP
        }
      })
      top += (rows.length - 1) * SUBROW + ROW
    })
  }
  place()
  // Order each layer by where what it references sits, twice down.
  const ins = new Map<number, number[]>()
  for (const [f, t] of edges) ins.set(t, [...(ins.get(t) ?? []), f])
  for (let sweep = 0; sweep < 2; sweep++) {
    for (let y = 1; y < layers.length; y++) {
      const center = (i: number) => {
        const ps = (ins.get(i) ?? []).map((p) => nodes.get(p)!).filter((p) => p && p.layer < y)
        return ps.length ? ps.reduce((s, p) => s + p.x + p.w / 2, 0) / ps.length : nodes.get(i)!.x
      }
      const c = new Map(layers[y].map((i) => [i, center(i)]))
      layers[y].sort((a, b) => c.get(a)! - c.get(b)!)
    }
    place()
  }
  const cyclic = new Set((as.cycles ?? []).flat())
  return { nodes: [...nodes.values()], edges, cyclic }
})

const nodeAt = computed(() => new Map(graph.value.nodes.map((n) => [n.i, n])))

// What the focused assembly needs and what needs it, all the way.
const lit = computed(() => {
  if (focus.value < 0) return null
  const out = new Map<number, number[]>()
  const ins = new Map<number, number[]>()
  for (const [f, t] of graph.value.edges) {
    out.set(f, [...(out.get(f) ?? []), t])
    ins.set(t, [...(ins.get(t) ?? []), f])
  }
  const reach = (start: number, next: Map<number, number[]>) => {
    const seen = new Set<number>([start])
    const todo = [start]
    while (todo.length) for (const n of next.get(todo.pop()!) ?? []) if (!seen.has(n)) seen.add(n), todo.push(n)
    return seen
  }
  const needs = reach(focus.value, out)
  const neededBy = reach(focus.value, ins)
  return { needs, neededBy, all: new Set([...needs, ...neededBy]) }
})

function edgePath(f: number, t: number): string {
  const a = nodeAt.value.get(f)!
  const b = nodeAt.value.get(t)!
  const x1 = a.x + a.w / 2
  const y1 = a.y + H
  const x2 = b.x + b.w / 2
  const y2 = b.y
  const my = (y1 + y2) / 2
  return `M${x1},${y1} C${x1},${my} ${x2},${my} ${x2},${y2}`
}

function edgeClass(f: number, t: number) {
  const g = graph.value
  const loop = g.cyclic.has(store.assemblies!.assemblies[f].name) && g.cyclic.has(store.assemblies!.assemblies[t].name)
  const l = lit.value
  return { loop, dim: l && !(l.all.has(f) && l.all.has(t)), lit: l && l.all.has(f) && l.all.has(t) }
}

function nodeClass(n: GNode) {
  const l = lit.value
  return {
    focus: focus.value === n.i,
    dim: l && !l.all.has(n.i),
    editor: n.a.editor,
    test: n.a.test,
    external: n.a.external,
    loop: graph.value.cyclic.has(n.a.name),
  }
}

// Pan and zoom.
const box = ref<HTMLDivElement>()
const view = reactive({ x: 0, y: 0, k: 1, w: 0, h: 0 })
let ro: ResizeObserver | undefined

function fit() {
  const ns = graph.value.nodes
  if (!ns.length || !view.w) return
  const minX = Math.min(...ns.map((n) => n.x)) - 20
  const maxX = Math.max(...ns.map((n) => n.x + n.w)) + 20
  const minY = Math.min(...ns.map((n) => n.y)) - 20
  const maxY = Math.max(...ns.map((n) => n.y + H)) + 20
  // Never smaller than readable: a larger graph starts at its top, and is
  // panned and zoomed from there.
  const fits = Math.min(view.w / (maxX - minX), view.h / (maxY - minY))
  view.k = Math.min(1.2, Math.max(0.6, fits))
  view.x = view.w / 2 - ((minX + maxX) / 2) * view.k
  view.y = fits < 0.6 ? -minY * view.k : view.h / 2 - ((minY + maxY) / 2) * view.k
}

onMounted(() => {
  if (!store.assemblies && !store.busy.assemblies) void loadAssemblies()
  ro = new ResizeObserver(([e]) => {
    const first = !view.w
    view.w = e.contentRect.width
    view.h = e.contentRect.height
    if (first) fit()
  })
  if (box.value) ro.observe(box.value)
})
onBeforeUnmount(() => ro?.disconnect())
watch(() => graph.value.nodes.length, fit)

let drag: { x: number; y: number; vx: number; vy: number } | null = null
function down(e: PointerEvent) {
  if ((e.target as Element).closest('.node')) return
  drag = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y }
  ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
}
function move(e: PointerEvent) {
  if (!drag) return
  view.x = drag.vx + e.clientX - drag.x
  view.y = drag.vy + e.clientY - drag.y
}
function up(e: PointerEvent) {
  if (drag && Math.abs(e.clientX - drag.x) + Math.abs(e.clientY - drag.y) < 3) focus.value = -1
  drag = null
}
function wheel(e: WheelEvent) {
  const r = box.value!.getBoundingClientRect()
  const px = e.clientX - r.left
  const py = e.clientY - r.top
  const k = Math.min(3, Math.max(0.1, view.k * Math.exp(-e.deltaY * 0.0015)))
  view.x = px - ((px - view.x) * k) / view.k
  view.y = py - ((py - view.y) * k) / view.k
  view.k = k
}

function center(n: GNode) {
  view.x = view.w / 2 - (n.x + n.w / 2) * view.k
  view.y = view.h / 2 - (n.y + H / 2) * view.k
}

watch(find, (q) => {
  const s = q.trim().toLowerCase()
  if (!s) return
  const n = graph.value.nodes.find((n) => n.a.name.toLowerCase().includes(s))
  if (n) {
    focus.value = n.i
    center(n)
  }
})

const focused = computed(() => (focus.value >= 0 ? store.assemblies?.assemblies[focus.value] : undefined))
const neededByCount = computed(() => (lit.value ? lit.value.neededBy.size - 1 : 0))

function refLabel(r: { raw: string; name?: string; status: string }) {
  if (r.status === 'unresolved') return r.raw.startsWith('GUID:') ? `${r.raw} — no assembly here has it` : `${r.raw} — not found`
  return r.name + (r.status === 'external' ? ' (package or Unity)' : '')
}
</script>

<template>
  <div class="graph">
    <div class="bar">
      <input v-model="find" placeholder="Find assembly" spellcheck="false" />
      <label><input v-model="showTests" type="checkbox" /> Tests</label>
      <label><input v-model="showExternal" type="checkbox" /> Packages</label>
      <span v-if="store.assemblies" class="facts">
        {{ graph.nodes.length }} assemblies · {{ graph.edges.length }} references
        <span v-if="store.assemblies.cycles.length" class="bad">· {{ store.assemblies.cycles.length }} loop(s)</span>
      </span>
      <button title="Fit" @click="fit"><i class="codicon codicon-screen-full" /></button>
      <button title="Read again" @click="loadAssemblies()"><i class="codicon codicon-refresh" :class="{ 'codicon-modifier-spin': store.busy.assemblies }" /></button>
    </div>
    <div class="body">
      <div ref="box" class="canvas" @pointerdown="down" @pointermove="move" @pointerup="up" @wheel.prevent="wheel">
        <p v-if="!store.assemblies" class="empty">Reading assembly definitions…</p>
        <p v-else-if="!graph.nodes.length" class="empty">No assembly definitions (.asmdef) here.</p>
        <svg v-else :width="view.w" :height="view.h">
          <defs>
            <marker id="cs-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto"><path d="M0,0 L8,4 L0,8 z" /></marker>
          </defs>
          <g :transform="`translate(${view.x},${view.y}) scale(${view.k})`">
            <path v-for="([f, t], k) in graph.edges" :key="k" class="edge" :class="edgeClass(f, t)" :d="edgePath(f, t)" marker-end="url(#cs-arrow)" />
            <g v-for="n in graph.nodes" :key="n.i" class="node" :class="nodeClass(n)" :transform="`translate(${n.x},${n.y})`" @click="focus = n.i" @dblclick="openPath(n.a.path)">
              <title>{{ n.a.name }} — {{ rel(n.a.path) }}</title>
              <rect :width="n.w" :height="H" rx="4" />
              <text :x="n.w / 2" :y="H / 2 + 4">{{ n.a.name }}</text>
              <circle v-if="n.missing" :cx="n.w - 4" cy="4" r="4" class="missing" />
            </g>
          </g>
        </svg>
        <div class="legend">
          <span><i class="sw editor" /> Editor only</span>
          <span><i class="sw test" /> Tests</span>
          <span><i class="sw loop" /> Loop</span>
          <span><i class="dot" /> References to nothing</span>
          <span class="hint">Click to focus · double-click to open · drag and scroll to move</span>
        </div>
      </div>
      <aside v-if="focused" class="side">
        <h4 :title="focused.path">{{ focused.name }}</h4>
        <p class="path" @click="openPath(focused.path)">{{ rel(focused.path) }}</p>
        <p class="facts">
          {{ focused.rootNamespace ? 'namespace ' + focused.rootNamespace + ' · ' : '' }}{{ focused.autoReferenced ? 'auto-referenced' : 'not auto-referenced' }}
          · needed by {{ neededByCount }}
        </p>
        <p v-if="focused.includePlatforms?.length" class="facts">Only on {{ focused.includePlatforms.join(', ') }}</p>
        <p v-if="focused.excludePlatforms?.length" class="facts">Not on {{ focused.excludePlatforms.join(', ') }}</p>
        <p v-if="focused.defineConstraints?.length" class="facts">When {{ focused.defineConstraints.join(' && ') }}</p>
        <h5>References</h5>
        <ul>
          <li v-for="r in focused.references" :key="r.raw" :class="r.status">{{ refLabel(r) }}</li>
          <li v-if="!focused.references.length" class="faint">None</li>
        </ul>
        <button @click="openPath(focused.path)"><i class="codicon codicon-go-to-file" /> Open .asmdef</button>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.graph {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border);
}
.bar input:not([type]) {
  background: var(--bg);
  color: var(--text);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  padding: 2px 6px;
  font: inherit;
  width: 180px;
}
label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--text-dim);
}
.facts {
  color: var(--text-faint);
}
.bad {
  color: var(--error);
}
button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 7px;
  border-radius: 4px;
  color: var(--text-dim);
}
button:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.bar button:first-of-type {
  margin-left: auto;
}
.body {
  flex: 1;
  min-height: 0;
  display: flex;
}
.canvas {
  flex: 1;
  min-width: 0;
  position: relative;
  overflow: hidden;
  cursor: grab;
}
svg {
  display: block;
}
.edge {
  fill: none;
  stroke: var(--border-strong);
  stroke-width: 1.2;
}
marker path {
  fill: var(--border-strong);
}
.edge.lit {
  stroke: var(--accent);
  stroke-width: 1.6;
}
.edge.loop {
  stroke: var(--error);
  stroke-width: 1.8;
}
.edge.dim {
  opacity: 0.12;
}
.node {
  cursor: pointer;
}
.node rect {
  fill: var(--bg-raised, var(--bg));
  stroke: var(--border-strong);
}
.node text {
  fill: var(--text);
  font-size: 11px;
  text-anchor: middle;
  font-family: var(--font-mono);
}
.node.editor rect {
  fill: var(--bg-side, var(--bg));
  stroke-dasharray: 4 2;
}
.node.test rect {
  stroke: var(--success, #7fb069);
}
.node.external text {
  fill: var(--text-faint);
}
.node.loop rect {
  stroke: var(--error);
  stroke-width: 2;
}
.node.focus rect {
  stroke: var(--accent);
  stroke-width: 2;
}
.node.dim {
  opacity: 0.2;
}
.missing {
  fill: var(--warning, #d7a14a);
}
.legend {
  position: absolute;
  left: 8px;
  bottom: 6px;
  display: flex;
  gap: 12px;
  color: var(--text-faint);
  font-size: 11px;
  pointer-events: none;
}
.sw {
  display: inline-block;
  width: 12px;
  height: 8px;
  border: 1px solid var(--border-strong);
  border-radius: 2px;
}
.sw.editor {
  border-style: dashed;
}
.sw.test {
  border-color: var(--success, #7fb069);
}
.sw.loop {
  border-color: var(--error);
}
.dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 4px;
  background: var(--warning, #d7a14a);
}
.empty {
  color: var(--text-dim);
  padding: 12px;
}
.side {
  width: 280px;
  border-left: 1px solid var(--border);
  padding: 8px 10px;
  overflow: auto;
}
.side h4 {
  margin: 0 0 2px;
  font-size: 13px;
  color: var(--text);
}
.side h5 {
  margin: 10px 0 4px;
  color: var(--text-dim);
  font-weight: 600;
}
.side p {
  margin: 2px 0;
}
.path {
  color: var(--accent);
  cursor: pointer;
  word-break: break-all;
}
ul {
  margin: 0 0 8px;
  padding-left: 16px;
}
li {
  color: var(--text);
  word-break: break-all;
}
li.external,
li.faint {
  color: var(--text-faint);
}
li.unresolved {
  color: var(--warning, #d7a14a);
}
</style>
