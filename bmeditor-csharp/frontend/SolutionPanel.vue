<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { api, basename, call, fail, loadAssemblies, loadSolution, openPath, rel, sep, setShowGenerated, store } from './api'
import { packageCount, pathTo, rows, solutionTree, type Node } from './tree'
import Graph from './Graph.vue'

// The Solution Explorer: the solution as Visual Studio shows it, the
// assembly graph, and New Script for a folder.

// side is the Explorer's Solution view: narrow, one row of tools, and the
// graph a view of its own.
const props = defineProps<{ side?: boolean }>()

const open = reactive(new Set<string>())
const filter = ref('')
const selected = ref('')
const treeEl = ref<HTMLElement>()
const most = 3000

// Track Active Item and package projects, as Visual Studio's toolbar has
// them, kept between sessions.
const prefsKey = 'bmeditor-csharp.solution'
const prefs = reactive({ track: true, packages: false })
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

const tree = computed(() => (store.solution ? solutionTree(store.solution, sep(), prefs.packages) : null))
const packages = computed(() => (store.solution ? packageCount(store.solution) : 0))
const visible = computed(() => (tree.value ? rows(tree.value, open, filter.value) : []))
const shown = computed(() => visible.value.slice(0, most))

// A new solution opens on its folders and projects.
watch(
  tree,
  (t) => {
    if (!t) return
    open.add(t.id)
    for (const c of t.children ?? []) if (c.kind === 'sfolder' && c.id !== 's:packages') open.add(c.id)
  },
  { immediate: true },
)

onMounted(() => {
  if (!store.solution && !store.busy.solution && store.workspace) void loadSolution()
  if (store.ui.solutionTab === 'graph' && !store.assemblies) void loadAssemblies()
})

watch(
  () => store.ui.solutionTab,
  (t) => t === 'graph' && !store.assemblies && loadAssemblies(),
)

watch(
  () => store.solutionPath,
  () => store.workspace && loadSolution(),
)

function collapseAll() {
  open.clear()
  if (tree.value) open.add(tree.value.id)
}

// reveal opens the folders above the active tab's file and selects it.
async function reveal() {
  const t = tree.value
  const tab = api.session.tab(api.ide.activeId)
  if (!prefs.track || !t || !tab?.path) return
  const ids = pathTo(t, tab.path)
  if (!ids) return
  for (const id of ids.slice(0, -1)) open.add(id)
  selected.value = ids[ids.length - 1]
  await nextTick()
  treeEl.value?.querySelector('.row.selected')?.scrollIntoView({ block: 'nearest' })
}
watch(() => [api.ide.activeId, tree.value, prefs.track], () => void reveal(), { immediate: true })

// The context menu, and the bar that asks for a name or a yes.
const menu = ref<{ node: Node; x: number; y: number } | null>(null)
const asking = ref<{ kind: 'rename' | 'folder' | 'delete'; node: Node; value: string } | null>(null)
const askInput = ref<HTMLInputElement>()

interface MenuEntry {
  label: string
  icon: string
  run: () => unknown
  danger?: boolean
  checked?: boolean
}

function entries(n: Node): MenuEntry[] {
  const out: MenuEntry[] = []
  const folder = n.dir
  if (n.kind === 'file' && n.path) out.push({ label: 'Open', icon: 'go-to-file', run: () => openPath(n.path!) })
  if (folder) {
    out.push({ label: 'New Script…', icon: 'new-file', run: () => (store.ui.newScriptDir = folder) })
    out.push({ label: 'New Folder…', icon: 'new-folder', run: () => ask('folder', n, '') })
  }
  if ((n.kind === 'project' || n.kind === 'solution') && n.path) out.push({ label: n.kind === 'project' ? 'Open Project File' : 'Open Solution File', icon: 'file-code', run: () => openPath(n.path!) })
  if (n.kind === 'file' || (n.kind === 'folder' && folder)) {
    out.push({ label: 'Rename…', icon: 'edit', run: () => ask('rename', n, n.label) })
    out.push({ label: 'Delete', icon: 'trash', danger: true, run: () => ask('delete', n, '') })
  }
  const path = n.path ?? folder
  if (path) out.push({ label: 'Copy Path', icon: 'copy', run: () => navigator.clipboard?.writeText(path) })
  if (n.kind === 'solution') {
    out.push({ label: 'Collapse All', icon: 'collapse-all', run: collapseAll })
    if (store.workspace?.kind === 'unity') {
      out.push({ label: "Show Unity's Generated Files in Explorer", icon: 'eye', checked: store.showGenerated, run: () => setShowGenerated(!store.showGenerated) })
    }
  }
  return out
}

