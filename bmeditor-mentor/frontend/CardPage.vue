<script setup lang="ts">
import { computed } from 'vue'
import { api, go, icons, openAt, plural, rel, type Card } from './api'
import RefRow from './RefRow.vue'
import Section from './Section.vue'

// A symbol's card: what it is for, where it sits, what it is made of, who
// makes it, calls it and tests it, real code that uses it, and how far a
// change to it reaches. Every row is a place in the code.
const props = defineProps<{ card: Card }>()
const s = computed(() => props.card.symbol)
const usedBy = computed(() => props.card.usedBy.reduce((n, g) => n + g.refs.length, 0))
const tests = computed(() => [...props.card.tests, ...props.card.indirectTests])
const callable = computed(() => s.value.kind === 'func' || s.value.kind === 'method')
const pkgDir = computed(() => s.value.pkg.replace(/_test$/, ''))
</script>

<template>
  <div class="card">
    <div class="title">
      <i :class="['codicon', 'codicon-' + (icons[s.kind] || 'symbol-misc')]" />
      <span class="name">{{ s.recv ? s.recv + '.' : '' }}{{ s.name }}</span>
      <span class="kind">{{ s.kind }} in {{ s.pkgName }}</span>
    </div>
    <p class="what">
      {{ card.responsibility }}
      <span v-if="card.derived" class="from" title="There is no doc comment: this is what the code shows">from the code</span>
    </p>
    <div class="actions">
      <button @click="openAt(s.file, s.line, s.col)"><i class="codicon codicon-go-to-file" /> Code</button>
      <button v-if="callable" @click="go({ kind: 'flow', key: s.key })"><i class="codicon codicon-call-outgoing" /> Flow</button>
      <button @click="go({ kind: 'map', focus: pkgDir })"><i class="codicon codicon-type-hierarchy" /> Package</button>
    </div>
    <div class="where">{{ rel(s.file) }}:{{ s.line }}</div>
    <component :is="api.components.CodeView" v-if="s.sig && s.kind !== 'struct' && s.kind !== 'interface'" class="sig" :text="s.sig" language="plain" />

    <div class="impact">
      Changing it can reach {{ plural(card.impact.symbols, 'symbol') }}{{ card.impact.capped ? ' or more' : '' }}
      in {{ plural(card.impact.packages.length, 'package') }}, and {{ plural(card.impact.tests, 'test') }}.
    </div>

    <Section v-if="card.owner" title="Belongs to">
      <RefRow :r="card.owner" />
    </Section>
    <Section title="Fields" :count="card.fields.length">
      <RefRow v-for="f in card.fields" :key="f.key" :r="f" :detail="(s.fields || []).find((x) => x.name === f.name)?.type" />
    </Section>
    <Section title="Methods" :count="card.methods.length">
      <RefRow v-for="m in card.methods" :key="m.key" :r="m" detail="" />
    </Section>
    <Section title="Routes" :count="card.routes.length">
      <div v-for="r in card.routes" :key="r.file + r.line" class="row" @click="openAt(r.file, r.line)">
        <i class="codicon codicon-globe" /> {{ r.method ? r.method + ' ' : '' }}{{ r.path }}
        <span class="dim">{{ r.via }}, guessed from the call</span>
      </div>
    </Section>
    <Section title="Implements" :count="card.implements.length">
      <RefRow v-for="r in card.implements" :key="r.key" :r="r" />
    </Section>
    <Section title="Implemented by" :count="card.implementedBy.length">
      <RefRow v-for="r in card.implementedBy" :key="r.key" :r="r" />
    </Section>
    <Section title="Embeds" :count="card.embeds.length">
      <RefRow v-for="r in card.embeds" :key="r.key" :r="r" />
    </Section>
    <Section title="Made by" :count="card.constructors.length">
      <RefRow v-for="r in card.constructors" :key="r.key" :r="r" site :detail="r.via === 'returns' ? 'returns it' : 'builds it'" />
    </Section>
    <Section title="Called by" :count="card.callers.length">
      <RefRow v-for="r in card.callers" :key="r.key" :r="r" site :detail="r.via?.startsWith('through') ? r.via : r.pkgName" />
    </Section>
    <Section title="Calls" :count="card.callees.length" :open="false">
      <RefRow v-for="r in card.callees" :key="r.key" :r="r" site />
    </Section>
    <Section title="Used by" :count="usedBy" :open="false">
      <template v-for="g in card.usedBy" :key="g.pkg">
        <div class="group">{{ g.pkg }}</div>
        <RefRow v-for="r in g.refs" :key="r.key" :r="r" site :detail="r.via" />
      </template>
    </Section>
    <Section title="How it is used" :count="card.usages.length">
      <div v-for="u in card.usages" :key="u.file + u.line" class="usage">
        <div class="row" @click="openAt(u.file, u.line)">
          <span>{{ u.in }}</span> <span class="dim">{{ rel(u.file) }}:{{ u.line }}</span>
        </div>
        <component :is="api.components.CodeView" :text="u.text" language="plain" />
      </div>
    </Section>
    <Section title="Tests" :count="tests.length">
      <RefRow v-for="r in tests" :key="r.key" :r="r" site :detail="r.via?.startsWith('through') ? r.via : 'directly'" />
    </Section>
  </div>
</template>

<style scoped>
.card {
  padding-bottom: 12px;
}
.title {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 8px 8px 2px;
}
.title .codicon {
  align-self: center;
  color: var(--accent);
}
.name {
  font-weight: 600;
  word-break: break-all;
}
.kind {
  color: var(--text-faint);
  font-size: 11px;
  white-space: nowrap;
}
.what {
  margin: 2px 8px 6px;
  line-height: 1.45;
}
.from {
  font-size: 10px;
  padding: 0 4px;
  border-radius: 3px;
  border: 1px dashed var(--border-strong);
  color: var(--text-faint);
}
.actions {
  display: flex;
  gap: 4px;
  padding: 0 8px 4px;
}
.actions button {
  display: flex;
  align-items: center;
  gap: 3px;
  padding: 2px 7px;
  border-radius: 4px;
  background: var(--bg-raised);
  border: 1px solid var(--border);
  font-size: 12px;
}
.actions button:hover {
  border-color: var(--accent);
}
.where {
  padding: 0 8px 4px;
  color: var(--text-faint);
  font-size: 11px;
}
.sig {
  margin: 0 8px 4px;
}
.impact {
  margin: 2px 8px 4px;
  padding: 5px 7px;
  border-radius: 4px;
  background: var(--bg-raised);
  color: var(--text-dim);
  font-size: 12px;
}
.group {
  padding: 4px 8px 0 14px;
  font-size: 11px;
  color: var(--text-faint);
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
.dim {
  color: var(--text-faint);
  font-size: 11px;
}
.usage {
  margin-bottom: 6px;
}
.usage :deep(*) {
  font-size: 12px;
}
</style>
