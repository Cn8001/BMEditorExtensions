<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { call, message, openAt, rel, reveal } from './api'

// Every place a symbol is used, under its lens, as an editor lists
// references: grouped by file, each line with its number and text and the
// name marked, a click away. The backend answers a page at a time and the
// next one loads as the list is scrolled to its end, so a symbol used a
// million times takes no longer to open than one used once.
const props = defineProps<{ symbol: string; close: () => void }>()

interface Use {
  file: string
  line: number
  col?: number
  text: string
  indent: number
  name: string
  in?: string
  test?: boolean
}
interface Uses {
  name: string
  total: number
  files: number
  offset: number
  items: Use[]
}

const page = 100
const name = ref('')
const total = ref(0)
const files = ref(0)
const items = ref<Use[]>([])
const loading = ref(false)
const error = ref('')

async function more() {
  if (loading.value || (total.value && items.value.length >= total.value)) return
  loading.value = true
  try {
    const u = await call<Uses>('refs', { key: props.symbol, offset: items.value.length, limit: page })
    name.value = u.name
    total.value = u.total
    files.value = u.files
    items.value = items.value.concat(u.items)
    error.value = ''
  } catch (err) {
    error.value = message(err)
  } finally {
    loading.value = false
  }
}

onMounted(more)

// The page's uses, file by file, in the order the backend gave them.
const groups = computed(() => {
  const out: { file: string; uses: Use[] }[] = []
  for (const u of items.value) {
    const last = out[out.length - 1]
    if (last && last.file === u.file) last.uses.push(u)
    else out.push({ file: u.file, uses: [u] })
  }
  return out
})

// parts splits a line at the name, to mark it: where the column says, else
// where the name first is.
function parts(u: Use): [string, string, string] {
  let at = u.col ? u.col - 1 - u.indent : -1
  if (at < 0 || !u.text.startsWith(u.name, at)) at = u.text.indexOf(u.name)
  if (at < 0) return [u.text, '', '']
  return [u.text.slice(0, at), u.name, u.text.slice(at + u.name.length)]
}

function scrolled(e: Event) {
  const el = e.target as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 80) void more()
}
</script>

<template>
  <div class="peek" @mousedown.stop @scroll="scrolled">
    <div class="head">
      <span v-if="name"
        ><b>{{ name }}</b
        ><span class="dim"> · {{ total }} use{{ total === 1 ? '' : 's' }} in {{ files }} file{{ files === 1 ? '' : 's' }}</span></span
      >
      <span v-else-if="loading" class="dim">Finding its uses…</span>
      <span class="spacer" />
      <button title="Its card: what it is, who uses it, its tests" @click="reveal({ page: { kind: 'card', key: symbol } })"><i class="codicon codicon-info" /></button>
      <button title="Close (Esc)" @click="close"><i class="codicon codicon-close" /></button>
    </div>
    <div v-if="error" class="dim pad">{{ error }}</div>
    <div v-else-if="name && !total" class="dim pad">Nothing in the project uses it yet.</div>
    <div v-for="g in groups" :key="g.file" class="group">
      <div class="file" :title="g.file">
        <i class="codicon codicon-file" /> {{ rel(g.file) }} <span class="dim">{{ g.uses.length }}</span>
      </div>
      <div v-for="u in g.uses" :key="u.line + ':' + (u.col || 0)" class="use" :class="{ test: u.test }" :title="u.in ? 'in ' + u.in : ''" @click="openAt(u.file, u.line, u.col || 1)">
        <span class="n">{{ u.line }}</span>
        <span class="t"
          >{{ parts(u)[0] }}<mark>{{ parts(u)[1] }}</mark>{{ parts(u)[2] }}</span
        >
      </div>
    </div>
    <div v-if="items.length < total" class="more">
      <button :disabled="loading" @click="more">{{ loading ? 'Loading…' : `Show more · ${items.length} of ${total}` }}</button>
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
  font-size: 13px;
  color: var(--text-dim);
  max-height: 360px;
  overflow: auto;
  white-space: normal;
}
.head {
  position: sticky;
  top: 0;
  z-index: 1;
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
.file {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 8px 2px;
  color: var(--text);
  font-size: 12px;
}
.use {
  display: flex;
  gap: 10px;
  padding: 1px 8px 1px 14px;
  font-family: var(--font-mono);
  font-size: 12px;
  white-space: pre;
  cursor: pointer;
}
.use:hover {
  background: var(--bg-hover);
}
.use.test {
  opacity: 0.7;
}
.n {
  flex: none;
  width: 36px;
  text-align: right;
  color: var(--text-faint);
}
.t {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-dim);
}
mark {
  background: var(--accent-dim, rgba(239, 227, 203, 0.18));
  color: var(--text);
  border-radius: 2px;
}
.more {
  padding: 4px 8px 6px;
}
</style>
