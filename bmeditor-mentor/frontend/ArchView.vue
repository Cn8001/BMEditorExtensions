<script setup lang="ts">
import { computed } from 'vue'
import { name, openAt, store, type PkgChange } from './api'

// The project's shape with the agent's work on it: every package of the
// project by layer, the ones it touched lit, what it added in them, and the
// dependencies between packages that came or went, the ones that cut across
// the layers said so. The whole at a glance, rather than line by line.
const props = defineProps<{ compact?: boolean }>()

const order = ['Entry', 'API', 'Service', 'Providers', 'Repository', 'Model', 'Config', 'Shared', '']
const a = computed(() => store.arch)

const rows = computed(() => {
  const touched = new Map<string, PkgChange>((a.value?.packages ?? []).map((p) => [p.path, p]))
  const byLayer = new Map<string, { path: string; label: string; change?: PkgChange }[]>()
  const put = (layer: string, path: string, label: string) => {
    if (!byLayer.has(layer)) byLayer.set(layer, [])
    byLayer.get(layer)!.push({ path, label, change: touched.get(path) })
  }
  for (const n of store.shape?.nodes ?? []) {
    if (n.kind !== 'package' || n.external) continue
    put(n.layer || '', n.id, n.label)
  }
  // A package the agent made that the map does not have yet.
  for (const p of touched.values()) {
    if (![...byLayer.values()].some((l) => l.some((x) => x.path === p.path))) put(p.layer || '', p.path, p.name)
  }
  return order
    .filter((l) => byLayer.has(l))
    .map((l) => ({ layer: l || 'Other', pkgs: byLayer.get(l)!.sort((x, y) => Number(!!y.change) - Number(!!x.change) || x.label.localeCompare(y.label)) }))
    .filter((r) => !props.compact || r.pkgs.some((p) => p.change))
})

const parts = computed(() => (a.value?.packages ?? []).flatMap((p) => p.parts.map((r) => ({ pkg: p.name, r }))))
const warned = computed(() => (a.value?.edges ?? []).filter((e) => e.warn))
</script>

<template>
  <div v-if="a && a.packages.length" class="arch">
    <div v-for="row in rows" :key="row.layer" class="row">
      <span class="layer">{{ row.layer }}</span>
      <span class="pkgs">
        <span v-for="p in row.pkgs" :key="p.path" class="pkg" :class="{ on: !!p.change, new: p.change?.new }" :title="p.path">
          {{ p.label }}
          <span v-if="p.change?.new" class="badge">new</span>
          <span v-else-if="p.change" class="n">+{{ p.change.added }}<template v-if="p.change.modified"> ~{{ p.change.modified }}</template><template v-if="p.change.removed"> −{{ p.change.removed }}</template></span>
        </span>
      </span>
    </div>
    <div v-if="parts.length" class="line">
      <span class="what">New</span>
      <button v-for="x in parts" :key="x.r.key" class="part" :title="x.pkg" @click="openAt(x.r.file, x.r.line)">{{ x.pkg }}.{{ name(x.r) }}</button>
    </div>
    <div v-for="e in a.edges" :key="e.from + e.to" class="line edge" :class="{ gone: !e.added, warn: !!e.warn }" @click="e.file && openAt(e.file, e.line || 1)">
      <span class="what">{{ e.added ? 'Now uses' : 'No longer uses' }}</span>
      <span>{{ e.fromName }} → {{ e.toName }}</span>
      <span v-if="e.warn" class="w"><i class="codicon codicon-warning" /> {{ e.warn }}</span>
    </div>
    <div v-if="!a.edges.length && !compact" class="line dim">No dependency between packages came or went.</div>
    <div v-if="warned.length === 0 && a.edges.length && !compact" class="line dim">The layers hold.</div>
  </div>
</template>

<style scoped>
.arch {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 4px 0;
  font-size: 11px;
}
.row {
  display: flex;
  align-items: baseline;
  gap: 6px;
}
.layer {
  flex: none;
  width: 68px;
  text-align: right;
  font-size: 9px;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-faint);
}
.pkgs {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
}
.pkg {
  padding: 0 6px;
  border: 1px solid var(--border);
  border-radius: 3px;
  font-family: var(--font-mono);
  color: var(--text-faint);
}
.pkg.on {
  border-color: var(--warning);
  background: rgba(229, 192, 123, 0.1);
  color: var(--text);
}
.pkg.new {
  border-color: var(--success, #b8d27a);
  background: rgba(184, 210, 122, 0.1);
}
.n {
  font-size: 10px;
  color: var(--text-dim);
}
.badge {
  font-family: var(--font-ui);
  font-size: 9px;
  color: var(--success, #b8d27a);
}
.line {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 6px;
  padding-left: 74px;
}
.what {
  color: var(--text-faint);
}
.part {
  padding: 0 5px;
  border-radius: 3px;
  background: rgba(184, 210, 122, 0.1);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 11px;
  cursor: pointer;
}
.edge {
  font-family: var(--font-mono);
  color: var(--text-dim);
  cursor: pointer;
}
.edge .what {
  font-family: var(--font-ui);
}
.edge.gone span:nth-child(2) {
  text-decoration: line-through;
}
.w {
  font-family: var(--font-ui);
  color: var(--warning);
}
.dim {
  color: var(--text-faint);
}
</style>
