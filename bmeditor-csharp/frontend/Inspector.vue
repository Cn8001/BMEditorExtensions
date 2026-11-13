<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, basename, loadAssemblies, openPath, store, type Assembly } from './api'

// The Inspector: the active tab's .asmdef, .asmref or UPM package.json as a
// form. Every change rewrites the tab's text as one edit, so undo, unsaved
// changes and saving work as they do for typing; the indentation, line
// endings and the order of the keys stay as they were.

/* eslint-disable @typescript-eslint/no-explicit-any */

// narrow is the sidebar's Properties: labels above their fields.
const props = defineProps<{ narrow?: boolean }>()

const tab = computed(() => (api.ide.activeId ? api.session.tab(api.ide.activeId) : undefined))
const text = computed(() => {
  const t = tab.value
  if (!t) return ''
  void t.version // read again on every edit
  return api.session.textOf(t.id)
})
const parsed = computed<{ data: any; error: string }>(() => {
  try {
    return { data: JSON.parse(text.value.replace(/^﻿/, '')), error: '' }
  } catch (e) {
    return { data: null, error: (e as Error).message }
  }
})
const upmName = /^[a-z0-9][a-z0-9._-]*\.[a-z0-9._-]+$/
const kind = computed<'asmdef' | 'asmref' | 'package' | ''>(() => {
  const p = tab.value?.path.toLowerCase() ?? ''
  if (p.endsWith('.asmdef')) return 'asmdef'
  if (p.endsWith('.asmref')) return 'asmref'
  if (/[\\/]package\.json$/.test(p) && typeof parsed.value.data?.name === 'string' && upmName.test(parsed.value.data.name)) return 'package'
  return ''
})
const d = computed(() => parsed.value.data ?? {})

watch(kind, (k) => (k === 'asmdef' || k === 'asmref') && !store.assemblies && loadAssemblies(), { immediate: true })

function indentOf(src: string): string | number {
  const m = /\n([ \t]+)"/.exec(src)
  if (!m) return kind.value === 'package' ? 2 : 4
  return m[1].startsWith('\t') ? '\t' : m[1].length
}

function write(mutate: (data: any) => void) {
  const t = tab.value
  const state = t && api.session.stateOf(t.id)
  if (!t || !state) return
  const src = state.doc.toString()
  let data: any
  try {
    data = JSON.parse(src.replace(/^﻿/, ''))
  } catch {
    return
  }
  mutate(data)
  const nl = src.includes('\r\n') ? '\r\n' : '\n'
  let out = JSON.stringify(data, null, indentOf(src)).replace(/\n/g, nl)
  if (/\n$/.test(src)) out += nl
  if (src.startsWith('﻿')) out = '﻿' + out
  if (out !== src) api.session.dispatchTo(t.id, { changes: { from: 0, to: state.doc.length, insert: out } })
}

// set writes a field; an empty optional one is left out.
function set(key: string, value: unknown, keepEmpty = false) {
  write((data) => {
    if (!keepEmpty && (value === '' || (Array.isArray(value) && !value.length))) delete data[key]
    else data[key] = value
  })
}

const val = (e: Event) => (e.target as HTMLInputElement).value
const checked = (e: Event) => (e.target as HTMLInputElement).checked
const lines = (e: Event) =>
  val(e)
    .split(/\r?\n|,/)
    .map((s) => s.trim())
    .filter(Boolean)

// Assembly definitions

const others = computed(() => store.assemblies?.assemblies.filter((a) => a.path !== tab.value?.path) ?? [])
const byName = computed(() => new Map(others.value.map((a) => [a.name, a])))
const byGUID = computed(() => new Map(others.value.filter((a) => a.guid).map((a) => [a.guid!.toLowerCase(), a])))

function resolve(raw: string): Assembly | undefined {
  return raw.startsWith('GUID:') ? byGUID.value.get(raw.slice(5).trim().toLowerCase()) : byName.value.get(raw)
}

const refs = computed<string[]>(() => (Array.isArray(d.value.references) ? d.value.references : []))
const useGUIDs = computed(() => (refs.value.length ? refs.value.every((r) => r.startsWith('GUID:')) : true))

