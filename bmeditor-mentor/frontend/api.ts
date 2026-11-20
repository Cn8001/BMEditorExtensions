import { markRaw, reactive } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'

// The editor's API, kept from activate for the components, and what the
// backend (cmd/mentor-host) has said.
export let api: ExtensionAPI

export function setApi(a: ExtensionAPI) {
  api = a
}

export const view = 'mentor'

// What the backend sends, as cmd/mentor-host and internal/* write it.

export interface Status {
  root: string
  state: 'indexing' | 'ready' | 'stale' | 'error'
  detail?: string
  packages: number
  symbols: number
  files: number
  built?: string
  tookMs: number
  head: { commit: string; branch: string }
  errors: string[]
  semantic: string
}

export interface Candidate {
  key: string
  name: string
  kind: string
  pkg: string
  pkgName: string
  recv?: string
  file: string
  line: number
  col: number
  sig: string
  score: number
  responsibility: string
  derived: boolean
  matched: string[]
  uses: number
  tests: number
  usedIn: string[]
  differs: string[]
}

export interface SearchResult {
  query: string
  words: string[]
  expanded: string[]
  semantic: boolean
  candidates: Candidate[]
}

export interface Ref {
  key: string
  name: string
  kind: string
  recv?: string
  pkg: string
  pkgName: string
  file: string
  line: number
  col: number
  sig?: string
  test?: boolean
  via?: string
  certainty?: 'verified' | 'dynamic' | 'inferred'
  siteFile?: string
  siteLine?: number
}

export interface Field {
  name: string
  type: string
  embedded?: boolean
  tag?: string
}

export interface Symbol {
  key: string
  name: string
  kind: string
  pkg: string
  pkgName: string
  recv?: string
  file: string
  line: number
  col: number
  sig: string
  doc?: string
  exported: boolean
  test?: boolean
  returnsError?: boolean
  branches?: number
  errReturns?: number
  fields?: Field[]
}

export interface Route {
  method?: string
  path: string
  handler?: string
  via: string
  file: string
  line: number
}

export interface Card {
  symbol: Symbol
  language: string
  responsibility: string
  derived: boolean
  packageDoc?: string
  owner?: Ref
  fields: Ref[]
  methods: Ref[]
  implements: Ref[]
  implementedBy: Ref[]
  embeds: Ref[]
  embeddedBy: Ref[]
  constructors: Ref[]
  callers: Ref[]
  callees: Ref[]
  usedBy: { pkg: string; refs: Ref[] }[]
  usages: { file: string; line: number; first: number; text: string; in: string }[]
  tests: Ref[]
  indirectTests: Ref[]
  routes: Route[]
  impact: { symbols: number; packages: string[]; tests: number; capped: boolean }
}

export interface Step extends Ref {
  returnsError?: boolean
  errReturns?: number
  branches?: number
  doc?: string
  cycle?: boolean
  truncated?: boolean
  choices?: number
  children: Step[]
}

export interface Flow {
  root: Step
  steps: number
  note: string
}

export interface GNode {
  id: string
  label: string
  kind: string
  doc?: string
  layer?: string
  layerInferred?: boolean
  files?: number
  symbols?: number
  entry?: string[]
  file?: string
  line?: number
  external?: boolean
  errors?: number
}

export interface GEdge {
  from: string
  to: string
  kind: string
  count: number
  certainty: string
}

export interface Graph {
  level: 'packages' | 'package'
  focus?: string
  nodes: GNode[]
  edges: GEdge[]
  note: string
  routes: Route[]
}

export interface Change extends Ref {
  change: 'added' | 'modified' | 'removed'
  shapeChanged?: boolean
  oldShape?: string
  shape?: string
  review?: 'unseen' | 'reviewed' | 'stale'
}