function openMenu(n: Node, e: MouseEvent) {
  selected.value = n.id
  if (!entries(n).length) return
  menu.value = { node: n, x: Math.min(e.clientX, window.innerWidth - 260), y: Math.min(e.clientY, window.innerHeight - 260) }
}

function pick(entry: MenuEntry) {
  menu.value = null
  void entry.run()
}

async function ask(kind: 'rename' | 'folder' | 'delete', node: Node, value: string) {
  asking.value = { kind, node, value }
  await nextTick()
  const input = askInput.value
  if (!input) return
  input.focus()
  // The name without its extension selected, as Visual Studio does.
  const dot = value.lastIndexOf('.')
  input.setSelectionRange(0, kind === 'rename' && dot > 0 ? dot : value.length)
}

async function answer() {
  const a = asking.value
  if (!a) return
  const target = a.node.path ?? a.node.dir
  try {
    if (a.kind === 'delete') await call('asset.delete', { path: target })
    else if (a.kind === 'rename') {
      if (!a.value.trim() || a.value === a.node.label) return void (asking.value = null)
      const r = await call<{ path: string; warning?: string }>('asset.rename', { path: target, name: a.value })
      if (r.warning) api.ui.toast('info', 'Renamed', r.warning)
      if (a.node.kind === 'file') void openPath(r.path)
    } else {
      if (!a.value.trim()) return
      const r = await call<{ path: string; warning?: string }>('asset.newFolder', { dir: a.node.dir, name: a.value })
      if (r.warning) api.ui.toast('info', 'Folder created', r.warning)
    }
    asking.value = null
    void loadSolution()
  } catch (err) {
    fail(a.kind === 'delete' ? 'Delete' : a.kind === 'rename' ? 'Rename' : 'New Folder', err)
  }
}

function closeMenu() {
  menu.value = null
}
onMounted(() => window.addEventListener('click', closeMenu))
onBeforeUnmount(() => window.removeEventListener('click', closeMenu))

function click(n: Node) {
  selected.value = n.id
  if (n.children?.length) {
    if (open.has(n.id)) open.delete(n.id)
    else open.add(n.id)
  } else if (n.path && (n.kind === 'file' || n.kind === 'error' || n.kind === 'ref')) {
    void openPath(n.path)
  }
}

function solutionMissing() {
  return store.workspace?.kind === 'unity'
}

// New Script

const kinds = ref<{ ID: string; Label: string }[]>([])
const name = ref('')
const kind = ref('monobehaviour')
const nameInput = ref<HTMLInputElement>()
const creating = ref(false)

watch(
  () => store.ui.newScriptDir,
  async (dir) => {
    if (!dir) return
    if (!kinds.value.length) kinds.value = await call('script.kinds')
    name.value = ''
    await nextTick()
    nameInput.value?.focus()
  },
  { immediate: true },
)

