<script setup lang="ts">
import { ref } from 'vue'
import { go, icons, name, openAt, plural, type Step } from './api'

// One function on a flow, and the calls it makes, in the order of its
// code. Calls through an interface open into the implementations that
// may run.
const props = defineProps<{ step: Step; depth: number }>()
const open = ref(props.depth < 2)

function where() {
  // The line of the call in the caller, when there is one.
  if (props.step.siteFile && props.step.siteLine) void openAt(props.step.siteFile, props.step.siteLine)
  else void openAt(props.step.file, props.step.line, props.step.col)
}
</script>

<template>
  <div class="step">
    <div class="line" :style="{ paddingLeft: 6 + depth * 12 + 'px' }" @click="where">
      <button class="fold" :class="{ none: !step.children.length }" @click.stop="open = !open">
        <i :class="['codicon', open ? 'codicon-chevron-down' : 'codicon-chevron-right']" />
      </button>
      <i :class="['codicon', 'codicon-' + (icons[step.kind] || 'symbol-misc')]" />
      <span class="name" :title="step.sig">{{ name(step) }}</span>
      <span v-if="step.certainty === 'dynamic'" class="tag" title="Through an interface: which implementation runs is decided at run time">
        {{ step.choices ? 'one of ' + step.choices : 'dynamic' }}
      </span>
      <span v-if="step.errReturns" class="err" :title="plural(step.errReturns, 'place') + ' it returns an error'">
        <i class="codicon codicon-error" />{{ step.errReturns }}
      </span>
      <span v-if="step.cycle" class="dim">calls back up</span>
      <span v-if="step.truncated" class="dim">…</span>
      <span class="dim">{{ step.pkgName }}</span>
      <button class="card" title="Card" @click.stop="go({ kind: 'card', key: step.key })"><i class="codicon codicon-info" /></button>
    </div>
    <template v-if="open">
      <FlowStep v-for="c in step.children" :key="c.key + c.siteLine" :step="c" :depth="depth + 1" />
    </template>
  </div>
</template>

<style scoped>
.line {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-top: 1px;
  padding-bottom: 1px;
  padding-right: 6px;
  cursor: pointer;
  white-space: nowrap;
}
.line:hover {
  background: var(--bg-hover);
}
.line .codicon {
  font-size: 14px;
  color: var(--text-dim);
}
.fold.none {
  visibility: hidden;
}
.name {
  overflow: hidden;
  text-overflow: ellipsis;
}
.tag {
  font-size: 10px;
  padding: 0 4px;
  border-radius: 3px;
  border: 1px dashed var(--border-strong);
  color: var(--text-dim);
}
.err {
  display: flex;
  align-items: center;
  gap: 1px;
  font-size: 11px;
  color: var(--warning);
}
.err .codicon {
  font-size: 12px;
  color: var(--warning);
}
.dim {
  color: var(--text-faint);
  font-size: 11px;
}
.card {
  margin-left: auto;
  visibility: hidden;
}
.line:hover .card {
  visibility: visible;
}
</style>
