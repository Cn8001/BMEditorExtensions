<script setup lang="ts">
import { go, icons, name, openAt, plural, rel, store } from './api'
import TaskView from './TaskView.vue'

// The answer to a question: the types and functions that may be what was
// asked for, best first, each with what it is for, why it matched, how
// much it is used, and how it differs from the others listed.
</script>

<template>
  <div class="results">
    <div v-if="store.searching" class="hint">Looking…</div>
    <template v-else-if="store.result">
      <TaskView v-if="store.task" />
      <div v-if="store.task && store.result.candidates.length" class="hint small">The types and functions that match it:</div>
      <div v-if="!store.result.candidates.length" class="hint">Nothing here matches {{ store.result.words.join(', ') }}.</div>
      <div v-for="(c, i) in store.result.candidates" :key="c.key" class="cand" :class="{ first: i === 0 }" @click="go({ kind: 'card', key: c.key })">
        <div class="head">
          <i :class="['codicon', 'codicon-' + (icons[c.kind] || 'symbol-misc')]" />
          <span class="name">{{ name(c) }}</span>
          <span class="dim">{{ c.kind }} · {{ c.pkgName }}</span>
          <button class="open" :title="'Go to ' + rel(c.file) + ':' + c.line" @click.stop="openAt(c.file, c.line, c.col)">
            <i class="codicon codicon-go-to-file" />
          </button>
        </div>
        <div class="what">
          {{ c.responsibility }}<span v-if="c.derived" class="from" title="No doc comment: this is what the code shows"> (from the code)</span>
        </div>
        <div class="facts">
          <span>{{ plural(c.uses, 'use') }}{{ c.usedIn.length ? ' in ' + c.usedIn.join(', ') : '' }}</span>
          <span v-if="c.tests">· {{ plural(c.tests, 'test') }}</span>
          <span v-if="c.matched.length" class="why" :title="c.matched.join('\n')">· matched {{ c.matched.map((m) => m.split(':')[0]).join(', ') }}</span>
        </div>
        <div v-for="d in c.differs" :key="d" class="differs">{{ d }}</div>
      </div>
      <div v-if="store.result.expanded.length" class="hint small">Also looked for {{ store.result.expanded.join(', ') }}.</div>
    </template>
    <div v-else class="hint">
      Ask what you need in plain words: <em>which struct holds a user's login</em>, <em>where is the session made</em>, or what you want to do:
      <em>add a new authentication provider</em>.
      Or put the cursor in a declaration and run <strong>Mentor: Explain This</strong> (Ctrl+Alt+U).
    </div>
  </div>
</template>

<style scoped>
.cand {
  padding: 6px 8px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
}
.cand:hover {
  background: var(--bg-hover);
}
.head {
  display: flex;
  align-items: center;
  gap: 5px;
  white-space: nowrap;
}
.head .codicon {
  font-size: 14px;
  color: var(--accent);
}
.name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
}
.dim {
  color: var(--text-faint);
  font-size: 11px;
}
.open {
  margin-left: auto;
  visibility: hidden;
  color: var(--text-dim);
}
.cand:hover .open {
  visibility: visible;
}
.what {
  margin-top: 2px;
  line-height: 1.4;
}
.from {
  color: var(--text-faint);
}
.facts {
  margin-top: 2px;
  color: var(--text-faint);
  font-size: 11px;
}
.differs {
  margin-top: 2px;
  font-size: 11px;
  color: var(--text-dim);
}
.hint {
  padding: 10px 8px;
  color: var(--text-dim);
  line-height: 1.5;
}
.hint.small {
  font-size: 11px;
  color: var(--text-faint);
}
</style>
