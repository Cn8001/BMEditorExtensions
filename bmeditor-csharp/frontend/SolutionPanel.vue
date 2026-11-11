<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { api, basename, call, fail, loadAssemblies, loadSolution, openPath, rel, sep, store } from './api'
import { rows, solutionTree, type Node } from './tree'
import Graph from './Graph.vue'

// The Solution Explorer: the solution as Visual Studio shows it, the
// assembly graph, and New Script for a folder.

const open = reactive(new Set<string>())
const filter = ref('')
const selected = ref('')
const most = 3000

const tree = computed(() => (store.solution ? solutionTree(store.solution, sep()) : null))
const visible = computed(() => (tree.value ? rows(tree.value, open, filter.value) : []))
const shown = computed(() => visible.value.slice(0, most))

// A new solution opens on its folders and projects.
watch(tree, (t) => {
  if (!t) return
  open.add(t.id)
  for (const c of t.children ?? []) if (c.kind === 'sfolder') open.add(c.id)
})

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
  <div class="panel">
    <div class="bar">
      <div class="tabs">
        <button :class="{ on: store.ui.solutionTab === 'solution' }" @click="store.ui.solutionTab = 'solution'"><i class="codicon codicon-list-tree" /> Solution</button>
        <button :class="{ on: store.ui.solutionTab === 'graph' }" @click="store.ui.solutionTab = 'graph'"><i class="codicon codicon-type-hierarchy" /> Assemblies</button>
      </div>
      <template v-if="store.ui.solutionTab === 'solution'">
        <select v-if="store.solutions.length > 1" v-model="store.solutionPath" title="Solution">
          <option v-for="s in store.solutions" :key="s" :value="s">{{ rel(s) || basename(s) }}</option>
        </select>
        <input v-model="filter" class="filter" placeholder="Filter files" spellcheck="false" />
        <button title="New Script in the solution's folder" :disabled="!store.workspace" @click="store.ui.newScriptDir = store.workspace!.root"><i class="codicon codicon-new-file" /></button>
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

    <Graph v-if="store.ui.solutionTab === 'graph'" />
    <div v-else class="tree">
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
          :title="r.node.path || r.node.dir || r.node.label"
          @click="click(r.node)"
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
