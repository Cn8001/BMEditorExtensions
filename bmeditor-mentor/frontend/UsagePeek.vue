<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { call, message, openAt, rel, reveal } from './api'

// How a symbol is used, under its lens, in the code: the places in the
// project that show it best, as they are written there. A click goes there.
const props = defineProps<{ symbol: string; close: () => void }>()

interface Example {
  file: string
  line: number
  first: number
  text: string
  in: string
  how: string
  pkg: string
  test?: boolean
}
interface Examples {
  name: string
  total: number
  items: Example[]
}

const ex = ref<Examples | null>(null)
const error = ref('')
const loading = ref(false)

async function load(n: number) {
  loading.value = true
  try {
    ex.value = await call<Examples>('examples', { key: props.symbol, n })
    error.value = ''
  } catch (err) {
    error.value = message(err)
  } finally {
    loading.value = false
  }
}

onMounted(() => load(3))

function lines(e: Example) {
  return e.text.split('\n').map((t, i) => ({ n: e.first + i, t }))
}
</script>

<template>
  <div class="peek" @mousedown.stop>
    <div class="head">
      <span v-if="ex">How <b>{{ ex.name }}</b> is used<span class="dim"> · {{ ex.items.length }} of {{ ex.total }} places</span></span>
      <span v-else-if="loading" class="dim">Finding how it is used…</span>
      <span class="spacer" />
      <button v-if="ex && ex.items.length < ex.total && ex.items.length < 10" :disabled="loading" @click="load(10)">More</button>
      <button title="Its card: what it is, who uses it, its tests" @click="reveal({ page: { kind: 'card', key: symbol } })"><i class="codicon codicon-info" /></button>
      <button title="Close (Esc)" @click="close"><i class="codicon codicon-close" /></button>
    </div>
    <div v-if="error" class="dim pad">{{ error }}</div>
    <div v-else-if="ex && !ex.items.length" class="dim pad">Nothing in the project uses it yet.</div>
    <div v-for="e in ex?.items" :key="e.file + e.line" class="ex">
      <div class="where" :title="rel(e.file) + ':' + e.line" @click="openAt(e.file, e.line)">
        <i class="codicon codicon-go-to-file" />
        <span class="fn">{{ e.in }}</span>
        <span class="dim">{{ e.pkg }} · {{ e.how }}</span>
        <span v-if="e.test" class="tag">test</span>
        <span class="file dim">{{ rel(e.file) }}:{{ e.line }}</span>
      </div>
      <pre class="code" @click="openAt(e.file, e.line)"><span v-for="l in lines(e)" :key="l.n" class="l" :class="{ hit: l.n === e.line }"><span class="n">{{ l.n }}</span>{{ l.t || ' ' }}</span></pre>
    </div>
  </div>
</template>

<style scoped>
.peek {
  margin: 2px 0 6px;
  border: 1px solid var(--border-strong);
  border-left: 3px solid var(--accent);
  border-radius: 4px;
  background: var(--bg-raised);
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--text-dim);
  max-height: 340px;
  overflow: auto;
  white-space: normal;
}
.head {
  position: sticky;
  top: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  background: var(--bg-raised);
  border-bottom: 1px solid var(--border);
  color: var(--text);
}
.head b {
  font-family: var(--font-mono);
  font-weight: 600;
}
.spacer {
  flex: 1;
}
button {
  padding: 1px 6px;
  border-radius: 3px;
  color: var(--text-dim);
  cursor: pointer;
}
button:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.dim {
  color: var(--text-faint);
}
.pad {
  padding: 8px;
}
.ex + .ex {
  border-top: 1px solid var(--border);
}
.where {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px 2px;
  cursor: pointer;
}
.where:hover .fn {
  text-decoration: underline;
}
.fn {
  color: var(--text);
  font-family: var(--font-mono);
}
.file {
  margin-left: auto;
  font-size: 11px;
}
.tag {
  padding: 0 4px;
  border-radius: 3px;
  background: var(--bg-hover);
  font-size: 10px;
}
.code {
  margin: 0;
  padding: 2px 0 6px;
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-dim);
  white-space: pre;
  overflow-x: auto;
  cursor: pointer;
}
.l {
  display: block;
  padding-right: 8px;
}
.l.hit {
  background: var(--accent-dim, rgba(239, 227, 203, 0.12));
  color: var(--text);
}
.n {
  display: inline-block;
  width: 44px;
  padding-right: 10px;
  text-align: right;
  color: var(--text-faint);
  user-select: none;
}
</style>
