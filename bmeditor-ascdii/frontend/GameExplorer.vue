<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { absolute, api, manifestTab, openUse, store, type Asset, type Kind } from './api'
import ProjectSettings from './ProjectSettings.vue'

// The Explorer's Game view of an ASCDII project: its assets by kind, as the
// engine's Project window has them, each script with the objects that use
// it, and installed packages last. The .meta files stay out of sight.
// With the .ascdiiproj open, its Project Settings show under the list, as
// Rider's Properties follow what is open.

const kinds: { kind: Kind; label: string; icon: string }[] = [
  { kind: 'script', label: 'Scripts', icon: 'file-code' },
  { kind: 'scene', label: 'Scenes', icon: 'globe' },
  { kind: 'prefab', label: 'Prefabs', icon: 'package' },
  { kind: 'material', label: 'Materials', icon: 'symbol-color' },
  { kind: 'model', label: 'Models', icon: 'symbol-structure' },
  { kind: 'texture', label: 'Textures', icon: 'file-media' },
  { kind: 'sprite', label: 'Sprites', icon: 'symbol-misc' },
  { kind: 'animator', label: 'Animators', icon: 'pulse' },
  { kind: 'audio', label: 'Audio', icon: 'unmute' },
]
const iconOf = Object.fromEntries(kinds.map((k) => [k.kind, k.icon])) as Record<Kind, string>

const prefsKey = 'bmeditor-ascdii.explorer'
const closed = reactive(new Set<string>())
try {
  for (const id of JSON.parse(localStorage.getItem(prefsKey) ?? '[]')) closed.add(id)
} catch {
  // all open then
}
watch(closed, () => {
  try {
    localStorage.setItem(prefsKey, JSON.stringify([...closed]))
  } catch {
    // only for this session
  }
})
function toggle(id: string) {
  if (closed.has(id)) closed.delete(id)
  else closed.add(id)
}

const filter = ref('')
const open = reactive(new Set<string>()) // scripts showing their uses

const base = (p: string) => p.slice(p.lastIndexOf('/') + 1)
const folder = (p: string) => (p.includes('/') ? p.slice(0, p.lastIndexOf('/')) : '')

const groups = computed(() => {
  const all = store.index?.assets ?? []
  const f = filter.value.trim().toLowerCase()
  const keep = (a: Asset) => !f || a.path.toLowerCase().includes(f)
  const own = kinds
    .map((k) => ({ id: k.kind, label: k.label, icon: k.icon, assets: all.filter((a) => !a.package && a.kind === k.kind && keep(a)) }))
    .filter((g) => g.assets.length)
  const packages = [...new Set(all.filter((a) => a.package).map((a) => a.package!))].sort()
  const pkg = packages
    .map((p) => ({ id: 'pkg:' + p, label: p, icon: 'library', assets: all.filter((a) => a.package === p && keep(a)) }))
    .filter((g) => g.assets.length)
  return { own, pkg }
})

const start = computed(() => store.status?.project?.manifest.start ?? '')
const active = computed(() => {
  const tab = api.session.tab(api.ide.activeId)
  return tab?.path ?? ''
})
const isActive = (a: Asset) => active.value.replace(/\\/g, '/').endsWith('/' + a.path)

function openAsset(a: Asset) {
  void api.session.openFile(absolute(a.path))
}

const uses = (a: Asset) => store.index?.uses[a.path] ?? []

const settingsTab = computed(() => {
  const t = manifestTab()
  return t && t.id === api.ide.activeId ? t : undefined
})

function openManifest() {
  if (store.status?.project) void api.session.openFile(store.status.project.file)
}
</script>

