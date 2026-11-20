<script setup lang="ts">
import { computed, markRaw, ref, watch } from 'vue'
import { agent, api, call, go, icons, name, store, type SearchResult } from './api'
import Section from './Section.vue'
import UnitCard from './UnitCard.vue'

// The coding agent's plan and checkpoints, at the top of the Changes tab. The
// agent works on its own; at the end of each step it waits here, and the
// developer sees what the step did before letting it go on.
const st = computed(() => store.agent)
const note = ref('')
const related = ref<SearchResult | null>(null)

const level = computed(() => {
  switch (st.value?.level) {
    case 'off':
      return { label: 'Mentor is off', title: 'Pause the agent at each step is off in Settings: the agent works without plans or checkpoints.' }
    case 'enforced':
      return { label: 'Checkpoints hold', title: 'The agent waits at each checkpoint: its hooks hold edits until you answer here.' }
    case 'cooperative':
      return { label: 'Checkpoints, not held', title: 'The agent has the tools but not the hooks: it can go on without waiting.' }
  }
  return { label: 'Not connected', title: 'Connect the agent is off in Settings, or connecting did not work: Mentor shows what changed, and does not pause anything.' }
})

const step = computed(() => (st.value?.plan && st.value.step ? st.value.plan.steps[st.value.step - 1] : null))

// Preflight: what the code already has for the task, before any change.
watch(
  () => (st.value?.waiting === 'plan' ? st.value.plan?.task : ''),
  async (task) => {
    related.value = null
    if (!task) return
    try {
      related.value = markRaw(await call<SearchResult>('search', { q: task, limit: 6 }))
    } catch {
      related.value = null
    }
  },
  { immediate: true },
)

async function answerPlan(approve: boolean) {
  await agent('plan', { approve, note: note.value })
  note.value = ''
}

async function answerCheckpoint(answer: 'approve' | 'changes' | 'skip') {
  await agent('checkpoint', { answer, note: note.value })
  note.value = ''
}

function touches(s: { files: string[]; packages: string[] }) {
  return [...s.files, ...s.packages].join(', ')
}

