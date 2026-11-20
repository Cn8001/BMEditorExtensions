<script setup lang="ts">
import { computed } from 'vue'
import { go, icons, openAt, type GNode, type Graph } from './api'

// The architecture as lists, which a sidebar reads better than a drawing:
// the packages by the part of the service they seem to be, each with what
// it imports; or one package's types and functions, with what they use
// outside it and what uses them.
const props = defineProps<{ graph: Graph }>()

const order = ['Entry', 'API', 'Service', 'Providers', 'Repository', 'Model', 'Config', 'Shared', '']

const byId = computed(() => new Map(props.graph.nodes.map((n) => [n.id, n])))
const label = (id: string) => byId.value.get(id)?.label ?? id

const layers = computed(() => {
  const groups = new Map<string, GNode[]>()
  for (const n of props.graph.nodes) {
    if (n.external) continue
    const l = n.layer ?? ''
    groups.set(l, [...(groups.get(l) ?? []), n])
  }
  return order.filter((l) => groups.has(l)).map((l) => ({ layer: l || 'Other', nodes: groups.get(l)! }))
})

const out = (id: string) => props.graph.edges.filter((e) => e.from === id)
const into = (id: string) => props.graph.edges.filter((e) => e.to === id)

const users = computed(() =>
  props.graph.nodes
    .filter((n) => n.external && out(n.id).length)
    .map((n) => ({ n, uses: out(n.id).map((e) => byId.value.get(e.to)?.label ?? e.to) })),
)

function open(n: GNode) {
  if (n.kind === 'package') void go({ kind: 'map', focus: n.id })
  else void go({ kind: 'card', key: n.id })
}
</script>

<template>
  <div class="map">
    <p class="note">{{ graph.note }}</p>

    <template v-if="graph.level === 'packages'">
      <div v-for="g in layers" :key="g.layer" class="layer">
        <div class="layer-name" :title="g.layer === 'Other' ? '' : 'Guessed from the package names'">{{ g.layer }}</div>
        <div v-for="n in g.nodes" :key="n.id" class="node" :title="n.doc" @click="open(n)">
          <div class="head">
            <i class="codicon codicon-package" />
            <span class="name">{{ n.label }}</span>
            <span v-for="e in n.entry || []" :key="e" class="tag">{{ e }}</span>
            <span v-if="n.errors" class="err" title="The compiler reported errors here"><i class="codicon codicon-error" /></span>
            <span class="dim">{{ n.symbols }}</span>
          </div>
          <div v-if="out(n.id).length" class="deps">→ {{ out(n.id).map((e) => label(e.to)).join(', ') }}</div>
        </div>
      </div>
    </template>

    <template v-else>
      <div v-if="graph.routes.length" class="layer">
        <div class="layer-name">Routes</div>
        <div v-for="r in graph.routes" :key="r.file + r.line" class="node" @click="r.handler ? go({ kind: 'card', key: r.handler }) : openAt(r.file, r.line)">
          <div class="head"><i class="codicon codicon-globe" /><span class="name">{{ r.method ? r.method + ' ' : '' }}{{ r.path }}</span></div>
        </div>
      </div>
      <div v-if="users.length" class="layer">
        <div class="layer-name">Used from</div>
        <div v-for="u in users" :key="u.n.id" class="node" @click="open(u.n)">
          <div class="head"><i class="codicon codicon-package" /><span class="name">{{ u.n.label }}</span></div>
          <div class="deps">→ {{ u.uses.join(', ') }}</div>
        </div>
      </div>
      <div class="layer">
        <div class="layer-name">Inside</div>
        <div v-for="n in graph.nodes.filter((x) => !x.external)" :key="n.id" class="node" :title="n.doc" @click="open(n)">
          <div class="head">
            <i :class="['codicon', 'codicon-' + (icons[n.kind] || 'symbol-misc')]" />
            <span class="name">{{ n.label }}</span>
            <span v-if="into(n.id).length" class="dim">{{ into(n.id).length }} in</span>
          </div>
          <div v-if="out(n.id).length" class="deps">
            →
            <span v-for="(e, i) in out(n.id)" :key="e.to" :class="{ dyn: e.certainty === 'dynamic' }">{{ i ? ', ' : '' }}{{ label(e.to) }}</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.map {
  padding-bottom: 12px;
}
.note {
  margin: 6px 8px;
  color: var(--text-faint);
  font-size: 11px;
  line-height: 1.4;
}
.layer-name {
  padding: 6px 8px 2px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-dim);
}
.node {
  padding: 3px 8px 3px 12px;
  cursor: pointer;
}
.node:hover {
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
  color: var(--text-dim);
}
.name {
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
.err .codicon {
  color: var(--error);
}
.dim {
  margin-left: auto;
  color: var(--text-faint);
  font-size: 11px;
}
.deps {
  padding-left: 19px;
  color: var(--text-faint);
  font-size: 11px;
  white-space: normal;
  line-height: 1.35;
}
.dyn {
  font-style: italic;
}
</style>