<template>
  <div class="game">
    <div v-if="store.status?.project" class="title" :title="`Project Settings: ${store.status.project.file}`" @click="openManifest">
      <i class="codicon codicon-game" />
      <span class="name">{{ store.status.project.manifest.name }}</span>
      <span class="dim">{{ store.status.project.manifest.version }}</span>
      <span v-if="store.running" class="running"><i class="codicon codicon-circle-filled" />{{ store.running }}</span>
    </div>
    <div v-if="store.status?.missing" class="notice">
      <p>{{ store.status.missing }}</p>
      <button class="link" @click="api.ui.openSettings()">Set the ascdii program in Settings</button>
      <a class="link" href="https://github.com/Cn8001/ASCDII-Engine" target="_blank" rel="noopener">Get the ASCDII Engine</a>
    </div>
    <input v-model="filter" class="filter" placeholder="Filter assets" spellcheck="false" />
    <div class="list">
      <p v-if="!store.index" class="empty">Reading the project…</p>
      <template v-for="g in [...groups.own, ...groups.pkg]" :key="g.id">
        <div class="row group" @click="toggle(g.id)">
          <i class="codicon chev" :class="closed.has(g.id) && !filter ? 'codicon-chevron-right' : 'codicon-chevron-down'" />
          <i class="codicon" :class="'codicon-' + g.icon" />
          <span class="label">{{ g.label }}</span>
          <span class="count">{{ g.assets.length }}</span>
        </div>
        <template v-if="!closed.has(g.id) || filter">
          <template v-for="a in g.assets" :key="a.path">
            <div class="row asset" :class="{ selected: isActive(a) }" :title="a.path" @click="openAsset(a)">
              <i
                class="codicon chev"
                :class="uses(a).length ? (open.has(a.path) ? 'codicon-chevron-down' : 'codicon-chevron-right') : ''"
                @click.stop="uses(a).length && (open.has(a.path) ? open.delete(a.path) : open.add(a.path))"
              />
              <i class="codicon" :class="'codicon-' + iconOf[a.kind]" />
              <span class="label">{{ base(a.path) }}</span>
              <span v-if="a.path === start" class="tag" title="The scene the game starts in">start</span>
              <span v-if="a.kind === 'script' && uses(a).length" class="count" :title="`Used by ${uses(a).length} object${uses(a).length === 1 ? '' : 's'}`">{{ uses(a).length }}</span>
              <span class="dim">{{ folder(a.path) }}</span>
            </div>
            <template v-if="open.has(a.path)">
              <div v-for="u in uses(a)" :key="u.in + u.object" class="row use" :title="`${u.object} in ${u.in}`" @click="openUse(u.object, u.in)">
                <i class="codicon codicon-symbol-object" />
                <span class="label">{{ u.object || '(unnamed)' }}</span>
                <span class="dim">{{ base(u.in) }}</span>
              </div>
            </template>
          </template>
        </template>
      </template>
      <p v-if="store.index && !groups.own.length && !groups.pkg.length" class="empty">{{ filter ? 'No asset matches.' : 'No assets yet.' }}</p>
    </div>
    <div v-if="settingsTab" class="props">
      <div class="props-head"><i class="codicon codicon-settings" />Project Settings</div>
      <ProjectSettings :tab="settingsTab" />
    </div>
  </div>
</template>

<style scoped>
.game {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.title {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 12px 6px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
}
.title {
  cursor: pointer;
}
.title:hover .name {
  text-decoration: underline;
}
.props {
  flex: none;
  max-height: 60%;
  overflow: auto;
  border-top: 1px solid var(--border);
}
.props-head {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--bg-side);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-dim);
}
.title .dim {
  font-weight: 400;
}
.running {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 400;
  font-size: 11.5px;
  color: var(--text-dim);
}
.running .codicon {
  font-size: 9px;
  color: var(--accent);
}
.notice {
  margin: 0 8px 6px;
  padding: 6px 8px;
  border-radius: 5px;
  background: var(--bg-hover);
  font-size: 12px;
  color: var(--text-dim);
}
.notice p {
  margin: 0 0 4px;
}
.link {
  display: block;
  margin-top: 2px;
  color: var(--accent);
  font-size: 12px;
  text-decoration: none;
}
.link:hover {
  text-decoration: underline;
}
.filter {
  margin: 0 8px 6px;
  padding: 4px 8px;
  border: 1px solid var(--border-strong);
  border-radius: 5px;
  background: var(--bg);
  outline: none;
  font-size: 12px;
  user-select: text;
}
.filter:focus {
  border-color: var(--accent);
}
.list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding-bottom: 12px;
}
.row {
  display: flex;
  align-items: center;
  gap: 5px;
  height: 22px;
  padding: 0 8px 0 6px;
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
.row .codicon {
  flex: none;
  color: var(--text-dim);
}
.chev {
  width: 16px;
}
.group {
  font-weight: 600;
}
.asset {
  padding-left: 20px;
}
.use {
  padding-left: 58px;
}
.label {
  overflow: hidden;
  text-overflow: ellipsis;
}
.dim {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 11.5px;
  color: var(--text-faint);
}
.count {
  padding: 0 6px;
  border-radius: 8px;
  background: var(--bg-active);
  font-size: 11px;
  font-weight: 400;
  color: var(--text-dim);
}
.tag {
  padding: 0 5px;
  border-radius: 3px;
  background: var(--accent-dim);
  color: var(--accent);
  font-size: 10.5px;
}
.empty {
  padding: 4px 12px;
  font-size: 12px;
  color: var(--text-faint);
}
</style>
