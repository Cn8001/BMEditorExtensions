<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { api, basename, call, checkMeta, fail, openPath, rel, store, type MetaFix } from './api'

// Unity: the compile errors and warnings Unity wrote to its Editor.log,
// and the .meta files: what is missing, left over, doubled or likely
// renamed outside Unity, with a fix for each.

const showErrors = ref(true)
const showWarnings = ref(true)
const showLog = ref(false)
const logBox = ref<HTMLPreElement>()

const messages = computed(() => store.log?.messages ?? [])
const errors = computed(() => messages.value.filter((m) => m.severity === 'error').length)
const warnings = computed(() => messages.value.length - errors.value)
const shown = computed(() => messages.value.filter((m) => (m.severity === 'error' ? showErrors.value : showWarnings.value)))

watch(
  () => store.logLines.length,
  async () => {
    if (!showLog.value) return
    await nextTick()
    logBox.value?.scrollTo({ top: logBox.value.scrollHeight })
  },
)

const status = computed(() => {
  const l = store.log
  if (!store.workspace) return 'Open a folder.'
  if (store.workspace.kind !== 'unity') return 'This folder is not a Unity project (no ProjectSettings/ProjectVersion.txt).'
  if (!l) return 'Starting…'
  if (!l.exists) return `No Editor.log at ${l.path}. Start Unity on this project, or set the log's path in Settings › C#.`
  if (!l.matches) return `Unity's log is for another project: ${l.project}. Open this one in Unity to see its errors.`
  return `Following ${l.path}`
})

// .meta files

const report = computed(() => store.meta?.report)
const paired = computed(() => new Set((report.value?.renames ?? []).flatMap((r) => [r.meta, r.to])))
const missing = computed(() => (report.value?.missing ?? []).filter((p) => !paired.value.has(p)))
const orphans = computed(() => (report.value?.orphans ?? []).filter((p) => !paired.value.has(p)))
const problems = computed(() => {
  const r = report.value
  return r ? r.renames.length + missing.value.length + orphans.value.length + r.duplicates.length : 0
})
const confirm = ref('')

async function fix(fixes: MetaFix[], what: string) {
  if (!fixes.length) return
  try {
    const r = await call<{ failed: Record<string, string> }>('meta.fix', { fixes })
    const failed = Object.entries(r.failed)
    if (failed.length) {
      api.ui.toast('error', `${what}: ${failed.length} of ${fixes.length} failed`, failed.slice(0, 5).map(([i, e]) => `${basename(fixes[+i].path)}: ${e}`).join('\n'))
    }
  } catch (err) {
    fail(what, err)
  }
  confirm.value = ''
  await checkMeta()
}

// A destructive fix asks twice: the first click arms it.
function armed(key: string) {
  if (confirm.value === key) return true
  confirm.value = key
  setTimeout(() => confirm.value === key && (confirm.value = ''), 4000)
  return false
}

const fixRenames = () => fix(report.value!.renames.map((r) => ({ kind: 'move', path: r.meta, to: r.to })), 'Move .meta files')
const createMissing = (paths = missing.value) => fix(paths.map((p) => ({ kind: 'create', path: p })), 'Create .meta files')
function deleteOrphans(paths = orphans.value, key = 'orphans') {
  if (armed(key)) void fix(paths.map((p) => ({ kind: 'delete', path: p })), 'Delete .meta files')
}
function regen(meta: string) {
  if (armed('regen:' + meta)) void fix([{ kind: 'regen', path: meta }], 'New GUID')
}

function openAsset(path: string) {
  void openPath(path.replace(/\.meta$/, ''))
}
</script>