function time(at: string) {
  const d = new Date(at)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div class="agent">
    <div class="line" :title="level.title">
      <span class="dot" :class="st?.level || 'observe'" />
      <span>Agent: {{ level.label }}</span>
      <button v-if="st?.level !== 'enforced'" class="link" title="Connect the agent, in Mentor's settings" @click="api.ui.openSettings()">Settings</button>
    </div>
    <div v-if="st?.problem" class="setup">
      <div class="warn">{{ st.problem.split('\n')[0] }}</div>
      <code v-for="cmd in st.problem.split('\n').slice(1)" :key="cmd" class="cmd">{{ cmd }}</code>
    </div>

    <!-- The plan waits. -->
    <div v-if="st?.waiting === 'plan' && st.plan" class="card wait">
      <div class="head"><i class="codicon codicon-checklist" /> The agent's plan <span class="badge">waits for you</span></div>
      <div class="task">{{ st.plan.task }}</div>
      <ol class="steps">
        <li v-for="s in st.plan.steps" :key="s.n">
          <div>{{ s.title }}</div>
          <div v-if="s.why" class="dim">{{ s.why }}</div>
          <div v-if="touches(s)" class="dim"><i class="codicon codicon-file-code" /> {{ touches(s) }}</div>
        </li>
      </ol>
      <Section v-if="related" title="What the code has for it" :count="related.candidates.length">
        <div v-for="c in related.candidates" :key="c.key" class="rel" @click="go({ kind: 'card', key: c.key })">
          <i :class="['codicon', 'codicon-' + (icons[c.kind] || 'symbol-misc')]" />
          <span>{{ name(c) }}</span>
          <span class="dim">{{ c.pkgName }} · {{ c.responsibility }}</span>
        </div>
      </Section>
      <textarea v-model="note" rows="2" placeholder="A note for the agent (optional)" />
      <div class="buttons">
        <button class="primary" @click="answerPlan(true)"><i class="codicon codicon-check" /> Approve</button>
        <button @click="answerPlan(false)">Ask for changes</button>
      </div>
    </div>

    <!-- A checkpoint waits. -->
    <div v-else-if="st?.waiting === 'checkpoint' && st.checkpoint" class="card wait">
      <div class="head">
        <i class="codicon codicon-debug-pause" /> Step {{ st.checkpoint.step }}<template v-if="step">: {{ step.title }}</template>
        <span class="badge">waits for you</span>
      </div>
      <div class="said">
        <div class="who">The agent says it</div>
        <div class="text">{{ st.checkpoint.summary }}</div>
        <template v-if="st.checkpoint.why">
          <div class="who">and why</div>
          <div class="text">{{ st.checkpoint.why }}</div>
        </template>
        <div v-if="st.checkpoint.unknown.length" class="warn">
          <i class="codicon codicon-warning" /> It names {{ st.checkpoint.unknown.join(', ') }}, which the code does not have.
        </div>
      </div>
      <div class="who pad">What the code shows changed</div>
      <div v-if="!st.checkpoint.report" class="dim pad">Reading the changes…</div>
      <div v-else-if="!st.checkpoint.report.units.length" class="dim pad">No Go declaration changed in this step.</div>
      <UnitCard v-for="u in st.checkpoint.report?.units" :key="u.id" :u="u" no-review />
      <textarea v-model="note" rows="2" placeholder="What to change, or a note for the agent" />
      <div class="buttons">
        <button class="primary" title="Mark it reviewed and let the agent go on" @click="answerCheckpoint('approve')">
          <i class="codicon codicon-debug-continue" /> Approve and go on
        </button>
        <button title="The agent stays on this step and makes the changes in your note" @click="answerCheckpoint('changes')">Ask for changes</button>
        <button class="quiet" title="Go on without reviewing it" @click="answerCheckpoint('skip')">Skip</button>
      </div>
    </div>

    <!-- A plan under way. -->
    <div v-else-if="st?.plan && st.plan.status === 'approved'" class="card">
      <div class="head"><i class="codicon codicon-checklist" /> {{ st.plan.task }}</div>
      <ol class="steps">
        <li v-for="s in st.plan.steps" :key="s.n" :class="s.status">
          <i :class="['codicon', s.status === 'done' ? 'codicon-pass' : s.status === 'doing' ? 'codicon-play-circle' : 'codicon-circle-large']" />
          {{ s.title }}
          <span v-if="s.status === 'doing' && st.files.length" class="dim">· {{ st.files.length }} file{{ st.files.length === 1 ? '' : 's' }} so far</span>
        </li>
      </ol>
      <div class="buttons"><button class="quiet" @click="agent('end')">End the plan</button></div>
    </div>

    <Section v-if="st?.events?.length" title="What the agent did" :count="st.events.length" :open="false">
      <div v-for="(e, i) in [...st.events].reverse().slice(0, 30)" :key="i" class="event" :class="e.result" :title="e.reason">
        <span class="t">{{ time(e.at) }}</span>
        <span class="tool">{{ e.tool }}</span>
        <span class="what">{{ e.what }}</span>
        <span v-if="e.result !== 'done'" class="res">{{ e.result }}</span>
      </div>
    </Section>
  </div>
</template>

<style scoped>
.agent {
  border-bottom: 1px solid var(--border);
  padding-bottom: 4px;
}
.line {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 6px 8px;
  font-size: 12px;
  color: var(--text-dim);
}
.dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-faint);
}
.dot.enforced {
  background: var(--success, #b8d27a);
}
.dot.cooperative {
  background: var(--warning);
}
.link {
  margin-left: auto;
  color: var(--accent);
  font-size: 12px;
}
.setup {
  margin: 0 8px 6px;
  font-size: 11px;
  line-height: 1.5;
  color: var(--text-dim);
}
.cmd {
  display: block;
  margin: 3px 0;
  padding: 3px 5px;
  border-radius: 3px;
  background: var(--bg-raised);
  font-family: var(--font-mono);
  word-break: break-all;
  user-select: text;
}
.card {
  margin: 4px 6px 8px;
  padding: 6px 0;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--bg-raised);
}
.card.wait {
  border-color: var(--accent);
}
.head {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 8px 4px;
  font-weight: 600;
}
.badge {
  margin-left: auto;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--accent-dim);
  color: var(--text);
  font-size: 10px;
  font-weight: 400;
}
.task {
  padding: 0 8px 4px;
  line-height: 1.4;
}
.steps {
  margin: 0;
  padding: 0 8px 4px 26px;
  line-height: 1.45;
}
.steps li {
  margin-bottom: 3px;
}
.steps li.done {
  color: var(--text-faint);
}
.steps li.doing {
  font-weight: 600;
}
.said {
  margin: 0 8px 6px;
  padding: 5px 7px;
  border-left: 2px solid var(--border-strong);
}
.who {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-faint);
}
.who.pad,
.dim.pad {
  padding: 2px 8px;
}
.text {
  margin-bottom: 4px;
  line-height: 1.45;
  white-space: pre-wrap;
}
.warn {
  color: var(--warning);
  font-size: 12px;
}
.dim {
  color: var(--text-faint);
  font-size: 11px;
}
.rel {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px 2px 14px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
}
.rel:hover {
  background: var(--bg-hover);
}
.rel .dim {
  overflow: hidden;
  text-overflow: ellipsis;
}
textarea {
  display: block;
  width: calc(100% - 16px);
  margin: 6px 8px 4px;
  padding: 4px 6px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
  font: inherit;
  resize: vertical;
}
.buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 2px 8px;
}
.buttons button {
  display: flex;
  align-items: center;
  gap: 3px;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--bg);
  font-size: 12px;
}
.buttons button:hover {
  border-color: var(--accent);
}
.buttons .primary {
  border-color: var(--accent);
  background: var(--accent);
  color: var(--bg);
}
.buttons .quiet {
  margin-left: auto;
  border-color: transparent;
  color: var(--text-dim);
}
.event {
  display: flex;
  gap: 5px;
  padding: 1px 8px 1px 14px;
  font-size: 11px;
  white-space: nowrap;
}
.event .t {
  color: var(--text-faint);
}
.event .tool {
  color: var(--text-dim);
}
.event .what {
  overflow: hidden;
  text-overflow: ellipsis;
}
.event.held .res {
  margin-left: auto;
  color: var(--warning);
}
</style>
