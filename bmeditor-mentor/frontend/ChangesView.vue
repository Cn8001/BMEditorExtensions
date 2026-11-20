<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { diff, go, icons, loadChanges, markReviewed, name, openAt, rel, store, type AgentFile, type Change } from './api'
import AgentCard from './AgentCard.vue'

// What the agent changed that still differs from HEAD, file by file, flat:
// a file opens beside HEAD, a declaration opens where it is. The check
// marks a file's declarations reviewed as they are now; a later change
// shows again. A file leaves the list once it is committed or put back.
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

function open(c: Change) {
  if (c.change === 'removed') void diff(c.file)
  else void openAt(c.file, c.line)
}

function card(c: Change) {
  if (c.change !== 'removed') void go({ kind: 'card', key: c.key })
}
</script>

<template>
  <div class="changes">
    <AgentCard />
    <div class="head">
      <span>Agent's changes</span>
      <span v-if="r?.files.length" class="dim">
        {{ r.files.length }} file{{ r.files.length === 1 ? '' : 's' }} · <span class="add">+{{ r.lines[0] }}</span> <span class="del">−{{ r.lines[1] }}</span>
      </span>
      <button title="Read the changes again" @click="loadChanges"><i class="codicon codicon-refresh" :class="{ 'codicon-modifier-spin': store.reportBusy }" /></button>
    </div>
    <div v-if="store.reportError" class="hint">{{ store.reportError }}</div>
    <div v-else-if="r?.warning" class="warn">{{ r.warning }}</div>
    <div v-if="r && !r.files.length" class="hint">
      {{ off ? 'Mentor is off, so it does not follow the agent.' : 'Nothing the agent changed differs from HEAD.' }}
    </div>

    <div v-for="f in r?.files" :key="f.path" class="file" :class="{ done: reviewed(f) }">
      <div class="frow" :title="'Beside HEAD: ' + rel(f.path)" @click="diff(f.path)">
        <i class="codicon codicon-diff" />
        <span class="fname">{{ base(f.path) }}</span>
        <span class="fdir">{{ dir(f.path) }}</span>
        <span class="nums"><span class="add">+{{ f.added }}</span> <span class="del">−{{ f.deleted }}</span></span>
        <button v-if="live(f).length" :class="{ ok: reviewed(f) }" :title="reviewed(f) ? 'Reviewed; click to unmark' : 'Mark reviewed'" @click.stop="markReviewed(live(f).map((c) => c.key), !reviewed(f))">
          <i class="codicon" :class="reviewed(f) ? 'codicon-pass-filled' : 'codicon-check'" />
        </button>
      </div>
      <div v-for="c in f.changes" :key="c.key + c.change" class="crow" :class="c.change" :title="rel(c.file) + ':' + c.line" @click="open(c)">
        <span class="mark">{{ mark[c.change] }}</span>
        <i :class="['codicon', 'codicon-' + (icons[c.kind] || 'symbol-misc')]" />
        <span class="cname">{{ name(c) }}</span>
        <span v-if="c.shapeChanged" class="tag" :title="(c.oldShape || '') + '\n→\n' + (c.shape || '')">signature</span>
        <span v-if="c.review === 'stale'" class="tag again" title="Changed again since you reviewed it">changed again</span>
        <button v-if="c.change !== 'removed'" class="info" title="Its card: who uses it, its tests" @click.stop="card(c)"><i class="codicon codicon-info" /></button>
      </div>
    </div>
  </div>
</template>

<style scoped>
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
  color: var(--success);
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
  color: var(--success);
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
  color: var(--success);
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
</style>
