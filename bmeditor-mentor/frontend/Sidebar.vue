<script setup lang="ts">
import { computed, ref } from 'vue'
import { api, ask, back, call, go, home, page, plural, store, type Status } from './api'
import CardPage from './CardPage.vue'
import ChangesView from './ChangesView.vue'
import FlowPage from './FlowPage.vue'
import MapPage from './MapPage.vue'
import Results from './Results.vue'

// Mentor's one view. Home is the agent's work: its plan or milestone, the
// project's shape with what it did, and its files. At the bottom, one box
// for a question about the project, a part of it or something to do to it;
// the answer, a card, a flow or a map opens in place, and Back goes to the
// page before.
const input = ref<HTMLInputElement>()
const status = computed(() => store.status)
const p = computed(() => page())

function line(st: Status | null): string {
  if (!api.ide.folder) return 'Open a folder.'
  if (!st) return ''
  switch (st.state) {
    case 'indexing':
      return st.detail || 'Reading the code…'
    case 'stale':
      return 'Files changed; reading them again…'
    case 'error':
      return st.detail || 'The code could not be read.'
  }
  let s = `${plural(st.packages, 'package')}, ${plural(st.symbols, 'symbol')}`
  if (st.head.branch) s += ` · ${st.head.branch}`
  if (st.semantic === 'on') s += ' · semantic'
  else if (st.semantic === 'building') s += ' · embedding…'
  return s
}

async function reindex() {
  store.status = await call<Status>('reindex')
}

defineExpose({ focus: () => input.value?.focus() })
</script>

<template>
  <div class="mentor">
    <div class="status" :class="status?.state" :title="status?.errors?.join('\n')">
      <i v-if="status?.state === 'indexing' || status?.state === 'stale'" class="codicon codicon-loading codicon-modifier-spin" />
      <i v-else-if="status?.state === 'error' || status?.errors?.length" class="codicon codicon-warning" />
      <span>{{ line(status) }}</span>
      <button title="The map of the packages" @click="go({ kind: 'map', focus: '' })"><i class="codicon codicon-type-hierarchy" /></button>
      <button title="Read the code again" @click="reindex"><i class="codicon codicon-refresh" /></button>
    </div>
    <div v-if="p.kind !== 'home'" class="nav">
      <button @click="back"><i class="codicon codicon-arrow-left" /> Back</button>
      <button v-if="store.pages.length > 2" class="right" @click="home"><i class="codicon codicon-home" /> The agent's work</button>
    </div>
    <div class="body">
      <ChangesView v-if="p.kind === 'home'" />
      <div v-else-if="store.error" class="error">{{ store.error }}</div>
      <div v-else-if="store.loading" class="hint">Loading…</div>
      <Results v-else-if="p.kind === 'search'" />
      <CardPage v-else-if="p.kind === 'card' && p.card" :card="p.card" />
      <FlowPage v-else-if="p.kind === 'flow' && p.flow" :flow="p.flow" />
      <MapPage v-else-if="p.kind === 'map' && p.graph" :graph="p.graph" />
    </div>
    <form class="ask" @submit.prevent="ask(store.query)">
      <i class="codicon codicon-search" />
      <input ref="input" v-model="store.query" class="q" placeholder="Ask about the project: authentication, add a provider…" spellcheck="false" />
    </form>
  </div>
</template>

<style scoped>
.mentor {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.tabs {
  display: flex;
  flex: none;
  gap: 2px;
  padding: 4px 6px 0;
  border-bottom: 1px solid var(--border);
}
.tabs button {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-bottom: 2px solid transparent;
  color: var(--text-dim);
  font-size: 12px;
}
.tabs button.on {
  color: var(--text);
  border-bottom-color: var(--accent);
}
.badge.wait {
  background: var(--accent);
  color: var(--bg);
}
.badge {
  padding: 0 5px;
  border-radius: 8px;
  background: var(--accent-dim);
  color: var(--text);
  font-size: 10px;
}
.top {
  flex: none;
  padding: 8px 8px 4px;
  border-bottom: 1px solid var(--border);
}
form.ask {
  flex: none;
  margin: 6px 8px;
}
form {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 7px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg);
}
form:focus-within {
  border-color: var(--accent);
}
form .codicon {
  color: var(--text-faint);
}
.q {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  color: var(--text);
  font: inherit;
}
.status {
  flex: none;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 8px;
  border-bottom: 1px solid var(--border);
  font-size: 11px;
  color: var(--text-faint);
  white-space: nowrap;
}
.status span {
  overflow: hidden;
  text-overflow: ellipsis;
}
.status.error,
.status .codicon-warning {
  color: var(--warning);
}
.status button {
  color: var(--text-dim);
}
.status button:first-of-type {
  margin-left: auto;
}
.tabs button {
  flex: 1;
  justify-content: center;
}
.status button:hover {
  color: var(--accent);
}
.nav {
  flex: none;
  padding: 3px 4px;
  border-bottom: 1px solid var(--border);
}
.nav button {
  display: flex;
  align-items: center;
  gap: 3px;
  padding: 1px 5px;
  color: var(--text-dim);
  font-size: 12px;
}
.nav {
  display: flex;
}
.nav .right {
  margin-left: auto;
}
.nav button:hover {
  color: var(--text);
}
.body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.error {
  margin: 8px;
  color: var(--warning);
  line-height: 1.45;
}
.hint {
  padding: 10px 8px;
  color: var(--text-dim);
}
</style>
