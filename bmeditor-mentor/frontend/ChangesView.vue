<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { diff, loadChanges, rel, store } from './api'
import RefRow from './RefRow.vue'
import Section from './Section.vue'
import AgentCard from './AgentCard.vue'
import UnitCard from './UnitCard.vue'

// What changed since HEAD, in the terms the code is made of: each unit is
// declarations that change together, with where it is wired in, what it
// relies on, what it can break and which tests reach it. Marking a unit
// reviewed notes its code as it is now; a later change shows again.
onMounted(() => {
  if (!store.report) void loadChanges()
})

const r = computed(() => store.report)
</script>

<template>
  <div class="changes">
    <AgentCard />
    <div class="head">
      <span v-if="r">
        Since {{ r.since }}<template v-if="r.head.branch"> on {{ r.head.branch }}</template>:
        <span class="add">+{{ r.lines[0] }}</span> <span class="del">−{{ r.lines[1] }}</span>
      </span>
      <span v-else-if="store.reportBusy">Reading the changes…</span>
      <button title="Read the changes again" @click="loadChanges"><i class="codicon codicon-refresh" :class="{ 'codicon-modifier-spin': store.reportBusy }" /></button>
    </div>
    <div v-if="store.reportError" class="hint">{{ store.reportError }}</div>
    <div v-if="r?.warning" class="warn">{{ r.warning }}</div>
    <div v-if="r && !r.units.length && !r.other.length" class="hint">Nothing changed since {{ r.since }}.</div>

    <UnitCard v-for="u in r?.units" :key="u.id" :u="u" />

    <Section v-if="r" title="Other files" :count="r.other.length" :open="false">
      <div v-for="f in r.other" :key="f.path" class="row" @click="diff(f.path)">
        <i class="codicon codicon-file" /> {{ rel(f.path) }}
        <span class="dim"><span class="add">+{{ f.added }}</span> <span class="del">−{{ f.deleted }}</span></span>
      </div>
    </Section>
    <Section title="Changed since you reviewed" :count="store.stale.length">
      <RefRow v-for="x in store.stale" :key="x.key" :r="x" />
    </Section>
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
  margin-left: auto;
  font-size: 11px;
}
</style>
