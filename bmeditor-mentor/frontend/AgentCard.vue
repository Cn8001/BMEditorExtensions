<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { agent, api, diff, icons, name, openAt, rel, store, type Change } from './api'
import ArchView from './ArchView.vue'
import Section from './Section.vue'

// The agent's plan and milestones, at the top of the Changes tab. It is
// simple on purpose: the agent proposes its plan, you approve it; it works
// on its own to the end of a step, a milestone, and tells you what it did
// and where that sits in the project; you go on. At the plan and at a
// milestone you can ask it a question, which it answers without changing
// anything, or tell it what to change. The code is a click away, when you
// want it.
const st = computed(() => store.agent)
const cp = computed(() => st.value?.checkpoint)
const text = ref('')
const busy = ref(false)
const output = ref(false)
// A milestone's two tabs: what the agent says and the files it changed, and
// what the step did to the project's shape.
const view = ref<'summary' | 'arch'>('summary')
watch(
  () => cp.value?.id,
  () => (view.value = 'summary'),
)

// The step's files, each with the declarations it changed in it: the ones
// the agent's edits touched, Go or not, a click from the code.
const stepFiles = computed(() => {
  const changes = (cp.value?.report?.units ?? []).flatMap((u) => u.changes)
  const folder = (api.ide.folder || '').replace(/[\\/]+$/, '')
  const out = (st.value?.files ?? [])
    .slice()
    .sort()
    .map((r) => {
      const file = folder + '/' + r
      const own = changes.filter((c) => c.file.replace(/\\/g, '/').endsWith('/' + r)).sort((a, b) => a.line - b.line)
      return { rel: r, file: own[0]?.file ?? file, changes: own }
    })
  return out
})
const mark: Record<string, string> = { added: '+', modified: '~', removed: '−' }

function openChange(f: { file: string }, c: Change) {
  if (c.change === 'removed') void diff(f.file)
  else void openAt(c.file, c.line)
}

const level = computed(() => {
  switch (st.value?.level) {
    case 'off':
      return { label: 'Mentor is off', title: 'Connect the agent is off: the agent works without plans or milestones.' }
    case 'enforced':
      return { label: 'Connected', title: 'The agent stops at each milestone and waits for you.' }
    case 'cooperative':
      return { label: 'Connected without its hooks', title: 'The agent has the tools but not the hooks: it can go on without waiting.' }
  }
  return { label: 'Not connected', title: 'Connecting the agent did not work: Mentor shows what changed, and does not pause anything.' }
})

const step = computed(() => (st.value?.plan && st.value.step ? st.value.plan.steps[st.value.step - 1] : null))
const talk = computed(() => st.value?.talk ?? [])
const asking = computed(() => talk.value.length > 0 && !talk.value[talk.value.length - 1].a)

function list(xs: string[] | null | undefined) {
  return (xs ?? []).join(', ')
}

// setOn turns Mentor's hold on the agent on or off: the setting Connect the
// agent, which also adds or takes out its hooks.
async function setOn(on: boolean) {
  await api.saveSettings?.({ watchAgent: on ? 'true' : 'false' })
  await agent('state')
}

async function act(fn: () => Promise<void>) {
  busy.value = true
  try {
    await fn()
    text.value = ''
  } finally {
    busy.value = false
  }
}

const go = () =>
  act(() => (st.value?.waiting === 'plan' ? agent('plan', { approve: true, note: '' }) : agent('checkpoint', { answer: 'approve', note: '' })))
const ask = () => act(() => agent('ask', { note: text.value }))
const change = () =>
  act(() => (st.value?.waiting === 'plan' ? agent('plan', { approve: false, note: text.value }) : agent('checkpoint', { answer: 'changes', note: text.value })))

