<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { api, diff, go, icons, loadChanges, markReviewed, name, openAt, rel, store, type AgentFile, type Change } from './api'
import AgentCard from './AgentCard.vue'
import ArchView from './ArchView.vue'
import Section from './Section.vue'

// Home: the agent's work. Its plan or milestone, the project's shape with
// what it did, and, folded, the files it changed that still differ from
// HEAD: a file opens beside HEAD, a declaration where it is, and the check
// marks a file reviewed as it is now. A file leaves the list once it is
// committed or put back.
onMounted(() => {
  if (!store.report) void loadChanges()
})

const r = computed(() => store.report)
const off = computed(() => store.agent?.level === 'off')
const mark: Record<string, string> = { added: '+', modified: '~', removed: '−' }

const live = (f: AgentFile) => f.changes.filter((c) => c.change !== 'removed')
const reviewed = (f: AgentFile) => live(f).length > 0 && live(f).every((c) => c.review === 'reviewed')

function base(path: string) {
  return path.slice(Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1)
}

function dir(path: string) {
  const p = rel(path)
  const i = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'))
  return i > 0 ? p.slice(0, i) : ''
}

// A new file has nothing to compare with, and a deleted one nothing to open.
function openFile(f: AgentFile) {
  if (f.status === 'A') void api.session.openFile(f.path)
  else if (f.status !== 'D') void diff(f.path)
}

function open(f: AgentFile, c: Change) {
  if (c.change !== 'removed') void openAt(c.file, c.line)
  else if (f.status !== 'D') void diff(c.file)
}

function card(c: Change) {
  if (c.change !== 'removed') void go({ kind: 'card', key: c.key })
}
</script>

<template>
  <div class="changes">
    <AgentCard />
    <div v-if="store.arch?.packages.length && store.agent?.waiting !== 'checkpoint'" class="shape">
      <div class="head"><span>The project's shape, with the agent's work</span></div>
      <ArchView />
    </div>
    <div v-if="store.reportError" class="hint">{{ store.reportError }}</div>
    <div v-else-if="r?.warning" class="warn">{{ r.warning }}</div>
    <div v-if="r && !r.files.length" class="hint">
      {{ off ? 'Mentor is off, so it does not follow the agent.' : 'Nothing the agent changed differs from HEAD.' }}
    </div>

    <Section v-if="r?.files.length" :title="`Files the agent changed · +${r.lines[0]} −${r.lines[1]}`" :count="r.files.length" :open="false">
    <div v-for="f in r?.files" :key="f.path" class="file" :class="{ done: reviewed(f) }">
      <div class="frow" :class="{ gone: f.status === 'D' }" :title="(f.status === 'A' ? 'New: ' : f.status === 'D' ? 'Deleted: ' : 'Beside HEAD: ') + rel(f.path)" @click="openFile(f)">
        <i class="codicon" :class="f.status === 'A' ? 'codicon-diff-added' : f.status === 'D' ? 'codicon-diff-removed' : 'codicon-diff'" />
        <span class="fname">{{ base(f.path) }}</span>
        <span v-if="f.status === 'A'" class="tag new">new</span>
        <span v-else-if="f.status === 'D'" class="tag gone">deleted</span>
        <span class="fdir">{{ dir(f.path) }}</span>
        <span class="nums"><span class="add">+{{ f.added }}</span> <span class="del">−{{ f.deleted }}</span></span>
        <button v-if="live(f).length" :class="{ ok: reviewed(f) }" :title="reviewed(f) ? 'Reviewed; click to unmark' : 'Mark reviewed'" @click.stop="markReviewed(live(f).map((c) => c.key), !reviewed(f))">
          <i class="codicon" :class="reviewed(f) ? 'codicon-pass-filled' : 'codicon-check'" />
        </button>
      </div>
      <div v-for="c in f.changes" :key="c.key + c.change" class="crow" :class="c.change" :title="rel(c.file) + ':' + c.line" @click="open(f, c)">
        <span class="mark">{{ mark[c.change] }}</span>
        <i :class="['codicon', 'codicon-' + (icons[c.kind] || 'symbol-misc')]" />
        <span class="cname">{{ name(c) }}</span>
        <span v-if="c.shapeChanged" class="tag" :title="(c.oldShape || '') + '\n→\n' + (c.shape || '')">signature</span>
        <span v-if="c.review === 'stale'" class="tag again" title="Changed again since you reviewed it">changed again</span>
        <button v-if="c.change !== 'removed'" class="info" title="Its card: who uses it, its tests" @click.stop="card(c)"><i class="codicon codicon-info" /></button>
      </div>
    </div>
    </Section>
  </div>
</template>

<style scoped>
.shape {
  border-bottom: 1px solid var(--border);
  padding-bottom: 4px;
}
.changes {
  padding-bottom: 12px;
}
.head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  color: var(--text-dim);
  font-size: 12px;
}
.head button {
  margin-left: auto;
  color: var(--text-dim);
}
.dim {
  color: var(--text-faint);
  font-size: 11px;
}
.add {
  color: var(--success, #b8d27a);
}
.del {
  color: var(--error);
}
.hint {
  padding: 8px;
  color: var(--text-dim);
  line-height: 1.45;
}
.warn {
  margin: 0 8px 6px;
  font-size: 11px;
  color: var(--warning);
}
.file {
  margin-bottom: 2px;
}
.file.done {
  opacity: 0.6;
}
.frow,
.crow {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  cursor: pointer;
  white-space: nowrap;
}
.frow {
  padding: 3px 6px 3px 8px;
}
.crow {
  padding: 1px 6px 1px 26px;
  font-size: 12px;
}
.frow:hover,
.crow:hover {
  background: var(--bg-hover);
}
.fname {
  font-weight: 500;
}
.fdir {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-faint);
  font-size: 11px;
}
.nums {
  margin-left: auto;
  font-size: 11px;
}
.frow button,
.crow button {
  color: var(--text-dim);
}
.frow button:hover,
.crow button:hover,
.frow button.ok {
  color: var(--success, #b8d27a);
}
.crow .info {
  margin-left: auto;
  visibility: hidden;
}
.crow:hover .info {
  visibility: visible;
}
.mark {
  width: 10px;
  text-align: center;
  font-family: var(--font-mono, monospace);
}
.crow.added .mark {
  color: var(--success, #b8d27a);
}
.crow.modified .mark {
  color: var(--warning);
}
.crow.removed .mark,
.crow.removed .cname {
  color: var(--error);
}
.crow.removed .cname {
  text-decoration: line-through;
}
.cname {
  overflow: hidden;
  text-overflow: ellipsis;
}
.tag {
  padding: 0 4px;
  border-radius: 3px;
  background: var(--bg-hover);
  color: var(--text-dim);
  font-size: 10px;
}
.tag.again {
  color: var(--warning);
}
.tag.new {
  color: var(--success, #b8d27a);
}
.tag.gone {
  color: var(--error);
}
.frow.gone {
  cursor: default;
}
.frow.gone .fname {
  text-decoration: line-through;
}
</style>
