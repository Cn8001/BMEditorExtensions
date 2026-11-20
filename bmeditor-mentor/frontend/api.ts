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
  start?: number // the lines it spans now
  end?: number
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

// AgentFile is a file the agent changed that still differs from HEAD,
// with the declarations changed in it.
export interface AgentFile extends FileChange {
  changes: Change[]
}

export interface AgentChanges {
  head: { commit: string; branch: string }
  files: AgentFile[]
  lines: [number, number]
  warning?: string
}

// TaskAnswer is what the code says about a task, such as "add a new
// authentication provider" (internal/intel).
export interface TaskImpl extends Ref {
  doc?: string
  template?: boolean
  fields: string[]
  notes: string[]
  tests: number
  made: number
}

export interface TaskPoint {
  interface: Ref
  why: string
  doc?: string
  methods: Ref[]
  impls: TaskImpl[]
  fakes: Ref[]
  holders: Ref[]
  consumers: Ref[]
  made: Ref[]
  routes: Route[]
  upstream: Ref[]
  downstream: Ref[]
  steps: { text: string; at?: Ref }[]
  tests: Ref[]
  breakpoints: Ref[]
}

// FeatureStep is one function on a feature's path, in the order of the code.
export interface FeatureStep extends Ref {
  depth: number
  layer?: string
  errReturns?: number
  branches?: number
  dynamic?: boolean
  seed?: boolean
}

export interface FeatureEntry {
  kind: 'route' | 'main'
  label: string
  ref: Ref
  file: string
  line: number
}

// Feature is a part of the project followed end to end (internal/intel).
export interface Feature {
  entry?: FeatureEntry
  others: FeatureEntry[]
  flow: FeatureStep[]
  data: FeatureStep[]
  errors: FeatureStep[]
  tests: Ref[]
  breakpoints: Ref[]
  note?: string
}

export interface TaskAnswer {
  query: string
  feature?: Feature
  point?: TaskPoint
  related: Ref[]
  note?: string
}

// ArchChange is what the agent's changes did to the project's shape
// (internal/changes/arch.go): the packages they touch, the parts they add,
// and the dependencies between packages that came or went.
export interface PkgChange {
  path: string
  name: string
  layer?: string
  new?: boolean
  added: number
  modified: number
  removed: number
  parts: Ref[]
}

export interface EdgeChange {
  from: string
  to: string
  fromName: string
  toName: string
  fromLayer?: string
  toLayer?: string
  added: boolean
  warn?: string
  file?: string
  line?: number
}

export interface ArchChange {
  packages: PkgChange[]
  edges: EdgeChange[]
}

export interface AgentStep {
  n: number
  title: string
  why?: string
  files: string[]
  packages: string[]
  status: 'todo' | 'doing' | 'done'
  // What the agent said before the step: what it does, the code it uses,
  // what it creates, what it leaves alone and when it is done.
  does?: string
  uses: string[] | null
  creates: string[] | null
  avoid: string[] | null
  doneWhen?: string
  unknown: string[] | null // uses the code does not have
  exists: string[] | null // creates the code already has
  budget: number
}

// FitBox is a declaration where a step's new part sits.
export interface FitBox extends Ref {
  layer?: string
  change?: string
}

