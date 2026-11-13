<script setup lang="ts">
import { computed, ref } from 'vue'
import { api } from './api'
import Inspector from './Inspector.vue'
import SolutionPanel from './SolutionPanel.vue'

// The Explorer of a C# project, as Visual Studio lays it out: the Solution
// Explorer, and under it the properties of the open assembly definition or
// package (the Inspector), which the bottom panel no longer holds.

const inspectable = computed(() => {
  const path = (api.ide.activeId ? api.session.tab(api.ide.activeId)?.path : '') ?? ''
  return /\.(asmdef|asmref)$/i.test(path) || /[\\/]package\.json$/i.test(path)
})
const folded = ref(false)
</script>

<template>
  <div class="solution-explorer">
    <SolutionPanel side class="tree-part" />
    <section v-if="inspectable" class="properties" :class="{ folded }">
      <button class="properties-head" @click="folded = !folded">
        <i class="codicon" :class="folded ? 'codicon-chevron-right' : 'codicon-chevron-down'" />
        Properties
      </button>
      <Inspector v-if="!folded" narrow class="properties-body" />
    </section>
  </div>
</template>

<style scoped>
.solution-explorer {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.tree-part {
  flex: 1;
  min-height: 120px;
}
.properties {
  display: flex;
  flex-direction: column;
  max-height: 55%;
  min-height: 0;
  border-top: 1px solid var(--border);
}
.properties.folded {
  flex: none;
}
.properties-head {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: none;
  padding: 5px 8px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-dim);
  text-align: left;
}
.properties-head:hover {
  color: var(--text);
}
.properties-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
</style>