function refFor(a: Assembly, guids = useGUIDs.value) {
  return guids && a.guid ? 'GUID:' + a.guid : a.name
}

function setGUIDs(on: boolean) {
  write((data) => {
    data.references = (data.references ?? []).map((r: string) => {
      const a = resolve(r)
      return a ? refFor(a, on) : r
    })
  })
}

const adding = ref('')
const addable = computed(() => {
  const have = new Set(refs.value.map((r) => resolve(r)?.name ?? r))
  return others.value.filter((a) => !have.has(a.name))
})

function addRef() {
  const a = byName.value.get(adding.value.trim())
  if (!a) return
  write((data) => (data.references = [...(data.references ?? []), refFor(a)]))
  adding.value = ''
}

function removeRef(i: number) {
  write((data) => data.references.splice(i, 1))
}

const platforms = [
  'Android',
  'Editor',
  'EmbeddedLinux',
  'GameCoreScarlett',
  'GameCoreXboxOne',
  'iOS',
  'LinuxStandalone64',
  'macOSStandalone',
  'PS4',
  'PS5',
  'QNX',
  'Switch',
  'tvOS',
  'VisionOS',
  'WSA',
  'WebGL',
  'WindowsStandalone32',
  'WindowsStandalone64',
  'XboxOne',
]
const anyPlatform = computed(() => !(d.value.includePlatforms?.length > 0))
const platformList = computed(() => [...new Set([...platforms, ...(d.value.includePlatforms ?? []), ...(d.value.excludePlatforms ?? [])])])
const listed = computed<string[]>(() => (anyPlatform.value ? (d.value.excludePlatforms ?? []) : (d.value.includePlatforms ?? [])))

function setAny(on: boolean) {
  write((data) => {
    data.includePlatforms = on ? [] : ['Editor']
    data.excludePlatforms = []
  })
}

function togglePlatform(p: string, on: boolean) {
  const key = anyPlatform.value ? 'excludePlatforms' : 'includePlatforms'
  write((data) => {
    const now: string[] = data[key] ?? []
    data[key] = on ? [...now, p].sort() : now.filter((x) => x !== p)
  })
}

function setVersionDefine(i: number | string, key: string, value: string) {
  write((data) => (data.versionDefines[i][key] = value))
}

// Packages

const semver = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/
const deps = computed<[string, string][]>(() => Object.entries(d.value.dependencies ?? {}) as [string, string][])
const author = computed(() => (typeof d.value.author === 'string' ? { name: d.value.author } : (d.value.author ?? {})))

function setAuthor(key: 'name' | 'email' | 'url', value: string) {
  write((data) => {
    const a = typeof data.author === 'string' ? { name: data.author } : { ...(data.author ?? {}) }
    if (value) a[key] = value
    else delete a[key]
    if (!Object.keys(a).length) delete data.author
    else data.author = Object.keys(a).length === 1 && a.name && typeof data.author === 'string' ? a.name : a
  })
}

function setDep(i: number, name: string, version: string) {
  write((data) => {
    const entries = Object.entries(data.dependencies ?? {})
    entries[i] = [name, version]
    data.dependencies = Object.fromEntries(entries)
  })
}

function removeDep(i: number) {
  write((data) => {
    const entries = Object.entries(data.dependencies ?? {})
    entries.splice(i, 1)
    data.dependencies = Object.fromEntries(entries)
  })
}

const newDep = ref({ name: '', version: '' })
function addDep() {
  const { name, version } = newDep.value
  if (!name.trim() || !version.trim()) return
  write((data) => (data.dependencies = { ...(data.dependencies ?? {}), [name.trim()]: version.trim() }))
  newDep.value = { name: '', version: '' }
}

function depVersionOK(v: string) {
  return semver.test(v) || /^(file:|git\+|https?:|ssh:|git@)/.test(v)
}

function setSample(i: number | string, key: string, value: string) {
  write((data) => (data.samples[i][key] = value))
}
</script>

