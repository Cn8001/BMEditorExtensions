import { reactive } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'

// The editor's API, kept from activate for the components, and what the
// backend (cmd/csharp-host) has said about the open folder.
export let api: ExtensionAPI

export function setApi(a: ExtensionAPI) {
  api = a
}

export const panels = { solution: 'csharp-solution', inspector: 'csharp-inspector', unity: 'csharp-unity', assemblies: 'csharp-assemblies' }

export interface Package {
  name: string
  displayName?: string
  version?: string
  dir: string
  manifest: string
}

export interface Workspace {
  root: string
  kind: 'unity' | 'upm' | 'folder'
  unityVersion?: string
  packages: Package[]
  rootNamespace?: string
}

export interface ProjectFile {
  rel: string
  path: string
  kind: 'compile' | 'none' | 'content'
}

export interface Reference {
  name: string
  version?: string
  path?: string
}

export interface Project {
  path: string
  name: string
  sdk?: string
  targetFramework?: string
  assemblyName?: string
  outputType?: string
  files: ProjectFile[]
  projectReferences?: string[]
  packageReferences?: Reference[]
  references?: Reference[]
  analyzers?: string[]
}

export interface SolutionItem {
  kind: 'folder' | 'project'
  name: string
  path?: string
  children?: SolutionItem[]
  project?: Project
  error?: string
  files?: string[]
}

export interface Solution {
  path: string
  name: string
  items: SolutionItem[]
}

export interface Ref {
  raw: string
  name?: string
  status: 'ok' | 'external' | 'unresolved'
}

export interface Assembly {
  name: string
  path: string
  guid?: string
  rootNamespace?: string
  references: Ref[]
  includePlatforms?: string[]
  excludePlatforms?: string[]
  defineConstraints?: string[]
  autoReferenced: boolean
  editor?: boolean
  test?: boolean
  external?: boolean
  error?: string
}

export interface Assemblies {
  assemblies: Assembly[]
  asmrefs: { path: string; reference: string; target?: string }[]
  edges: [number, number][]
  cycles: string[][]
}

export interface MetaReport {
  missing: string[]
  orphans: string[]
  duplicates: { guid: string; metas: string[] }[]
  renames: { meta: string; to: string; reason: 'moved' | 'renamed' }[]
  checked: number
}

export interface MetaFix {
  kind: 'create' | 'delete' | 'move' | 'regen'
  path: string
  to?: string
}

export interface Message {
  file: string
  rel: string
  line: number
  col: number
  severity: 'error' | 'warning'
  code: string
  message: string
}

export interface LogState {
  path: string
  root: string
  exists: boolean
  matches: boolean
  project: string
  messages: Message[]
}

export const store = reactive({
  workspace: null as Workspace | null,
  solutions: [] as string[],
  solutionPath: '',
  solution: null as Solution | null,
  solutionError: '',
  assemblies: null as Assemblies | null,
  meta: null as { roots: string[]; report: MetaReport } | null,
  log: null as LogState | null,
  logLines: [] as string[],
  busy: { solution: false, assemblies: false, meta: false },
  // Unity's .csproj, .sln and .meta in the Explorer: hidden, as in Visual Studio.
  showGenerated: false,
  // Which tab each panel shows, and the folder New Script is open for.
  ui: { solutionTab: 'solution' as 'solution' | 'graph', unityTab: 'console' as 'console' | 'meta', newScriptDir: '' },
})

export function setShowGenerated(show: boolean) {
  store.showGenerated = show
  void api.saveSettings?.({ explorerShowGenerated: String(show) })
}

export function call<T>(method: string, params?: unknown): Promise<T> {
  return api.call<T>(method, params)
}

// log writes to the extension's channel in Output (BMEditor 0.1.5 and later).
export function log(text: string, stream: 'info' | 'stderr' = 'info') {
  api.ui.output?.(text, stream)
}

export function fail(title: string, err: unknown) {
  const f = api.ui.failureOf(err)
  log(`${title}: ${f.message}`, 'stderr')
  api.ui.toastFailure(title, f)
}

export function clear() {
  Object.assign(store, { workspace: null, solutions: [], solutionPath: '', solution: null, solutionError: '', assemblies: null, meta: null, log: null, logLines: [] })
}

export async function loadWorkspace() {
  if (!api.ide.folder) return clear()
  const r = await call<{ workspace: Workspace; solutions: string[] }>('workspace')
  store.workspace = r.workspace
  store.solutions = r.solutions
  const w = r.workspace
  log(
    `${w.root}: ${w.kind === 'unity' ? `Unity ${w.unityVersion} project` : w.kind === 'upm' ? 'UPM package' : 'folder'}` +
      `, ${r.solutions.length ? 'solution ' + basename(r.solutions[0]) : 'no solution'}` +
      (w.packages.length ? `, ${w.packages.length} package(s)` : ''),
  )
  if (!r.solutions.includes(store.solutionPath)) store.solutionPath = r.solutions[0] ?? ''
  if (r.workspace.kind === 'unity') {
    store.log = await call<LogState>('unity.watch')
  }
}

export async function loadSolution() {
  store.busy.solution = true
  try {
    store.solution = await call<Solution>('solution', { path: store.solutionPath })
    store.solutionError = ''
  } catch (err) {
    store.solution = null
    store.solutionError = api.ui.failureOf(err).message
    log(`Solution: ${store.solutionError}`, 'stderr')
  } finally {
    store.busy.solution = false
  }
}

export async function loadAssemblies() {
  store.busy.assemblies = true
  try {
    store.assemblies = await call<Assemblies>('assemblies')
  } catch (err) {
    fail('Assemblies', err)
  } finally {
    store.busy.assemblies = false
  }
}

export async function checkMeta() {
  store.busy.meta = true
  try {
    store.meta = await call<{ roots: string[]; report: MetaReport }>('meta.check')
  } catch (err) {
    fail('Check .meta files', err)
  } finally {
    store.busy.meta = false
  }
}

export async function openPath(path: string, line?: number, col?: number) {
  await api.session.openFile(path, line ? { line, col: col ?? 1 } : undefined)
}

// The path separator of the open folder: Windows' or the others'.
export function sep(): string {
  return (store.workspace?.root ?? api.ide.folder).includes('\\') ? '\\' : '/'
}

export function join(dir: string, name: string): string {
  return dir.endsWith(sep()) ? dir + name : dir + sep() + name
}

export function dirname(path: string): string {
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))
  return i > 0 ? path.slice(0, i) : path
}

export function basename(path: string): string {
  return path.slice(Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1)
}

export function rel(path: string): string {
  return api.paths.relativeTo(store.workspace?.root ?? api.ide.folder, path)
}
