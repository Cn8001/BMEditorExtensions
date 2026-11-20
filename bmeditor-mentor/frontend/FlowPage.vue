<script setup lang="ts">
import { type Flow } from './api'
import FlowStep from './FlowStep.vue'

// The path a call takes: what runs, in what order, where it can fail.
defineProps<{ flow: Flow }>()
</script>

<template>
  <div class="flow">
    <p class="note">{{ flow.note }}</p>
    <FlowStep :step="flow.root" :depth="0" />
  </div>
</template>

<style scoped>
.flow {
  padding-bottom: 12px;
}
.note {
  margin: 6px 8px;
  color: var(--text-faint);
  font-size: 11px;
  line-height: 1.4;
}
</style>
