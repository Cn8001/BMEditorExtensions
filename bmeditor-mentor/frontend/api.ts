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

// A page of the Explore tab: the answer to a question, a symbol's card,
// a flow or a map. Back goes to the one before.
export type Page =
  | { kind: 'search' }
  | { kind: 'card'; key: string; card?: Card }
  | { kind: 'flow'; key: string; flow?: Flow }
  | { kind: 'map'; focus: string; graph?: Graph }

export const store = reactive({
  status: null as Status | null,
  tab: 'explore' as 'explore' | 'agent',
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
  if (store.pages.length > 1) store.pages.pop()
  store.error = ''
}

export function clear() {
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
