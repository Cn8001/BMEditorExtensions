<script setup lang="ts">
import { diff, go, icons, markReviewed, name, openAt, plural, rel, type Change, type Unit } from './api'
import RefRow from './RefRow.vue'
import Section from './Section.vue'

// One unit of change: declarations that change together, with its risks,
// where it is wired into the code that did not change, what it relies on,
// the routes and tests that reach it, and Diff, Flow and Reviewed.
defineProps<{ u: Unit; noReview?: boolean }>()

const mark: Record<string, string> = { added: '+', modified: '~', removed: '−' }

function reviewed(u: Unit) {
  const live = u.changes.filter((c) => c.change !== 'removed')
  return live.length > 0 && live.every((c) => c.review === 'reviewed')
}

function keys(u: Unit) {
  return u.changes.filter((c) => c.change !== 'removed').map((c) => c.key)
}

function show(c: Change) {
  if (c.change === 'removed') return
  void go({ kind: 'card', key: c.key })
}
</script>

<template>
  <div class="unit" :class="{ done: reviewed(u) }">
    <div class="title">
      <span v-if="u.layer" class="layer" title="Guessed from the package name">{{ u.layer }}</span>
      <span class="pkg">{{ u.pkgName }}</span>
      <span class="name">{{ u.title }}</span>
      <i v-if="reviewed(u)" class="codicon codicon-pass-filled ok" title="Reviewed as it is now" />
    </div>
    <div v-for="risk in u.risks" :key="risk" class="risk"><i class="codicon codicon-warning" /> {{ risk }}</div>

    <div v-for="c in u.changes" :key="c.key + c.change" class="change" :class="[c.change, { removed: c.change === 'removed' }]" @click="show(c)">
      <span class="mark">{{ mark[c.change] }}</span>
      <i :class="['codicon', 'codicon-' + (icons[c.kind] || 'symbol-misc')]" />
      <span class="cname" :title="c.shape || c.oldShape">{{ name(c) }}</span>
      <span v-if="c.shapeChanged" class="tag" :title="(c.oldShape || '') + '\n→\n' + (c.shape || '')">signature</span>
      <span v-if="c.test" class="tag">test</span>
      <span v-if="c.review === 'stale'" class="stale" title="Changed again since you reviewed it">changed again</span>
      <button class="open" :title="rel(c.file) + ':' + c.line" @click.stop="c.change === 'removed' ? diff(c.file) : openAt(c.file, c.line)">
        <i class="codicon codicon-go-to-file" />
      </button>
    </div>

    <Section title="Wired in at" :count="u.plugsIn.length" :open="u.plugsIn.length <= 4">
      <RefRow v-for="x in u.plugsIn" :key="x.key" :r="x" site :detail="x.via === 'calls_dynamic' ? 'through an interface' : x.via" />
    </Section>
    <Section title="Relies on" :count="u.reliesOn.length" :open="false">
      <RefRow v-for="x in u.reliesOn" :key="x.key" :r="x" site :detail="x.via" />
    </Section>
    <Section title="Routes" :count="u.routes.length">
      <div v-for="rt in u.routes" :key="rt.file + rt.line" class="row" @click="openAt(rt.file, rt.line)">
        <i class="codicon codicon-globe" /> {{ rt.method ? rt.method + ' ' : '' }}{{ rt.path }}
      </div>
    </Section>
    <Section title="Tests that reach it" :count="u.tests.length" :open="false">
      <RefRow v-for="x in u.tests" :key="x.key" :r="x" />
    </Section>
    <div class="impact">
      Reaches {{ plural(u.impact.symbols, 'symbol') }}{{ u.impact.capped ? '+' : '' }} in {{ plural(u.impact.packages.length, 'package') }}.
    </div>
    <div class="actions">
      <button v-for="f in u.files.filter((f) => !f.endsWith('_test.go')).slice(0, 3)" :key="f" :title="'Beside HEAD: ' + rel(f)" @click="diff(f)">
        <i class="codicon codicon-diff" /> {{ f.slice(f.lastIndexOf('/') + 1) }}
      </button>
      <button v-if="u.flows.length" title="The path a call takes through it" @click="go({ kind: 'flow', key: u.flows[0] })"><i class="codicon codicon-call-outgoing" /> Flow</button>
      <template v-if="!noReview">
        <button v-if="!reviewed(u)" class="primary" @click="markReviewed(keys(u))"><i class="codicon codicon-check" /> Reviewed</button>
        <button v-else @click="markReviewed(keys(u), false)">Not reviewed</button>
      </template>
    </div>
  </div>
</template>

<style scoped>
.unit {
  margin: 0 6px 8px;
  padding: 6px 0 4px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-raised);
}
.unit.done {
  opacity: 0.75;
}
.title {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 8px 4px;
  min-width: 0;
}
.layer {
  flex: none;
  font-size: 10px;
  padding: 0 4px;
  border-radius: 3px;
  border: 1px dashed var(--border-strong);
  color: var(--text-dim);
}
.pkg {
  flex: none;
  color: var(--text-faint);
  font-size: 11px;
}
.name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ok {
  margin-left: auto;
  color: var(--success);
}
.risk {
  display: flex;
  gap: 4px;
  padding: 1px 8px;
  font-size: 12px;
  color: var(--warning);
  line-height: 1.4;
}
.risk .codicon {
  flex: none;
  margin-top: 2px;
  font-size: 12px;
}
.change {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 1px 8px;
  cursor: pointer;
  white-space: nowrap;
}
.change:hover {
  background: var(--bg-hover);
}
.change.removed {
  cursor: default;
  color: var(--text-faint);
  text-decoration: line-through;
}
.mark {
  width: 10px;
  font-family: var(--font-mono);
  text-align: center;
}
.added .mark {
  color: var(--success);
}
.modified .mark {
  color: var(--warning);
}
.removed .mark {
  color: var(--error);
}
.change .codicon {
  font-size: 14px;
  color: var(--text-dim);
}
.cname {
  overflow: hidden;
  text-overflow: ellipsis;
}
.tag {
  font-size: 10px;
  padding: 0 4px;
  border-radius: 3px;
  border: 1px solid var(--border-strong);
  color: var(--text-dim);
}
.stale {
  font-size: 10px;
  color: var(--warning);
}
.open {
  margin-left: auto;
  visibility: hidden;
  color: var(--text-dim);
}
.change:hover .open {
  visibility: visible;
}
.impact {
  padding: 4px 8px 2px;
  font-size: 11px;
  color: var(--text-faint);
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 4px 8px 2px;
}
.actions button {
  display: flex;
  align-items: center;
  gap: 3px;
  padding: 1px 6px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--bg);
  font-size: 11px;
}
.actions button:hover {
  border-color: var(--accent);
}
.actions .primary {
  margin-left: auto;
  border-color: var(--accent);
  color: var(--accent);
}
.row {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px 2px 14px;
  cursor: pointer;
}
.row:hover {
  background: var(--bg-hover);
}
</style>
