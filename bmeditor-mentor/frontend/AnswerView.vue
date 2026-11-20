<script setup lang="ts">
import { computed } from 'vue'
import { go, name, openAt, rel, store, type Ref } from './api'

// A part of the project, or something to do to it, answered from what the
// code says for certain, in six questions: where it starts, what runs,
// where its data lives, how it fails, how to extend it, and its tests and
// where to stop. Every line is a place in the code, a click away; nothing
// here is written by a model.
const a = computed(() => store.task)
const f = computed(() => a.value?.feature)
const p = computed(() => a.value?.point)

function at(r: Ref) {
  void openAt(r.siteFile || r.file, r.siteLine || r.line)
}

function where(r: Ref) {
  return `${rel(r.siteFile || r.file)}:${r.siteLine || r.line}`
}

function sig(r: Ref) {
  const s = r.sig || ''
  const i = s.indexOf(').')
  return s.startsWith('func (') && i > 0 ? s.slice(i + 2) : s.replace(/^func /, '')
}

// Each key once, in the order they come: the data and the failures of a
// path list one function once.
function once<T extends Ref>(xs: T[]): T[] {
  const seen = new Set<string>()
  return xs.filter((x) => !seen.has(x.key) && seen.add(x.key))
}

const tests = computed(() => once([...(f.value?.tests ?? []), ...(p.value?.tests ?? [])]))
const stops = computed(() => {
  const all = [...(f.value?.breakpoints ?? []), ...(p.value?.breakpoints ?? [])]
  const seen = new Set<string>()
  return all.filter((b) => {
    const k = where(b)
    return !seen.has(k) && seen.add(k)
  })
})

// The questions, numbered as they are answered.
const qs = computed(() => {
  const out: string[] = []
  if (f.value) out.push('start', 'flow', 'data', 'errors')
  if (p.value) out.push('extend')
  if (tests.value.length || stops.value.length) out.push('tests')
  return out
})
const n = (q: string) => qs.value.indexOf(q) + 1
</script>