export interface TestRun {
  state: 'running' | 'passed' | 'failed' | 'none'
  packages: string[]
  failed: string[]
  output?: string
  tookMs: number
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
  made?: string
  know?: string
  fits?: string // where it sits in the architecture, as the agent tells it
  fit: { above: FitBox[]; parts: FitBox[]; below: FitBox[] } | null
  tests: TestRun | null
  avoided: string[] | null
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

// AgentState is the checkpoints of the folder: the agent's plan, its
// step, and what waits for the developer.
export interface AgentState {
  level: 'off' | 'enforced' | 'cooperative' | 'observe'
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
  problem?: string // why connecting the agent did not work; then the commands to run by hand, a line each
  talk?: { q: string; a?: string; at: string }[] // the developer's questions about what waits, and the agent's answers
}

// A page of Mentor's one view. Home is the agent's work: its plan or
// milestone, the project's shape and its files. Over it open the answer to
// a question, a symbol's card, a flow or a map, and Back goes to the page
// before.
export type Page =
  | { kind: 'home' }
  | { kind: 'search' }
  | { kind: 'card'; key: string; card?: Card }
  | { kind: 'flow'; key: string; flow?: Flow }
  | { kind: 'map'; focus: string; graph?: Graph }

export const store = reactive({
  status: null as Status | null,
  report: null as AgentChanges | null,
  arch: null as ArchChange | null, // what the agent's changes did to the project's shape
  shape: null as Graph | null, // the project's packages, by layer
  reportError: '',
  reportBusy: false,
  agent: null as AgentState | null,
  query: '',
  result: null as SearchResult | null,
  task: null as TaskAnswer | null,
  searching: false,
  pages: [{ kind: 'home' }] as Page[],
  loading: false,
  error: '',
  lens: true,
  place: 'sidebar' as Place,
  windowOpen: false,
})

export function page(): Page {
  return store.pages[store.pages.length - 1] ?? { kind: 'home' }
}

// home goes back to the agent's work, closing what was open over it.
export function home() {
  store.pages = [{ kind: 'home' }]
  store.error = ''
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

// Where Mentor opens, as the setting Open Mentor in says: a view in the
// sidebar, a tab of the bottom panel, or a window of its own over the
// editor.
export type Place = 'sidebar' | 'panel' | 'window'

// popupMode says whether Mentor's window is a popup of the editor's own,
// which the user moves anywhere (BMEditor 0.1.7 and later), rather than one
// drawn inside the editor's window.
export function popupMode(): boolean {
  return store.place === 'window' && !!api.ui.openPopup && !api.ui.inPopup
}

export const popup = 'main'

// show opens Mentor where it lives.
export function show() {
  if (store.place === 'panel') api.ui.showPanel(view)
  else if (popupMode()) void api.ui.openPopup!(popup)
  else if (store.place === 'window') store.windowOpen = true
  else {
    api.ide.sidebarOpen = true
    api.ide.sidebarView = view
  }
}

// Go is what to show: home, a page over it, or the question box.
export type Go = { home?: boolean; page?: Page; ask?: boolean }

// reveal shows Mentor on what g says. A popup has its own state, so it is
// asked through the backend, which every window hears (mentor/go).
export async function reveal(g: Go) {
  if (popupMode()) {
    await api.ui.openPopup!(popup)
    await call('ui.go', g)
    return
  }
  show()
  await apply(g)
}

// apply shows what reveal asked for, in this window.
export async function apply(g: Go) {
  if (g.home) {
    home()
    void loadChanges()
  }
  if (g.page) await go(g.page)
  if (g.ask) setTimeout(() => (document.querySelector('.mentor .q') as HTMLInputElement | null)?.focus(), 50)
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
  if (!q.trim()) {
    store.result = null
    store.task = null
    home()
    return
  }
  store.pages = [{ kind: 'home' }, { kind: 'search' }]
  store.searching = true
  store.error = ''
  store.task = null
  try {
    // A task's answer, when the code has one, comes beside the matches.
    const [res, task] = await Promise.all([call<SearchResult>('search', { q }), call<TaskAnswer>('task', { q }).catch(() => null)])
    store.result = markRaw(res)
    store.task = task?.point || task?.feature ? markRaw(task) : null
  } catch (err) {
    store.result = null
    store.error = message(err)
  } finally {
    store.searching = false
  }
}

// go opens a page: a card, a flow or a map, loading what it shows.
export async function go(p: Page) {
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
  if (store.pages.length > 1) store.pages.pop()
  store.error = ''
}

export async function loadChanges() {
  if (!api.ide.folder) return
  store.reportBusy = true
  try {
    const [rep, arch, shape] = await Promise.all([
      call<AgentChanges>('agent.changes'),
      call<ArchChange>('agent.arch').catch(() => null),
      store.shape ? Promise.resolve(store.shape) : call<Graph>('arch', { focus: '' }).catch(() => null),
    ])
    store.report = markRaw(rep)
    store.arch = arch ? markRaw(arch) : null
    store.shape = shape ? markRaw(shape) : null
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
export async function agent(method: string, params?: unknown, quiet = false) {
  try {
    store.agent = await call<AgentState>('agent.' + method, params)
  } catch (err) {
    if (!quiet) fail('Mentor', err)
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
  store.report = null
  store.arch = null
  store.shape = null
  store.reportError = ''
  store.status = null
  store.result = null
  store.task = null
  store.query = ''
  store.pages = [{ kind: 'home' }]
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
