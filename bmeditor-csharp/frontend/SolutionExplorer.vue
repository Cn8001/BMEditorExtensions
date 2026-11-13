<script setup lang="ts">
import { computed, ref } from 'vue'
import { api, store } from './api'
import Graph from './Graph.vue'
import Inspector from './Inspector.vue'
import ProjectProperties from './ProjectProperties.vue'
import SolutionPanel from './SolutionPanel.vue'

// The Explorer's Solution and Assemblies views of a C# project, as Rider
// offers them beside the files: the Solution Explorer or the assembly
// graph, and under it the properties of the open assembly definition or
// package (the Inspector), as Visual Studio has them under its Solution
// Explorer.
const props = defineProps<{ graph?: boolean }>()

const inspectable = computed(() => {
  const path = (api.ide.activeId ? api.session.tab(api.ide.activeId)?.path : '') ?? ''
  return /\.(asmdef|asmref)$/i.test(path) || /[\\/]package\.json$/i.test(path)
})
// The project selected in the Solution Explorer first, as Visual Studio's
// Properties window follows the selection; else the open file's.
const project = computed(() => (props.graph ? null : store.ui.selectedProject))
const folded = ref(false)
</script>

<template>
  <div class="solution-explorer">
    <Graph v-if="props.graph" narrow class="tree-part" />
    <SolutionPanel v-else side class="tree-part" />
    <section v-if="project || inspectable" class="properties" :class="{ folded }">
      <button class="properties-head" @click="folded = !folded">
        <i class="codicon" :class="folded ? 'codicon-chevron-right' : 'codicon-chevron-down'" />
        Properties
      </button>
      <template v-if="!folded">
        <ProjectProperties v-if="project" :key="project.path" :project="project" class="properties-body" />
        <Inspector v-else narrow class="properties-body" />
      </template>
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