<template>
  <div class="inspector" :class="{ narrow: props.narrow }">
    <p v-if="!kind" class="empty">Open an assembly definition (.asmdef, .asmref) or a UPM package's package.json to edit it here.</p>
    <p v-else-if="parsed.error" class="empty bad">{{ basename(tab!.path) }} is not valid JSON ({{ parsed.error }}). Fix it in the text to edit it here.</p>

    <form v-else-if="kind === 'asmdef'" class="form" @submit.prevent>
      <h3><i class="codicon codicon-json" /> Assembly Definition <span class="file" @click="openPath(tab!.path)">{{ basename(tab!.path) }}</span></h3>
      <div class="grid">
        <label>Name</label>
        <input :value="d.name" spellcheck="false" @change="set('name', val($event), true)" />
        <label>Root namespace</label>
        <input :value="d.rootNamespace ?? ''" spellcheck="false" placeholder="New scripts use it" @change="set('rootNamespace', val($event), true)" />
        <label>Options</label>
        <div class="checks">
          <label><input type="checkbox" :checked="d.autoReferenced !== false" @change="set('autoReferenced', checked($event), true)" /> Auto referenced</label>
          <label><input type="checkbox" :checked="!!d.allowUnsafeCode" @change="set('allowUnsafeCode', checked($event), true)" /> Allow unsafe code</label>
          <label><input type="checkbox" :checked="!!d.noEngineReferences" @change="set('noEngineReferences', checked($event), true)" /> No engine references</label>
          <label><input type="checkbox" :checked="!!d.overrideReferences" @change="set('overrideReferences', checked($event), true)" /> Override references</label>
        </div>
        <template v-if="d.overrideReferences">
          <label>Precompiled references</label>
          <textarea :value="(d.precompiledReferences ?? []).join('\n')" rows="3" spellcheck="false" placeholder="One .dll per line" @change="set('precompiledReferences', lines($event), true)" />
        </template>
      </div>

      <h4>
        References
        <label class="right"><input type="checkbox" :checked="useGUIDs" @change="setGUIDs(checked($event))" /> Use GUIDs</label>
      </h4>
      <ul class="list">
        <li v-for="(r, i) in refs" :key="r + i">
          <i class="codicon" :class="resolve(r) ? 'codicon-symbol-namespace' : 'codicon-warning'" />
          <span :class="{ warn: !resolve(r) }">{{ resolve(r)?.name ?? r }}</span>
          <span v-if="resolve(r)?.external" class="faint">package</span>
          <span v-else-if="!resolve(r)" class="faint">{{ store.assemblies ? 'not found here' : '' }}</span>
          <button title="Remove" @click="removeRef(i)"><i class="codicon codicon-close" /></button>
        </li>
        <li v-if="!refs.length" class="faint">None</li>
      </ul>
      <div class="add">
        <input v-model="adding" list="cs-asm-names" placeholder="Add a reference…" spellcheck="false" @change="addRef" @keydown.enter.prevent="addRef" />
        <datalist id="cs-asm-names">
          <option v-for="a in addable" :key="a.path" :value="a.name">{{ a.external ? 'package' : '' }}</option>
        </datalist>
      </div>

      <h4>Platforms</h4>
      <label class="check"><input type="checkbox" :checked="anyPlatform" @change="setAny(checked($event))" /> Any platform</label>
      <p class="faint">{{ anyPlatform ? 'Exclude:' : 'Include only:' }}</p>
      <div class="platforms">
        <label v-for="p in platformList" :key="p"><input type="checkbox" :checked="listed.includes(p)" @change="togglePlatform(p, checked($event))" /> {{ p }}</label>
      </div>

      <h4>Define constraints</h4>
      <textarea
        :value="(d.defineConstraints ?? []).join('\n')"
        rows="2"
        spellcheck="false"
        placeholder="One per line: UNITY_INCLUDE_TESTS, !UNITY_WEBGL…"
        @change="set('defineConstraints', lines($event), true)"
      />

      <h4>Version defines</h4>
      <table class="table">
        <tr v-if="d.versionDefines?.length">
          <th>Resource</th>
          <th>Expression</th>
          <th>Define</th>
          <th />
        </tr>
        <tr v-for="(v, i) in d.versionDefines ?? []" :key="i">
          <td><input :value="v.name" spellcheck="false" placeholder="com.unity.inputsystem" @change="setVersionDefine(i, 'name', val($event))" /></td>
          <td><input :value="v.expression" spellcheck="false" placeholder="1.0.0" @change="setVersionDefine(i, 'expression', val($event))" /></td>
          <td><input :value="v.define" spellcheck="false" placeholder="HAS_INPUT_SYSTEM" @change="setVersionDefine(i, 'define', val($event))" /></td>
          <td>
            <button title="Remove" @click="write((data) => data.versionDefines.splice(i, 1))"><i class="codicon codicon-close" /></button>
          </td>
        </tr>
      </table>
      <button @click="write((data) => (data.versionDefines = [...(data.versionDefines ?? []), { name: '', expression: '', define: '' }]))"><i class="codicon codicon-add" /> Add version define</button>
    </form>

    <form v-else-if="kind === 'asmref'" class="form" @submit.prevent>
      <h3><i class="codicon codicon-json" /> Assembly Definition Reference <span class="file" @click="openPath(tab!.path)">{{ basename(tab!.path) }}</span></h3>
      <p class="faint">The scripts of this folder join the assembly it names.</p>
      <div class="grid">
        <label>Assembly</label>
        <div class="add">
          <input
            :value="resolve(d.reference ?? '')?.name ?? d.reference"
            list="cs-asm-all"
            spellcheck="false"
            @change="(e) => { const a = byName.get(val(e)); set('reference', a ? refFor(a, String(d.reference ?? '').startsWith('GUID:') || !d.reference) : val(e), true) }"
          />
          <datalist id="cs-asm-all">
            <option v-for="a in others" :key="a.path" :value="a.name" />
          </datalist>
        </div>
        <label />
        <p v-if="d.reference && !resolve(d.reference)" class="warn">{{ d.reference }} is not an assembly here.</p>
      </div>
    </form>

    <form v-else class="form" @submit.prevent>
      <h3><i class="codicon codicon-package" /> Package <span class="file" @click="openPath(tab!.path)">{{ basename(tab!.path) }}</span></h3>
      <div class="grid">
        <label>Name</label>
        <div>
          <input :value="d.name" spellcheck="false" @change="set('name', val($event), true)" />
          <p v-if="!upmName.test(d.name)" class="warn">Lower case, in reverse-domain form: com.company.package</p>
        </div>
        <label>Version</label>
        <div>
          <input :value="d.version" spellcheck="false" @change="set('version', val($event), true)" />
          <p v-if="!semver.test(d.version ?? '')" class="warn">A semantic version: 1.2.3 or 1.2.3-preview.1</p>
        </div>
        <label>Display name</label>
        <input :value="d.displayName ?? ''" @change="set('displayName', val($event))" />
        <label>Description</label>
        <textarea :value="d.description ?? ''" rows="2" @change="set('description', val($event))" />
        <label>Unity</label>
        <div class="pair">
          <input :value="d.unity ?? ''" placeholder="2022.3" spellcheck="false" @change="set('unity', val($event))" />
          <input :value="d.unityRelease ?? ''" placeholder="release, e.g. 0f1" spellcheck="false" @change="set('unityRelease', val($event))" />
        </div>
        <label>Author</label>
        <div class="pair">
          <input :value="author.name ?? ''" placeholder="Name" @change="setAuthor('name', val($event))" />
          <input :value="author.email ?? ''" placeholder="Email" @change="setAuthor('email', val($event))" />
          <input :value="author.url ?? ''" placeholder="URL" @change="setAuthor('url', val($event))" />
        </div>
        <label>License</label>
        <input :value="d.license ?? ''" placeholder="MIT" @change="set('license', val($event))" />
        <label>Keywords</label>
        <input :value="(d.keywords ?? []).join(', ')" placeholder="comma separated" @change="set('keywords', lines($event))" />
        <label>Links</label>
        <div class="pair">
          <input :value="d.documentationUrl ?? ''" placeholder="Documentation URL" @change="set('documentationUrl', val($event))" />
          <input :value="d.changelogUrl ?? ''" placeholder="Changelog URL" @change="set('changelogUrl', val($event))" />
        </div>
      </div>

      <h4>Dependencies</h4>
      <table class="table">
        <tr v-for="([name, version], i) in deps" :key="name">
          <td><input :value="name" spellcheck="false" :class="{ invalid: !upmName.test(name) }" @change="setDep(i, val($event), version)" /></td>
          <td><input :value="version" spellcheck="false" :class="{ invalid: !depVersionOK(version) }" @change="setDep(i, name, val($event))" /></td>
          <td>
            <button title="Remove" @click="removeDep(i)"><i class="codicon codicon-close" /></button>
          </td>
        </tr>
        <tr>
          <td><input v-model="newDep.name" placeholder="com.unity.inputsystem" spellcheck="false" @keydown.enter.prevent="addDep" /></td>
          <td><input v-model="newDep.version" placeholder="1.7.0" spellcheck="false" @keydown.enter.prevent="addDep" /></td>
          <td>
            <button title="Add" @click="addDep"><i class="codicon codicon-add" /></button>
          </td>
        </tr>
      </table>

      <h4>Samples</h4>
      <table class="table">
        <tr v-if="d.samples?.length">
          <th>Name</th>
          <th>Description</th>
          <th>Path</th>
          <th />
        </tr>
        <tr v-for="(s, i) in d.samples ?? []" :key="i">
          <td><input :value="s.displayName" @change="setSample(i, 'displayName', val($event))" /></td>
          <td><input :value="s.description" @change="setSample(i, 'description', val($event))" /></td>
          <td><input :value="s.path" spellcheck="false" placeholder="Samples~/Basic" @change="setSample(i, 'path', val($event))" /></td>
          <td>
            <button title="Remove" @click="write((data) => data.samples.splice(i, 1))"><i class="codicon codicon-close" /></button>
          </td>
        </tr>
      </table>
      <button @click="write((data) => (data.samples = [...(data.samples ?? []), { displayName: 'Sample', description: '', path: 'Samples~/Sample' }]))">
        <i class="codicon codicon-add" /> Add sample
      </button>
    </form>
  </div>
