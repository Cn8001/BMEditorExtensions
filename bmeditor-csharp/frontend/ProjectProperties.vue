<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { basename, loadAssemblies, openPath, rel, store, type Project } from './api'

// A project's properties, as Visual Studio's Properties window shows the
// project selected in the Solution Explorer, and in a Unity project those
// of its assembly definition, which Edit opens in the Inspector.

const props = defineProps<{ project: Project }>()

const p = computed(() => props.project)
const asmdef = computed(() => store.assemblies?.assemblies.find((a) => a.name === (p.value.assemblyName || p.value.name) && !a.external))
const kind = computed(() => (asmdef.value?.test ? 'Tests' : asmdef.value?.editor ? 'Editor only' : 'Runtime'))
const platforms = computed(() => {
  const a = asmdef.value
  if (a?.includePlatforms?.length) return 'Only ' + a.includePlatforms.join(', ')
  if (a?.excludePlatforms?.length) return 'All but ' + a.excludePlatforms.join(', ')
  return 'Any'
})
const references = computed(() => {
  const parts = [
    [p.value.projectReferences?.length ?? 0, 'project', 'projects'],
    [p.value.packageReferences?.length ?? 0, 'package', 'packages'],
    [p.value.references?.length ?? 0, 'assembly', 'assemblies'],
  ] as const
  return parts.filter(([n]) => n).map(([n, one, many]) => `${n} ${n === 1 ? one : many}`).join(' · ') || 'None'
})

function edit() {
  const a = asmdef.value
  if (!a) return
  store.ui.selectedProject = null
  void openPath(a.path)
}

onMounted(() => {
  const k = store.workspace?.kind
  if ((k === 'unity' || k === 'upm') && !store.assemblies && !store.busy.assemblies) void loadAssemblies()
})
</script>

<template>
  <div class="project-props">
    <h3><i class="codicon codicon-project" /> {{ p.name }}</h3>
    <dl>
      <dt>Assembly</dt>
      <dd>{{ p.assemblyName || p.name }}</dd>
      <dt v-if="p.targetFramework">Framework</dt>
      <dd v-if="p.targetFramework">{{ p.targetFramework }}</dd>
      <dt>Output</dt>
      <dd>{{ p.outputType || 'Library' }}{{ p.sdk ? ' · SDK-style' : '' }}</dd>
      <dt>Project file</dt>
      <dd><button class="link" :title="p.path" @click="openPath(p.path)">{{ basename(p.path) }}</button></dd>
      <dt>Files</dt>
      <dd>{{ p.files.length }}</dd>
      <dt>References</dt>
      <dd>{{ references }}</dd>
    </dl>

    <template v-if="asmdef">
      <h4>Assembly definition</h4>
      <dl>
        <dt>File</dt>
        <dd><button class="link" :title="asmdef.path" @click="openPath(asmdef.path)">{{ rel(asmdef.path) }}</button></dd>
        <dt v-if="asmdef.rootNamespace">Namespace</dt>
        <dd v-if="asmdef.rootNamespace">{{ asmdef.rootNamespace }}</dd>
        <dt>Kind</dt>
        <dd>{{ kind }}</dd>
        <dt>Platforms</dt>
        <dd>{{ platforms }}</dd>
        <dt>Auto-referenced</dt>
        <dd>{{ asmdef.autoReferenced ? 'Yes' : 'No' }}</dd>
        <dt v-if="asmdef.defineConstraints?.length">Defines</dt>
        <dd v-if="asmdef.defineConstraints?.length">{{ asmdef.defineConstraints.join(' && ') }}</dd>
        <dt>References</dt>
        <dd>
          <span v-if="!asmdef.references.length" class="faint">None</span>
          <span v-for="r in asmdef.references" :key="r.raw" class="ref" :class="r.status">{{ r.name ?? r.raw }}</span>
        </dd>
      </dl>
      <button class="edit" @click="edit"><i class="codicon codicon-edit" /> Edit assembly definition</button>
    </template>
    <p v-else-if="store.busy.assemblies" class="faint">Reading assembly definitions…</p>
  </div>
</template>

<style scoped>
.project-props {
  padding: 4px 10px 12px;
  font-size: 12px;
}
h3 {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 8px;
  font-size: 13px;
}
h3 .codicon {
  color: var(--accent);
}
h4 {
  margin: 12px 0 6px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-faint);
}
dl {
  display: grid;
  grid-template-columns: 96px 1fr;
  gap: 4px 8px;
  margin: 0;
}
dt {
  color: var(--text-faint);
}
dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--text);
}
.link {
  color: var(--accent);
  text-align: left;
}
.link:hover {
  text-decoration: underline;
}
.ref {
  display: inline-block;
  margin: 0 4px 3px 0;
  padding: 0 6px;
  border-radius: 4px;
  background: var(--bg-hover);
}
.ref.external {
  color: var(--text-dim);
}
.ref.unresolved {
  color: var(--warning, #e5c07b);
}
.faint {
  color: var(--text-faint);
}
.edit {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  padding: 4px 10px;
  border: 1px solid var(--border-strong);
  border-radius: 5px;
  color: var(--text-dim);
}
.edit:hover {
  color: var(--text);
  background: var(--bg-hover);
}
</style>
