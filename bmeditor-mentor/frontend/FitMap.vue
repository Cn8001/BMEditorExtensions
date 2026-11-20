<script setup lang="ts">
import { name, openAt, rel, type FitBox } from './api'

// Where a step's new part sits in the project: what calls into it above,
// the part in the middle, what it relies on below, each box a place in the
// code with its layer. The aim is to place it in your map of the project,
// not to read every line of it.
defineProps<{ fit: { above: FitBox[]; parts: FitBox[]; below: FitBox[] } }>()
</script>

<template>
  <div class="fit">
    <div v-if="fit.above.length" class="row">
      <button v-for="b in fit.above" :key="b.key" class="box" :title="rel(b.file) + ':' + b.line" @click="openAt(b.file, b.line)">
        <span v-if="b.layer" class="layer">{{ b.layer }}</span>{{ name(b) }}
      </button>
    </div>
    <div v-if="fit.above.length" class="arrow">↓ calls</div>
    <div class="row">
      <button v-for="b in fit.parts" :key="b.key" class="box new" :class="b.change" :title="(b.change || '') + ' · ' + rel(b.file) + ':' + b.line" @click="openAt(b.file, b.line)">
        <span v-if="b.layer" class="layer">{{ b.layer }}</span>{{ name(b) }}
        <span class="mark">{{ b.change === 'added' ? 'new' : 'changed' }}</span>
      </button>
      <span v-if="!fit.parts.length" class="dim">No declaration changed.</span>
    </div>
    <div v-if="fit.below.length" class="arrow">↓ uses</div>
    <div v-if="fit.below.length" class="row">
      <button v-for="b in fit.below" :key="b.key" class="box" :title="rel(b.file) + ':' + b.line" @click="openAt(b.file, b.line)">
        <span v-if="b.layer" class="layer">{{ b.layer }}</span>{{ name(b) }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.fit {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 4px 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 5px;
}
.box {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 11px;
  cursor: pointer;
}
.box:hover {
  border-color: var(--accent);
}
.box.new {
  border-color: var(--accent);
  background: var(--accent-dim, rgba(239, 227, 203, 0.12));
}
.layer {
  font-family: var(--font-ui);
  font-size: 9px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-faint);
}
.mark {
  font-family: var(--font-ui);
  font-size: 9px;
  color: var(--accent);
}
.arrow {
  font-size: 10px;
  color: var(--text-faint);
}
.dim {
  color: var(--text-faint);
  font-size: 11px;
}
</style>