<template>
  <div v-if="a && (f || p)" class="answer">
    <template v-if="f">
      <h4>{{ n('start') }}. Where it starts</h4>
      <div v-if="f.entry" class="row" @click="openAt(f.entry.file, f.entry.line)">
        <i class="codicon" :class="f.entry.kind === 'route' ? 'codicon-globe' : 'codicon-play'" />
        <b>{{ f.entry.label }}</b> <span class="dim">→ {{ name(f.entry.ref) }} · {{ rel(f.entry.file) }}:{{ f.entry.line }}</span>
      </div>
      <div v-if="f.note" class="dim small">{{ f.note }}</div>
      <div v-for="e in f.others" :key="e.file + e.line" class="row small" @click="openAt(e.file, e.line)">
        <span class="dim">also</span> {{ e.label }} <span class="dim">→ {{ name(e.ref) }}</span>
      </div>

      <h4>{{ n('flow') }}. What runs, in order</h4>
      <div
        v-for="(s, i) in f.flow"
        :key="i"
        class="step"
        :class="{ seed: s.seed }"
        :style="{ paddingLeft: 4 + s.depth * 12 + 'px' }"
        :title="where(s) + (s.dynamic ? ' · through an interface: one of these runs' : '')"
        @click="openAt(s.file, s.line)"
      >
        <span class="fn">{{ name(s) }}</span>
        <span v-if="s.dynamic" class="tag">one of</span>
        <span v-if="s.layer" class="layer">{{ s.layer }}</span>
        <span v-if="s.errReturns" class="err" title="Returns an error here">{{ s.errReturns }}✕</span>
      </div>

      <h4>{{ n('data') }}. Where its data lives</h4>
      <div v-if="!f.data.length" class="dim small">Nothing on the path reads or writes a store.</div>
      <div v-for="s in once(f.data)" :key="s.key" class="row" @click="openAt(s.file, s.line)">
        <i class="codicon codicon-database" /> {{ name(s) }} <span v-if="s.dynamic" class="dim">· an implementation of it</span>
        <span class="dim right">{{ rel(s.file) }}</span>
      </div>

      <h4>{{ n('errors') }}. How it fails</h4>
      <div v-if="!f.errors.length" class="dim small">Nothing on the path returns an error.</div>
      <div v-for="s in once(f.errors)" :key="s.key" class="row" @click="openAt(s.file, s.line)">
        <i class="codicon codicon-error" /> {{ name(s) }}
        <span class="dim">· {{ s.errReturns }} way{{ s.errReturns === 1 ? '' : 's' }} to fail{{ s.branches ? ', ' + s.branches + ' branch' + (s.branches === 1 ? '' : 'es') : '' }}</span>
      </div>
    </template>

    <template v-if="p">
      <h4>{{ n('extend') }}. To add one, implement <a @click="openAt(p.interface.file, p.interface.line)">{{ p.interface.name }}</a></h4>
      <div class="dim small">{{ p.doc }} Chosen because {{ p.why }}.</div>
      <div v-for="m in p.methods" :key="m.key" class="row code" @click="openAt(m.file, m.line)">{{ sig(m) }}</div>
      <div v-for="im in p.impls" :key="im.key" class="impl">
        <div class="row" @click="openAt(im.file, im.line)">
          <b>{{ im.name }}</b>
          <span v-if="im.template" class="tag" title="Tested and used: the one to start a new one from">start from this</span>
          <span class="dim">{{ im.fields.join(', ') }}</span>
          <button class="peek" title="Its card" @click.stop="go({ kind: 'card', key: im.key })"><i class="codicon codicon-info" /></button>
        </div>
        <div v-for="x in im.notes" :key="x" class="note">{{ x }}</div>
      </div>
      <ol class="todo">
        <li v-for="(s, i) in p.steps" :key="i" :class="{ link: !!s.at }" @click="s.at && at(s.at)">
          {{ s.text }} <span v-if="s.at" class="dim">{{ where(s.at) }}</span>
        </li>
      </ol>
    </template>

    <template v-if="tests.length || stops.length">
      <h4>{{ n('tests') }}. Its tests, and where to stop</h4>
      <div v-for="t in tests" :key="t.key" class="row" @click="at(t)">
        <i class="codicon codicon-beaker" /> {{ t.name }} <span class="dim">{{ t.via }}</span>
      </div>
      <div v-for="b in stops" :key="where(b)" class="row" @click="at(b)">
        <i class="codicon codicon-debug-breakpoint" /> {{ where(b) }} <span class="dim">{{ b.via }}</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.answer {
  padding: 2px 10px 12px;
  border-bottom: 1px solid var(--border-strong);
  background: var(--bg-raised);
  line-height: 1.45;
}
h4 {
  margin: 10px 0 3px;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-dim);
}
a {
  color: var(--accent);
  cursor: pointer;
}
.row,
.step {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 1px 4px;
  border-radius: 3px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
}
.row:hover,
.step:hover {
  background: var(--bg-hover);
}
.step .fn {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text);
}
.step.seed .fn {
  color: var(--accent);
  font-weight: 600;
}
.layer {
  font-size: 9px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}
.err {
  font-size: 10px;
  color: var(--error);
}
.tag {
  padding: 0 5px;
  border-radius: 3px;
  background: var(--accent-dim, rgba(239, 227, 203, 0.12));
  color: var(--accent);
  font-size: 10px;
}
.code {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text);
}
.impl + .impl {
  margin-top: 3px;
}
.note {
  padding-left: 14px;
  font-size: 11px;
  color: var(--text-dim);
}
.peek {
  margin-left: auto;
  color: var(--text-faint);
}
.todo {
  margin: 4px 0 0;
  padding-left: 18px;
}
.todo li.link {
  cursor: pointer;
}
.todo li.link:hover {
  color: var(--text);
}
.dim {
  color: var(--text-faint);
}
.right {
  margin-left: auto;
  font-size: 11px;
}
.small {
  font-size: 11px;
}
</style>