export interface Unit {
  id: string
  title: string
  pkg: string
  pkgName: string
  layer?: string
  changes: Change[]
  files: string[]
  plugsIn: Ref[]
  reliesOn: Ref[]
  tests: Ref[]
  routes: Route[]
  impact: { symbols: number; packages: string[]; tests: number; capped: boolean }
  risks: string[]
  lines: [number, number]
  flows: string[]
  stats: Record<string, number>
}

export interface FileChange {
  path: string
  status: string
  added: number
  deleted: number
}

export interface Report {
  since: string
  head: { commit: string; branch: string }
  units: Unit[]
  other: FileChange[]
  totals: Record<string, number>
  lines: [number, number]
  warning?: string
}

export interface AgentStep {
  n: number
  title: string
  why?: string
  files: string[]
  packages: string[]
  status: 'todo' | 'doing' | 'done'
}

export interface AgentPlan {
  task: string
  steps: AgentStep[]
  status: 'proposed' | 'approved' | 'changes_requested' | 'done'
  note?: string
}

export interface Checkpoint {
  id: number
  step: number
  summary: string
  why?: string
  unknown: string[]
  report: Report | null
  status: 'waiting' | 'approved' | 'changes_requested' | 'skipped'
  note?: string
}

export interface AgentEvent {
  at: string
  tool: string
  what: string
  result: string
  reason?: string
}

// AgentState is the checkpoints of the folder: Claude Code's plan, its
// step, and what waits for the developer.
export interface AgentState {
  level: 'enforced' | 'cooperative' | 'observe'
  hooks: boolean
  mcp: boolean
  lastSeen?: string
  plan: AgentPlan | null
  step: number
  checkpoint: Checkpoint | null
  waiting?: 'plan' | 'checkpoint'
  files: string[]
  events: AgentEvent[] | null
  policy: { requirePlan: boolean; maxFiles: number }
}

export interface Setup {
  hooksFile: string
  hooks: boolean
  mcp: boolean
  mcpCommand: string
  detail?: string
}

// A page of the Explore tab: the answer to a question, a symbol's card,
// a flow or a map. Back goes to the one before.
export type Page = (
  | { kind: 'search' }
  | { kind: 'card'; key: string; card?: Card }
  | { kind: 'flow'; key: string; flow?: Flow }
  | { kind: 'map'; focus: string; graph?: Graph }
) & { from?: 'explore' | 'changes' } // the tab it was opened from, which Back returns to

export const store = reactive({
  status: null as Status | null,
  tab: 'explore' as 'explore' | 'changes',
  report: null as Report | null,
  reportError: '',
  reportBusy: false,
  stale: [] as Ref[],
  agent: null as AgentState | null,
  setup: null as Setup | null,
  query: '',
  result: null as SearchResult | null,
  searching: false,
  pages: [{ kind: 'search' }] as Page[],
  loading: false,
  error: '',
  lens: true,
})

export function page(): Page {
  return store.pages[store.pages.length - 1]
}

export function call<T>(method: string, params?: unknown): Promise<T> {
  return api.call<T>(method, params)
}

export function log(text: string, stream: 'info' | 'stderr' = 'info') {
  api.ui.output(text, stream)
}

export function fail(title: string, err: unknown) {
  const f = api.ui.failureOf(err)
  log(`${title}: ${f.message}`, 'stderr')
  api.ui.toastFailure(title, f)
}

export function message(err: unknown): string {
  return api.ui.failureOf(err).message
}

export async function openAt(file: string, line: number, col = 1) {
  await api.session.openFile(file, { line, col })
}

export function rel(path: string): string {
  return api.ide.folder ? api.paths.relativeTo(api.ide.folder, path) : path
}

export function showSidebar() {
  api.ide.sidebarOpen = true
  api.ide.sidebarView = view
}

// The untyped parts of the editor's store this extension reads: where the
// cursor is (BMEditor 0.1.5 and later have it).
export function cursor(): { line: number; col: number } {
  const c = api.ide.cursor as { line: number; col: number } | undefined
  return c ?? { line: 1, col: 1 }
}

