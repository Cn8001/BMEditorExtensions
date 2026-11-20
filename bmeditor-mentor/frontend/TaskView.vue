<script setup lang="ts">
import { computed } from 'vue'
import { go, name, openAt, rel, store, type Ref } from './api'

// The answer to a task, such as "add a new authentication provider", from
// what the code says for certain: the interface to implement, the types
// that already do and how they differ, how it is wired in, what to change,
// the tests and where to stop. Each line is a place in the code, a click
// away; nothing here is written by a model.
const p = computed(() => store.task?.point)

function at(r: Ref) {
  void openAt(r.siteFile || r.file, r.siteLine || r.line)
}

function where(r: Ref) {
  const f = r.siteFile || r.file
  return `${rel(f)}:${r.siteLine || r.line}`
}

function sig(r: Ref) {
  const s = r.sig || ''
  const i = s.indexOf(').')
  return s.startsWith('func (') && i > 0 ? s.slice(i + 2) : s.replace(/^func /, '')
}

// The path a request takes: route, the functions that reach the
// consumers, the consumers, then what they call.
const path = computed(() => {
  const x = p.value
  if (!x) return []
  const out: { label: string; ref?: Ref; kind: string }[] = []
  for (const r of x.routes) out.push({ label: `${r.method ? r.method + ' ' : ''}${r.path}`, kind: 'route', ref: { key: r.handler || '', name: r.path, kind: 'route', pkg: '', pkgName: '', file: r.file, line: r.line, col: 1 } })
  for (const r of x.upstream) out.push({ label: name(r), ref: r, kind: 'up' })
  const consumers = [...new Map(x.consumers.map((r) => [r.key, r])).values()]
  for (const r of consumers) out.push({ label: name(r), ref: r, kind: 'here' })
  out.push({ label: x.interface.name, ref: x.interface, kind: 'point' })
  for (const r of x.downstream) out.push({ label: name(r), ref: r, kind: 'down' })
  return out
})
</script>

<template>
  <div v-if="p" class="task">
    <div class="title">
      <i class="codicon codicon-lightbulb" />
      <span>To do it, implement <a @click="openAt(p.interface.file, p.interface.line)">{{ p.interface.name }}</a></span>
    </div>
    <div class="why">{{ p.doc || '' }} <span class="dim">Chosen because {{ p.why }}.</span></div>

    <h4>What it must have</h4>
    <div v-for="m in p.methods" :key="m.key" class="row code" @click="openAt(m.file, m.line)">{{ sig(m) }}</div>

    <h4>Already done by</h4>
    <div v-for="im in p.impls" :key="im.key" class="impl">
      <div class="row" @click="openAt(im.file, im.line)">
        <b>{{ im.name }}</b>
        <span v-if="im.template" class="tag" title="Tested and used: the one to start a new one from">start from this</span>
        <span class="dim">{{ im.fields.join(', ') }}</span>
        <button class="peek" title="Its card" @click.stop="go({ kind: 'card', key: im.key })"><i class="codicon codicon-info" /></button>
      </div>
      <div v-for="n in im.notes" :key="n" class="note">{{ n }}</div>
    </div>
    <div v-if="p.fakes.length" class="dim small">In tests: {{ p.fakes.map((f) => f.name).join(', ') }}</div>

    <h4>How it is wired in</h4>
    <div class="path">
      <template v-for="(s, i) in path" :key="i + s.label">
        <span v-if="i" class="arrow">→</span>
        <a :class="s.kind" :title="s.ref ? where(s.ref) : ''" @click="s.ref && at(s.ref)">{{ s.label }}</a>
      </template>
    </div>
    <div v-for="h in p.holders" :key="'h' + h.key" class="row small" @click="at(h)">
      <span class="dim">{{ name(h) }}</span> {{ h.via }}
    </div>

    <h4>What to change</h4>
    <ol class="steps">
      <li v-for="(s, i) in p.steps" :key="i" :class="{ link: !!s.at }" @click="s.at && at(s.at)">
        {{ s.text }} <span v-if="s.at" class="dim">{{ where(s.at) }}</span>
      </li>
    </ol>

    <h4>Tests</h4>
    <div v-if="!p.tests.length" class="dim small">No test reaches it yet.</div>
    <div v-for="t in p.tests" :key="t.key" class="row" @click="at(t)">
      <i class="codicon codicon-beaker" /> {{ t.name }} <span class="dim">{{ t.via }} · {{ where(t) }}</span>
    </div>

    <h4>Where to stop when it breaks</h4>
    <div v-for="b in p.breakpoints" :key="where(b)" class="row" @click="at(b)">
      <i class="codicon codicon-debug-breakpoint" /> {{ where(b) }} <span class="dim">{{ b.via }}</span>
    </div>
  </div>
</template>

<style scoped>
.task {
  padding: 8px 10px 12px;
  border-bottom: 1px solid var(--border-strong);
  background: var(--bg-raised);
  line-height: 1.45;
}
.title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text);
}
.title .codicon {
  color: var(--accent);
}
a {
  color: var(--accent);
  cursor: pointer;
}
a:hover {
  text-decoration: underline;
}
.why {
  margin: 3px 0 2px;
  color: var(--text-dim);
}
h4 {
  margin: 10px 0 3px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}
.row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 1px 4px;
  border-radius: 3px;
  cursor: pointer;
}
.row:hover {
  background: var(--bg-hover);
}
.code {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text);
}
.impl + .impl {
  margin-top: 4px;
}
.note {
  padding-left: 14px;
  font-size: 11px;
  color: var(--text-dim);
}
.tag {
  padding: 0 5px;
  border-radius: 3px;
  background: var(--accent-dim, rgba(239, 227, 203, 0.12));
  color: var(--accent);
  font-size: 10px;
}
.peek {
  margin-left: auto;
  color: var(--text-faint);
}
.path {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 3px 5px;
  font-family: var(--font-mono);
  font-size: 12px;
}
.path .route {
  color: var(--info, #9fc4d0);
}
.path .point {
  font-weight: 600;
}
.arrow {
  color: var(--text-faint);
}
.steps {
  margin: 0;
  padding-left: 18px;
}
.steps li {
  padding: 1px 0;
}
.steps li.link {
  cursor: pointer;
}
.steps li.link:hover {
  color: var(--text);
}
.dim {
  color: var(--text-faint);
}
.small {
  font-size: 11px;
}
</style>
