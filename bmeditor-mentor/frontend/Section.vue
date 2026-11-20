<script setup lang="ts">
import { ref } from 'vue'

// A part of a page that folds: its title says how many it holds.
const props = defineProps<{ title: string; count?: number; open?: boolean }>()
const shown = ref(props.open ?? true)
</script>

<template>
  <section v-if="count === undefined || count > 0">
    <button class="head" @click="shown = !shown">
      <i :class="['codicon', shown ? 'codicon-chevron-down' : 'codicon-chevron-right']" />
      <span>{{ title }}</span>
      <span v-if="count !== undefined" class="count">{{ count }}</span>
    </button>
    <div v-if="shown"><slot /></div>
  </section>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  padding: 6px 8px 3px;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-dim);
  text-align: left;
}
.head:hover {
  color: var(--text);
}
.count {
  font-weight: 400;
  color: var(--text-faint);
}
</style>