function time(at: string) {
  return new Date(at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div class="agent">
    <div class="line" :title="level.title">
      <span class="dot" :class="st?.level || 'observe'" />
      <span>Agent: {{ level.label }}</span>
      <button v-if="st?.level !== 'enforced' && st?.level !== 'off'" class="link" title="Mentor's settings" @click="api.ui.openSettings()">Settings</button>
      <button v-if="api.saveSettings" class="link" :title="st?.level === 'off' ? 'Connect the agent again' : 'Let the agent work without Mentor'" @click="setOn(st?.level === 'off')">
        {{ st?.level === 'off' ? 'Turn on' : 'Turn off' }}
      </button>
    </div>
    <div v-if="st?.problem" class="setup">
      <div class="warn">{{ st.problem.split('\n')[0] }}</div>
      <code v-for="cmd in st.problem.split('\n').slice(1)" :key="cmd" class="cmd">{{ cmd }}</code>
    </div>

    <div v-if="st?.waiting && (st.plan || cp)" class="card wait">
      <!-- The plan. -->
      <template v-if="st.waiting === 'plan' && st.plan">
        <div class="head">The agent's plan <span class="badge">waits for you</span></div>
        <div class="task">{{ st.plan.task }}</div>
        <ol class="steps">
          <li v-for="s in st.plan.steps" :key="s.n">
            <b>{{ s.title }}</b><template v-if="s.does"> — {{ s.does }}</template>
            <div v-if="s.files.length || s.packages.length" class="pfiles">
              <span v-for="f in s.files" :key="f" class="pf" :class="s.new?.includes(f) ? 'add' : 'chg'" :title="s.new?.includes(f) ? 'It will add this file' : 'It will change this file'">
                {{ s.new?.includes(f) ? '+' : '~' }} {{ f }}
              </span>
              <span v-for="p in s.packages" :key="p" class="pf dir" title="A folder it will change">▸ {{ p }}/</span>
            </div>
            <div v-if="s.avoid?.length || s.doneWhen" class="dim">
              <template v-if="s.avoid?.length">Leaves alone {{ list(s.avoid) }}. </template>
              <template v-if="s.doneWhen">Done when {{ s.doneWhen }}</template>
            </div>
            <div v-if="s.unknown?.length || s.exists?.length" class="warn small">
              <template v-if="s.unknown?.length">The code has no {{ list(s.unknown) }}. </template>
              <template v-if="s.exists?.length">The code already has {{ list(s.exists) }}.</template>
            </div>
          </li>
        </ol>
      </template>

      <!-- A milestone: what the agent did, and the project's shape now. -->
      <template v-else-if="cp">
        <div class="head">
          Milestone<template v-if="st.plan"> {{ cp.step }}/{{ st.plan.steps.length }}</template><template v-if="step">: {{ step.title }}</template>
          <span class="badge">waits for you</span>
        </div>
        <div class="tabs2">
          <button :class="{ on: view === 'summary' }" @click="view = 'summary'">Summary</button>
          <button :class="{ on: view === 'arch' }" title="What this step did to the project's shape" @click="view = 'arch'">Architecture</button>
        </div>
        <template v-if="view === 'summary'">
          <div class="summary">
            <p>{{ cp.made || cp.summary }}</p>
            <p v-if="cp.fits">{{ cp.fits }}</p>
            <p v-if="cp.know" class="know"><span class="dim">To carry on without the agent:</span> {{ cp.know }}</p>
          </div>
          <div v-if="cp.tests" class="tests" :class="cp.tests.state">
            <template v-if="cp.tests.state === 'running'"><i class="codicon codicon-loading codicon-modifier-spin" /> Running its tests…</template>
            <template v-else-if="cp.tests.state === 'passed'"><i class="codicon codicon-pass" /> Tests pass</template>
            <template v-else-if="cp.tests.state === 'failed'">
              <i class="codicon codicon-error" /> Tests fail<template v-if="cp.tests.failed.length">: {{ cp.tests.failed.join(', ') }}</template>
              <button class="link" @click="output = !output">{{ output ? 'Hide' : 'Why' }}</button>
            </template>
            <template v-else><i class="codicon codicon-circle-slash" /> No tests for what changed</template>
          </div>
          <pre v-if="output && cp.tests?.output" class="out">{{ cp.tests.output }}</pre>
          <div v-if="cp.avoided?.length || cp.unknown.length" class="warn small pad">
            <template v-if="cp.avoided?.length">It changed {{ list(cp.avoided) }}, which it said it would leave alone. </template>
            <template v-if="cp.unknown.length">It names {{ cp.unknown.join(', ') }}, which the code does not have.</template>
          </div>
          <div class="files">
            <div class="dim pad">Files it changed</div>
            <div v-for="f in stepFiles" :key="f.rel">
              <div class="frow" :title="'Beside HEAD: ' + f.rel" @click="diff(f.file)">
                <i class="codicon codicon-diff" />
                <span class="fname">{{ f.rel.slice(f.rel.lastIndexOf('/') + 1) }}</span>
                <span class="fdir">{{ f.rel.includes('/') ? f.rel.slice(0, f.rel.lastIndexOf('/')) : '' }}</span>
              </div>
              <div v-for="c in f.changes" :key="c.key + c.change" class="crow" :class="c.change" :title="rel(c.file) + ':' + c.line" @click="openChange(f, c)">
                <span class="mark">{{ mark[c.change] }}</span>
                <i :class="['codicon', 'codicon-' + (icons[c.kind] || 'symbol-misc')]" />
                <span>{{ name(c) }}</span>
              </div>
            </div>
            <div v-if="!stepFiles.length" class="dim pad">No file yet.</div>
          </div>
        </template>
        <div v-else class="shape">
          <div class="dim">What this milestone changed in the project's shape</div>
          <ArchView v-if="cp.arch?.packages.length" compact :change="cp.arch" />
          <div v-else class="dim">{{ cp.arch ? 'It changed no Go package.' : 'Reading what it changed…' }}</div>
        </div>
      </template>

      <!-- Questions and answers. -->
      <div v-for="(t, i) in talk" :key="i" class="talk">
        <div class="q"><i class="codicon codicon-comment" /> {{ t.q }}</div>
        <div v-if="t.a" class="a">{{ t.a }}</div>
        <div v-else class="a dim"><i class="codicon codicon-loading codicon-modifier-spin" /> The agent is answering…</div>
      </div>

      <textarea v-model="text" rows="2" placeholder="Ask about it, or say what to change" @keydown.ctrl.enter="text.trim() && !asking && ask()" />
      <div class="buttons">
        <button class="primary" :disabled="busy || asking" @click="go">
          <i class="codicon" :class="st.waiting === 'plan' ? 'codicon-check' : 'codicon-debug-continue'" /> {{ st.waiting === 'plan' ? 'Approve' : 'Continue' }}
        </button>
        <button :disabled="busy || asking || !text.trim()" title="The agent answers, and changes nothing (Ctrl+Enter)" @click="ask">Ask</button>
        <button :disabled="busy || asking || !text.trim()" title="The agent makes the change, then stops here again" @click="change">Change it</button>
      </div>
    </div>

    <!-- A plan under way. -->
    <div v-else-if="st?.plan && st.plan.status === 'approved'" class="card">
      <div class="head">{{ st.plan.task }}</div>
      <ol class="steps">
        <li v-for="s in st.plan.steps" :key="s.n" :class="s.status">
          <i :class="['codicon', s.status === 'done' ? 'codicon-pass' : s.status === 'doing' ? 'codicon-play-circle' : 'codicon-circle-large']" />
          {{ s.title }}
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
  font-size: 13px;
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
  font-size: 13px;
}
.link + .link {
  margin-left: 10px;
}
.setup {
  margin: 0 8px 6px;
  font-size: 13px;
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
  font-size: 14px;
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
  font-size: 12px;
  font-weight: 400;
}
.task {
  padding: 0 8px 4px;
  font-size: 14px;
  line-height: 1.4;
}
.steps {
  font-size: 14px;
  margin: 0;
  padding: 0 8px 4px 26px;
  line-height: 1.45;
}
.steps li {
  margin-bottom: 4px;
}
.steps li.done {
  color: var(--text-faint);
}
.steps li.doing {
  font-weight: 600;
}
.summary {
  font-size: 14px;
  padding: 0 10px;
  line-height: 1.5;
  color: var(--text);
}
.summary p {
  margin: 0 0 6px;
  white-space: pre-wrap;
}
.summary .know {
  padding-left: 8px;
  border-left: 2px solid var(--accent);
}
.shape {
  margin: 4px 8px;
  padding: 4px 0;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}
.tests {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  font-size: 13px;
  color: var(--text-dim);
}
.tests.passed {
  color: var(--success, #b8d27a);
}
.tests.failed {
  color: var(--error);
}
.out {
  max-height: 180px;
  overflow: auto;
  margin: 4px 8px;
  padding: 4px 6px;
  background: var(--bg);
  font-family: var(--font-mono);
  font-size: 13px;
  white-space: pre-wrap;
  user-select: text;
}
.pfiles {
  display: flex;
  flex-direction: column;
  margin: 2px 0 2px 2px;
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 400;
}
.pf.add {
  color: var(--success, #b8d27a);
}
.pf.chg {
  color: var(--warning);
}
.pf.dir {
  color: var(--text-dim);
}
.tabs2 {
  display: flex;
  gap: 2px;
  margin: 0 8px 6px;
  border-bottom: 1px solid var(--border);
}
.tabs2 button {
  padding: 3px 10px;
  border-bottom: 2px solid transparent;
  color: var(--text-dim);
  font-size: 13px;
}
.tabs2 button.on {
  color: var(--text);
  border-bottom-color: var(--accent);
}
.files {
  margin-top: 4px;
}
.frow,
.crow {
  display: flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
}
.frow {
  padding: 3px 10px;
  font-size: 14px;
}
.crow {
  padding: 1px 10px 1px 28px;
  font-size: 13px;
}
.frow:hover,
.crow:hover {
  background: var(--bg-hover);
}
.fname {
  color: var(--text);
}
.fdir {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-faint);
  font-size: 12px;
}
.mark {
  width: 10px;
  text-align: center;
}
.crow.added .mark {
  color: var(--success, #b8d27a);
}
.crow.modified .mark {
  color: var(--warning);
}
.crow.removed {
  color: var(--error);
}
.more {
  display: flex;
  align-items: center;
  gap: 3px;
  margin: 2px 6px;
  color: var(--text-dim);
  font-size: 13px;
}
.talk {
  font-size: 14px;
  margin: 6px 8px 0;
  line-height: 1.45;
}
.talk .q {
  color: var(--text);
}
.talk .q .codicon {
  color: var(--accent);
}
.talk .a {
  margin: 2px 0 0 18px;
  padding-left: 8px;
  border-left: 2px solid var(--border-strong);
  color: var(--text-dim);
  white-space: pre-wrap;
}
.warn {
  color: var(--warning);
}
.small {
  font-size: 13px;
}
.pad {
  padding: 2px 10px;
}
.dim {
  color: var(--text-faint);
  font-size: 13px;
}
textarea {
  display: block;
  width: calc(100% - 16px);
  margin: 8px 8px 4px;
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
  font-size: 13px;
}
.buttons button:hover:not(:disabled) {
  border-color: var(--accent);
}
.buttons button:disabled {
  opacity: 0.5;
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
  gap: 6px;
  padding: 1px 8px 1px 14px;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
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
  color: var(--warning);
}
</style>