<template>
  <div class="panel">
    <div class="bar">
      <div class="tabs">
        <button :class="{ on: store.ui.unityTab === 'console' }" @click="store.ui.unityTab = 'console'">
          <i class="codicon codicon-output" /> Console
          <span v-if="errors" class="count err">{{ errors }}</span>
          <span v-if="warnings" class="count warn">{{ warnings }}</span>
        </button>
        <button :class="{ on: store.ui.unityTab === 'meta' }" @click="store.ui.unityTab = 'meta'">
          <i class="codicon codicon-file-symlink-file" /> Meta
          <span v-if="problems" class="count warn">{{ problems }}</span>
        </button>
      </div>
      <template v-if="store.ui.unityTab === 'console'">
        <label><input v-model="showErrors" type="checkbox" /> Errors</label>
        <label><input v-model="showWarnings" type="checkbox" /> Warnings</label>
        <label><input v-model="showLog" type="checkbox" /> Log</label>
        <span class="status" :title="status">{{ status }}</span>
      </template>
      <template v-else>
        <button class="primary" :disabled="store.busy.meta" @click="checkMeta()"><i class="codicon codicon-search" /> Check</button>
        <span v-if="report" class="status">{{ report.checked }} assets in {{ store.meta!.roots.map((r) => rel(r) || basename(r)).join(', ') || 'no asset folder' }}</span>
      </template>
    </div>

    <div v-if="store.ui.unityTab === 'console'" class="body" :class="{ split: showLog }">
      <div class="messages">
        <p v-if="!shown.length" class="empty">{{ messages.length ? 'Nothing to show with these filters.' : store.log?.exists && store.log.matches ? 'No compile errors or warnings.' : '' }}</p>
        <div v-for="(m, i) in shown" :key="i" class="msg" :class="m.severity" @click="openPath(m.file, m.line, m.col)">
          <i class="codicon" :class="m.severity === 'error' ? 'codicon-error' : 'codicon-warning'" />
          <span class="where">{{ m.rel }}({{ m.line }},{{ m.col }})</span>
          <span class="code">{{ m.code }}</span>
          <span class="text">{{ m.message }}</span>
        </div>
      </div>
      <pre v-if="showLog" ref="logBox" class="log">{{ store.logLines.join('\n') }}</pre>
    </div>

    <div v-else class="body meta">
      <p v-if="store.busy.meta" class="empty">Checking…</p>
      <p v-else-if="!report" class="empty">
        Check finds files and folders without a .meta, .meta files whose asset is gone, GUIDs used twice, and assets renamed or moved outside Unity whose .meta stayed
        behind. Unity gives a renamed asset a new GUID unless its .meta moves with it, which breaks every reference to it.
      </p>
      <p v-else-if="!store.meta!.roots.length" class="empty">No Unity assets here: no Assets folder, package or .meta files.</p>
      <p v-else-if="!problems" class="empty"><i class="codicon codicon-pass" /> Every .meta file is in order.</p>
      <template v-else>
        <section v-if="report.renames.length">
          <h4>
            Likely renamed or moved ({{ report.renames.length }})
            <button class="primary" @click="fixRenames">Move the .meta files</button>
          </h4>
          <p class="hint">Moving the .meta keeps the GUID, so references to the asset keep working.</p>
          <div v-for="r in report.renames" :key="r.meta" class="item">
            <span class="tag">{{ r.reason }}</span>
            <span class="path" @click="openAsset(r.to)">{{ rel(r.meta) }} → {{ rel(r.to) }}.meta</span>
            <button @click="fix([{ kind: 'move', path: r.meta, to: r.to }], 'Move .meta')">Move</button>
          </div>
        </section>
        <section v-if="missing.length">
          <h4>
            Without a .meta ({{ missing.length }})
            <button @click="createMissing()">Create all</button>
          </h4>
          <p class="hint">Unity makes these when it imports the folder; a package needs them in version control.</p>
          <div v-for="p in missing.slice(0, 300)" :key="p" class="item">
            <span class="path" @click="openAsset(p)">{{ rel(p) }}</span>
            <button @click="createMissing([p])">Create</button>
          </div>
          <p v-if="missing.length > 300" class="hint">and {{ missing.length - 300 }} more</p>
        </section>
        <section v-if="orphans.length">
          <h4>
            Left over ({{ orphans.length }})
            <button :class="{ danger: confirm === 'orphans' }" @click="deleteOrphans()">{{ confirm === 'orphans' ? 'Click again to delete all' : 'Delete all' }}</button>
          </h4>
          <p class="hint">Their asset is gone. If it was renamed outside Unity, move the .meta to it instead.</p>
          <div v-for="p in orphans" :key="p" class="item">
            <span class="path">{{ rel(p) }}</span>
            <button :class="{ danger: confirm === 'o:' + p }" @click="deleteOrphans([p], 'o:' + p)">{{ confirm === 'o:' + p ? 'Sure?' : 'Delete' }}</button>
          </div>
        </section>
        <section v-if="report.duplicates.length">
          <h4>Same GUID twice ({{ report.duplicates.length }})</h4>
          <p class="hint">Usually from copying an asset with its .meta. Give the copy a new GUID; references keep pointing at the first.</p>
          <div v-for="dup in report.duplicates" :key="dup.guid" class="dup">
            <span class="guid">{{ dup.guid }}</span>
            <div v-for="m in dup.metas" :key="m" class="item">
              <span class="path" @click="openAsset(m)">{{ rel(m) }}</span>
              <button :class="{ danger: confirm === 'regen:' + m }" @click="regen(m)">{{ confirm === 'regen:' + m ? 'Sure?' : 'New GUID' }}</button>
            </div>
          </div>
        </section>
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
.bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border);
}
.tabs {
  display: flex;
  gap: 2px;
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
.tabs button.on {
  background: var(--bg-active);
  color: var(--text);
}
button.primary {
  background: var(--accent);
  color: var(--accent-ink, #000);
}
button.danger {
  background: var(--error);
  color: #fff;
}
label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--text-dim);
}
.count {
  border-radius: 8px;
  padding: 0 5px;
  font-size: 10px;
  color: #000;
}
.count.err {
  background: var(--error);
  color: #fff;
}
.count.warn {
  background: var(--warning, #d7a14a);
}
.status {
  color: var(--text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
.body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.body.split .messages {
  flex: 1;
}
.messages {
  flex: 1;
  overflow: auto;
}
.log {
  flex: 1;
  margin: 0;
  overflow: auto;
  border-top: 1px solid var(--border);
  padding: 6px 8px;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-dim);
  white-space: pre-wrap;
}
.msg {
  display: flex;
  gap: 8px;
  align-items: baseline;
  padding: 3px 10px;
  cursor: pointer;
}
.msg:hover {
  background: var(--bg-hover);
}
.msg.error .codicon {
  color: var(--error);
}
.msg.warning .codicon {
  color: var(--warning, #d7a14a);
}
.where {
  color: var(--accent);
  white-space: nowrap;
}
.code {
  color: var(--text-faint);
}
.text {
  color: var(--text);
}
.meta {
  overflow: auto;
  padding: 4px 12px 12px;
  display: block;
}
section {
  margin-bottom: 12px;
}
h4 {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 8px 0 2px;
  color: var(--text);
}
.hint {
  color: var(--text-faint);
  margin: 0 0 4px;
}
.item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 1px 0;
}
.item button {
  margin-left: auto;
}
.path {
  color: var(--text);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.path:hover {
  color: var(--accent);
}
.tag {
  color: var(--text-faint);
  width: 60px;
}
.guid {
  font-family: var(--font-mono);
  color: var(--text-faint);
}
.dup {
  margin-bottom: 6px;
}
.empty {
  color: var(--text-dim);
  padding: 10px 12px;
  max-width: 760px;
  line-height: 1.5;
}
</style>