export async function refreshStatus() {
  if (!api.ide.folder) {
    store.status = null
    return
  }
  try {
    store.status = await call<Status>('status')
  } catch {
    store.status = null
  }
}

export async function ask(q: string) {
  store.query = q
  store.pages = [{ kind: 'search' }]
  store.tab = 'explore'
  if (!q.trim()) {
    store.result = null
    return
  }
  store.searching = true
  store.error = ''
  try {
    store.result = markRaw(await call<SearchResult>('search', { q }))
  } catch (err) {
    store.result = null
    store.error = message(err)
  } finally {
    store.searching = false
  }
}

// go opens a page: a card, a flow or a map, loading what it shows.
export async function go(p: Page) {
  if (store.tab === 'changes') {
    // From the Changes tab, a page starts a new trail that Back leaves.
    p.from = 'changes'
    store.pages = [{ kind: 'search' }]
  }
  store.tab = 'explore'
  store.error = ''
  store.pages.push(p)
  store.loading = true
  try {
    if (p.kind === 'card') p.card = markRaw(await call<Card>('card', { key: p.key }))
    else if (p.kind === 'flow') p.flow = markRaw(await call<Flow>('flow', { key: p.key, depth: 4 }))
    else if (p.kind === 'map') p.graph = markRaw(await call<Graph>('arch', { focus: p.focus }))
  } catch (err) {
    store.error = message(err)
  } finally {
    store.loading = false
  }
}

export function back() {
  const top = store.pages.length > 1 ? store.pages.pop() : undefined
  if (top?.from === 'changes' && store.pages.length === 1) store.tab = 'changes'
  store.error = ''
}

export async function loadChanges() {
  if (!api.ide.folder) return
  store.reportBusy = true
  try {
    const [rep, stale] = await Promise.all([call<Report>('changes'), call<Ref[]>('review.stale')])
    store.report = markRaw(rep)
    store.stale = stale
    store.reportError = ''
  } catch (err) {
    store.report = null
    store.reportError = message(err)
  } finally {
    store.reportBusy = false
  }
}

// markReviewed notes that the developer looked at keys as they are now.
export async function markReviewed(keys: string[], reviewed = true) {
  try {
    await call(reviewed ? 'review.mark' : 'review.unmark', { keys })
    await loadChanges()
  } catch (err) {
    fail('Mentor', err)
  }
}

// agent asks the gate something for the developer, and keeps its state.
export async function agent(method: string, params?: unknown) {
  try {
    store.agent = await call<AgentState>('agent.' + method, params)
  } catch (err) {
    fail('Mentor', err)
  }
}

export async function connect(on: boolean) {
  try {
    if (on) store.setup = await call<Setup>('agent.connect')
    else {
      store.setup = null
      await agent('disconnect')
    }
    await agent('state')
  } catch (err) {
    fail('Mentor', err)
  }
}

// diff opens a file beside what HEAD has, in the editor's own review.
export async function diff(path: string) {
  await api.session.openFile(path)
  const tab = api.ide.tabs.find((t) => t.path === path)
  const s = api.session as unknown as { compareWithHead?: (id: string) => void }
  if (tab && s.compareWithHead) s.compareWithHead(tab.id)
}

export function clear() {
  store.agent = null
  store.setup = null
  store.report = null
  store.stale = []
  store.reportError = ''
  store.status = null
  store.result = null
  store.query = ''
  store.pages = [{ kind: 'search' }]
  store.error = ''
}

export function name(r: { name: string; recv?: string }): string {
  return r.recv ? `${r.recv}.${r.name}` : r.name
}

export function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

// icons are the codicons of symbol kinds, as the editor's outline has them.
export const icons: Record<string, string> = {
  struct: 'symbol-structure',
  interface: 'symbol-interface',
  type: 'symbol-class',
  func: 'symbol-function',
  method: 'symbol-method',
  field: 'symbol-field',
  const: 'symbol-constant',
  var: 'symbol-variable',
  package: 'package',
}
