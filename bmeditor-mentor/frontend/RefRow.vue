<script setup lang="ts">
import { go, icons, name, openAt, rel, type Ref } from './api'

// One symbol in a list: a click shows its card, the arrow its code. A call
// through an interface is marked: which implementation runs is decided at
// run time.
const props = defineProps<{ r: Ref; detail?: string; site?: boolean }>()

function code() {
  if (props.site && props.r.siteFile && props.r.siteLine) void openAt(props.r.siteFile, props.r.siteLine)
  else void openAt(props.r.file, props.r.line, props.r.col)
}
</script>

<template>
  <div class="ref" :title="r.sig || r.key" @click="go({ kind: 'card', key: r.key })">
    <i :class="['codicon', 'codicon-' + (icons[r.kind] || 'symbol-misc')]" />
    <span class="name">{{ name(r) }}</span>
    <span v-if="r.certainty === 'dynamic'" class="tag" title="Through an interface: which implementation runs is decided at run time">dynamic</span>
    <span v-if="r.test" class="tag">test</span>
    <span class="dim">{{ detail ?? r.pkgName }}</span>
    <button class="open" :title="site ? 'Go to the line where it happens' : 'Go to ' + rel(r.file) + ':' + r.line" @click.stop="code">
      <i class="codicon codicon-go-to-file" />
    </button>
  </div>
</template>

<style scoped>
.ref {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px 2px 14px;
  cursor: pointer;
  white-space: nowrap;
  min-width: 0;
}
.ref:hover {
  background: var(--bg-hover);
}
.ref .codicon {
  flex: none;
  font-size: 14px;
  color: var(--text-dim);
}
.name {
  overflow: hidden;
  text-overflow: ellipsis;
}
.dim {
  color: var(--text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 11px;
}
.tag {
  flex: none;
  font-size: 10px;
  padding: 0 4px;
  border-radius: 3px;
  border: 1px solid var(--border-strong);
  color: var(--text-dim);
}
.open {
  margin-left: auto;
  flex: none;
  visibility: hidden;
  color: var(--text-dim);
}
.ref:hover .open {
  visibility: visible;
}
.open:hover {
  color: var(--accent);
}
</style>