async function create() {
  if (!name.value.trim() || creating.value) return
  creating.value = true
  try {
    const r = await call<{ path: string; warning?: string }>('script.new', { dir: store.ui.newScriptDir, name: name.value, kind: kind.value })
    if (r.warning) api.ui.toast('info', 'Script created', r.warning)
    store.ui.newScriptDir = ''
    await openPath(r.path)
    void loadSolution()
  } catch (err) {
    fail('New Script', err)
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="panel" :class="{ side: props.side }">
    <div class="bar">
      <div v-if="!props.side" class="tabs">
        <button :class="{ on: store.ui.solutionTab === 'solution' }" @click="store.ui.solutionTab = 'solution'"><i class="codicon codicon-list-tree" /> Solution</button>
        <button :class="{ on: store.ui.solutionTab === 'graph' }" @click="store.ui.solutionTab = 'graph'"><i class="codicon codicon-type-hierarchy" /> Assemblies</button>
      </div>
      <template v-if="props.side || store.ui.solutionTab === 'solution'">
        <select v-if="store.solutions.length > 1" v-model="store.solutionPath" title="Solution">
          <option v-for="s in store.solutions" :key="s" :value="s">{{ rel(s) || basename(s) }}</option>
        </select>
        <input v-model="filter" class="filter" placeholder="Filter files" spellcheck="false" />
        <button title="New Script in the solution's folder" :disabled="!store.workspace" @click="store.ui.newScriptDir = store.workspace!.root"><i class="codicon codicon-new-file" /></button>
        <button title="Track Active Item" :class="{ on: prefs.track }" @click="prefs.track = !prefs.track"><i class="codicon codicon-target" /></button>
        <button title="Collapse All" @click="collapseAll"><i class="codicon codicon-collapse-all" /></button>
        <button
          v-if="packages"
          :title="`${prefs.packages ? 'Hide' : 'Show'} the ${packages} package projects (Unity's registry, git and built-in packages)`"
          :class="{ on: prefs.packages }"
          @click="prefs.packages = !prefs.packages"
        >
          <i class="codicon codicon-package" />
        </button>
        <button title="Refresh" @click="loadSolution()"><i class="codicon codicon-refresh" :class="{ 'codicon-modifier-spin': store.busy.solution }" /></button>
      </template>
    </div>

    <form v-if="store.ui.newScriptDir" class="new" @submit.prevent="create">
      <span class="where" :title="store.ui.newScriptDir">New script in {{ rel(store.ui.newScriptDir) || basename(store.ui.newScriptDir) }}</span>
      <input ref="nameInput" v-model="name" placeholder="Name" spellcheck="false" @keydown.esc="store.ui.newScriptDir = ''" />
      <select v-model="kind">
        <option v-for="k in kinds" :key="k.ID" :value="k.ID">{{ k.Label }}</option>
      </select>
      <button type="submit" class="primary" :disabled="!name.trim() || creating">Create</button>
      <button type="button" @click="store.ui.newScriptDir = ''">Cancel</button>
    </form>

    <form v-if="asking" class="new" @submit.prevent="answer">
      <template v-if="asking.kind === 'delete'">
        <span class="where">Delete {{ asking.node.label }}{{ asking.node.kind === 'folder' ? ' and everything in it' : '' }}, with its .meta?</span>
        <button type="submit" class="primary danger">Delete</button>
      </template>
      <template v-else>
        <span class="where">{{ asking.kind === 'rename' ? 'Rename ' + asking.node.label + ' to' : 'New folder in ' + asking.node.label }}</span>
        <input ref="askInput" v-model="asking.value" placeholder="Name" spellcheck="false" @keydown.esc="asking = null" />
        <button type="submit" class="primary" :disabled="!asking.value.trim()">{{ asking.kind === 'rename' ? 'Rename' : 'Create' }}</button>
      </template>
      <button type="button" @click="asking = null">Cancel</button>
    </form>

    <Graph v-if="!props.side && store.ui.solutionTab === 'graph'" />
    <div v-else ref="treeEl" class="tree">
      <p v-if="!store.workspace" class="empty">Open a folder to see its solution.</p>
      <p v-else-if="store.busy.solution && !store.solution" class="empty">Reading the solution…</p>
      <div v-else-if="!store.solution" class="empty">
        <p>{{ store.solutionError || 'No solution.' }}</p>
        <p v-if="solutionMissing()">
          Unity writes the solution for its external script editor. In Unity, open Edit › Preferences › External Tools, choose Visual Studio or Visual Studio Code there, tick the
          project files to generate, and press Regenerate project files. The Assemblies tab works without it.
        </p>
      </div>
      <template v-else>
        <div
          v-for="r in shown"
          :key="r.node.id"
          class="row"
          :class="[r.node.kind, { selected: selected === r.node.id }]"
          :style="{ paddingLeft: 6 + r.depth * 14 + 'px' }"
          :title="r.node.title || r.node.path || r.node.dir || r.node.label"
          @click="click(r.node)"
          @contextmenu.prevent.stop="openMenu(r.node, $event)"
        >
          <i class="chevron codicon" :class="r.node.children?.length ? (r.open ? 'codicon-chevron-down' : 'codicon-chevron-right') : ''" />
          <i class="codicon" :class="'codicon-' + r.node.icon" />
          <span class="label">{{ r.node.label }}</span>
          <span v-if="r.node.detail" class="detail">{{ r.node.detail }}</span>
          <button v-if="r.node.dir" class="add" title="New Script here" @click.stop="store.ui.newScriptDir = r.node.dir!"><i class="codicon codicon-new-file" /></button>
        </div>
        <p v-if="visible.length > most" class="empty">{{ visible.length - most }} more rows: filter to narrow them.</p>
      </template>
    </div>

    <div v-if="menu" class="menu" :style="{ left: menu.x + 'px', top: menu.y + 'px' }" @click.stop>
      <button v-for="e in entries(menu.node)" :key="e.label" :class="{ danger: e.danger }" @click="pick(e)">
        <i class="codicon" :class="'codicon-' + (e.checked === undefined ? e.icon : e.checked ? 'check' : 'blank')" />{{ e.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  font-size: 12px;
}
.bar,
.new {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border);
}
.tabs {
  display: flex;
  gap: 2px;
  margin-right: 6px;
}
button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 7px;
  border-radius: 4px;
  color: var(--text-dim);
}
button:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--text);
}
button:disabled {
  opacity: 0.5;
}
.bar > button.on {
  background: var(--bg-active);
  color: var(--accent);
}
.menu {
  position: fixed;
  z-index: 95;
  display: flex;
  flex-direction: column;
  min-width: 220px;
  padding: 4px;
  background: var(--bg-raised);
  border: 1px solid var(--border-strong);
  border-radius: 7px;
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.5);
}
.menu button {
  gap: 8px;
  padding: 5px 9px;
  color: var(--text);
  text-align: left;
}
.menu button.danger:hover {
  color: var(--error, #e06c6c);
}
button.primary.danger {
  background: var(--error, #e06c6c);
  color: #fff;
}
.side .bar {
  flex-wrap: wrap;
  gap: 2px;
  padding: 0 8px 6px;
  border-bottom: none;
}
.side .bar > button {
  padding: 3px 5px;
}
.side .filter {
  order: -1;
  flex: 1;
  min-width: 90px;
  max-width: none;
  margin-right: 4px;
}
.side select {
  order: -2;
  flex-basis: 100%;
  min-width: 0;
  margin-bottom: 4px;
}
.tabs button.on {
  background: var(--bg-active);
  color: var(--text);
}
button.primary {
  background: var(--accent);
  color: var(--accent-ink, #000);
}
input,
select {
  background: var(--bg);
  color: var(--text);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  padding: 2px 6px;
  font: inherit;
}
.filter {
  flex: 1;
  min-width: 80px;
  max-width: 320px;
}
.where {
  color: var(--text-dim);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 40%;
}
.tree {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 2px 0;
}
.row {
  display: flex;
  align-items: center;
  gap: 5px;
  height: 22px;
  padding-right: 8px;
  cursor: pointer;
  white-space: nowrap;
  color: var(--text);
}
.row:hover {
  background: var(--bg-hover);
}
.row.selected {
  background: var(--bg-active);
}
.row .chevron {
  width: 14px;
  color: var(--text-faint);
}
.row .codicon:not(.chevron) {
  color: var(--text-dim);
}
.row.project .codicon:not(.chevron) {
  color: var(--accent);
}
.row.error .codicon:not(.chevron) {
  color: var(--warning, #d7a14a);
}
.label {
  overflow: hidden;
  text-overflow: ellipsis;
}
.detail {
  color: var(--text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
}
.row.error .detail {
  color: var(--warning, #d7a14a);
}
.add {
  margin-left: auto;
  visibility: hidden;
  padding: 1px 4px;
}
.row:hover .add {
  visibility: visible;
}
.empty {
  color: var(--text-dim);
  padding: 8px 12px;
  max-width: 720px;
  line-height: 1.5;
  white-space: normal;
}
.empty p {
  margin: 0 0 6px;
}
</style>