</template>

<style scoped>
.inspector {
  height: 100%;
  min-height: 0;
  overflow: auto;
  font-size: 12px;
}
.form {
  padding: 8px 14px 16px;
  max-width: 860px;
}
h3 {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--text);
}
h4 {
  display: flex;
  align-items: center;
  margin: 14px 0 6px;
  font-size: 12px;
  color: var(--text-dim);
}
.right {
  margin-left: auto;
  font-weight: normal;
}
.file {
  color: var(--accent);
  font-weight: normal;
  cursor: pointer;
}
.grid {
  display: grid;
  grid-template-columns: 150px 1fr;
  gap: 6px 10px;
  align-items: center;
}
.grid > label {
  color: var(--text-dim);
}
input:not([type='checkbox']),
textarea,
select {
  width: 100%;
  box-sizing: border-box;
  background: var(--bg);
  color: var(--text);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  padding: 3px 6px;
  font: inherit;
}
textarea {
  font-family: var(--font-mono);
  resize: vertical;
}
input.invalid {
  border-color: var(--warning, #d7a14a);
}
.checks,
.platforms {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
}
.platforms label {
  width: 170px;
}
label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--text);
}
.check {
  margin-bottom: 4px;
}
.pair {
  display: flex;
  gap: 6px;
}
.list {
  list-style: none;
  margin: 0 0 6px;
  padding: 0;
}
.list li {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0;
}
.list li button {
  margin-left: auto;
}
.add {
  max-width: 360px;
}
.table {
  border-collapse: collapse;
  width: 100%;
  margin-bottom: 6px;
}
.table th {
  text-align: left;
  font-weight: normal;
  color: var(--text-faint);
  padding: 0 4px 2px;
}
.table td {
  padding: 2px 4px 2px 0;
}
.table td:last-child {
  width: 28px;
}
button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 7px;
  border-radius: 4px;
  color: var(--text-dim);
}
button:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.faint {
  color: var(--text-faint);
  margin: 2px 0;
}
.warn {
  color: var(--warning, #d7a14a);
  margin: 2px 0 0;
}
.bad {
  color: var(--error);
}
.empty {
  color: var(--text-dim);
  padding: 12px;
}
.narrow .grid {
  grid-template-columns: 1fr;
}
.narrow .form {
  padding: 4px 10px 12px;
}
</style>
