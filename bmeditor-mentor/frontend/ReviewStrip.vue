<script setup lang="ts">
import { computed, ref } from 'vue'
import { agent, diff, markReviewed, name, openAt, reveal, store, type Change } from './api'
import { fileOf, live, reviewed, reviewAndNext, step } from './review'

// The strip above a file the agent changed: the declarations it changed
// here, each a click away; F7 on to the next change; Reviewed; and, while
// the agent waits at a checkpoint, its answer.
const props = defineProps<{ path: string }>()

const f = computed(() => fileOf(props.path))
const files = computed(() => store.report?.files ?? [])
const at = computed(() => (f.value ? files.value.indexOf(f.value) + 1 : 0))
const done = computed(() => !!f.value && reviewed(f.value))
const mark: Record<string, string> = { added: '+', modified: '~', removed: '−' }
const cp = computed(() => (store.agent?.waiting === 'checkpoint' ? store.agent.checkpoint : null))
const busy = ref(false)

function open(c: Change) {
  if (c.change === 'removed') void diff(props.path)
  else void openAt(c.file, c.line)
}

async function toggleReviewed() {
  if (!f.value) return
  if (done.value) await markReviewed(live(f.value).map((c) => c.key), false)
  else await reviewAndNext()
}

async function approve() {
  busy.value = true
  try {
    await agent('checkpoint', { answer: 'approve', note: '' })
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="f" class="mentor-strip" :class="{ done }">
    <i class="codicon codicon-mortar-board" title="Mentor" />
    <span class="what">{{ f.status === 'A' ? 'The agent added this file' : f.status === 'D' ? 'The agent deleted this file' : 'The agent changed this file' }}</span>
    <span class="nums"><span class="add">+{{ f.added }}</span> <span class="del">−{{ f.deleted }}</span></span>
    <span class="chips">
      <button v-for="c in f.changes" :key="c.key + c.change" class="chip" :class="c.change" :title="c.change + ': ' + (c.shape || c.oldShape || name(c))" @click="open(c)">
        <span class="mark">{{ mark[c.change] }}</span>{{ name(c) }}
      </button>
    </span>
    <span class="spacer" />
    <span class="pos" :title="'File ' + at + ' of the ' + files.length + ' the agent changed'">{{ at }}/{{ files.length }}</span>
    <button class="icon" title="The change before (Shift+F7)" @click="step(-1)"><i class="codicon codicon-arrow-up" /></button>
    <button class="icon" title="The next change (F7)" @click="step(1)"><i class="codicon codicon-arrow-down" /></button>
    <button v-if="f.status !== 'A' && f.status !== 'D'" title="Beside HEAD" @click="diff(path)"><i class="codicon codicon-diff" /> Diff</button>
    <button v-if="live(f).length" :class="{ ok: done }" :title="done ? 'Reviewed as it is now; click to unmark' : 'Mark it reviewed and go to the next file (Alt+R)'" @click="toggleReviewed">
      <i class="codicon" :class="done ? 'codicon-pass-filled' : 'codicon-check'" /> {{ done ? 'Reviewed' : 'Reviewed (Alt+R)' }}
    </button>
    <template v-if="cp">
      <span class="sep" />
      <span class="wait">Step {{ cp.step }} waits</span>
      <button class="primary" :disabled="busy" title="Approve the step: the agent goes on" @click="approve"><i class="codicon codicon-check-all" /> Approve</button>
      <button title="Ask for changes, in Mentor" @click="reveal({ home: true })">Changes…</button>
    </template>
  </div>
</template>

<style scoped>
.mentor-strip {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 3px 8px;
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--text-dim);
  white-space: nowrap;
  border-left: 3px solid var(--accent);
}
.mentor-strip.done {
  border-left-color: var(--success, #b8d27a);
}
.what {
  color: var(--text);
}
.nums {
  font-size: 11px;
}
.add {
  color: var(--success, #b8d27a);
}
.del {
  color: var(--error);
}
.chips {
  display: flex;
  gap: 4px;
  min-width: 0;
  overflow: hidden;
}
.chip {
  flex: none;
  padding: 0 6px;
  border-radius: 3px;
  background: var(--bg-hover);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 11px;
}
.chip .mark {
  margin-right: 3px;
}
.chip.added .mark {
  color: var(--success, #b8d27a);
}
.chip.modified .mark {
  color: var(--warning);
}
.chip.removed {
  color: var(--error);
  text-decoration: line-through;
}
.spacer {
  flex: 1;
}
.pos {
  font-size: 11px;
  color: var(--text-faint);
}
button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 3px;
  color: var(--text-dim);
}
button:hover {
  background: var(--bg-hover);
  color: var(--text);
}
button.ok {
  color: var(--success, #b8d27a);
}
button.primary {
  background: var(--accent);
  color: var(--accent-ink, #1d1b19);
}
.sep {
  width: 1px;
  height: 14px;
  background: var(--border-strong);
}
.wait {
  color: var(--accent);
}
</style>
