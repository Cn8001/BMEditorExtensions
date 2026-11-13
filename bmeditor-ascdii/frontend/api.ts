import { markRaw, reactive } from 'vue'
import type { ExtensionAPI, Problem } from '../bmext/bmeditor'

// The editor's API, kept from activate, and what the backend
// (cmd/ascdii-host) has said about the open game.
export let api: ExtensionAPI

export function setApi(a: ExtensionAPI) {
  api = a
}

export interface Manifest {
  name: string
  version: string
  engine: string
  start: string
  display: { mode?: string }
}

export interface Project {
  dir: string
  file: string
  manifest: Manifest
}

export interface Status {
  project: Project | null
  engine: string
  version?: string
  missing?: string
  running: string
}

export type Kind = 'scene' | 'prefab' | 'script' | 'model' | 'material' | 'animator' | 'sprite' | 'texture' | 'audio'

export interface Asset {
  path: string
  kind: Kind
  guid?: string
  package?: string
}

export interface Use {
  object: string
  in: string
  values?: Record<string, unknown> // what the Inspector gave the script's fields there
}

export interface Index {
  assets: Asset[]
  uses: Record<string, Use[]>
  errors?: Record<string, string>
  assetOnly: string[] // folders of nothing but assets, which Files leaves out
}

export type Target = 'window' | 'editor'

export type Level = 'info' | 'warn' | 'error' | 'out'

// A line of the Console: what the engine logged, read as its level, source
// and message, and the script line it names, if any.
export interface LogLine {
  id: number
  time: string
  level: Level
  source: string
  text: string
  file?: string // a script, slash-separated from the project
  line?: number
}

export const store = reactive({
  status: null as Status | null,
  index: null as Index | null,
  running: '', // what runs: the game's name, ASCDII Editor, or build
  building: false,
  stopping: false, // Stop was asked: the game's end is no error
  settings: {} as Record<string, string>,
  console: [] as LogLine[],
})

let logSeq = 0
const maxLog = 5000

// log reads a line the engine wrote: "<level> <source>: <message>", where
// a failed script's message starts with its file and line.
export function log(text: string) {
  const m = /^(info|debug|warn|warning|error)\s+(\S+?):\s+(.*)$/.exec(text)
  const level: Level = m ? (m[1] === 'error' ? 'error' : m[1].startsWith('warn') ? 'warn' : 'info') : 'out'
  const at = /([\w./-]+\.lua):(\d+)/.exec(m ? m[3] : text)
  const now = new Date()
  store.console.push({
    id: ++logSeq,
    time: now.toTimeString().slice(0, 8),
    level,
    source: m ? m[2] : '',
    text: m ? m[3] : text,
    file: at?.[1],
    line: at ? Number(at[2]) : undefined,
  })
  if (store.console.length > maxLog) store.console.splice(0, store.console.length - maxLog)
}

export function clearConsole() {
  store.console = []
}

export function isGame(): boolean {
  return !!store.status?.project
}

export function target(): Target {
  return store.settings.playTarget === 'editor' ? 'editor' : 'window'
}

const slash = (p: string) => p.replace(/\\/g, '/')

// relative is a path from the project's folder, slash-separated, or null
// for one outside it.
export function relative(path: string): string | null {
  const root = slash(store.status?.project?.dir ?? '').replace(/\/$/, '')
  const p = slash(path)
  if (!root || !p.toLowerCase().startsWith(root.toLowerCase() + '/')) return p.toLowerCase() === root.toLowerCase() ? '' : null
  return p.slice(root.length + 1)
}

const notCode = /\.(png|jpe?g|gif|obj|mtl|fbx|wav|mp3|ogg|xp|meta)$/i

// hiddenInFiles keeps the Files view to what is edited as text, as Rider's
// is: images, models, sounds and .meta files stay in the Game view, and so
// do folders holding only those, and the build output.
export function hiddenInFiles(e: { name: string; path: string; isDir: boolean }): boolean {
  if (!isGame() || store.settings.filesShowAssets === 'true') return false
  const rel = relative(e.path)
  if (rel === null) return false
  if (!e.isDir) return notCode.test(e.name)
  return rel === 'build' || !!store.index?.assetOnly?.includes(rel)
}

export function manifestTab() {
  const file = store.status?.project?.file
  return file ? api.ide.tabs.find((t) => slash(t.path).toLowerCase() === slash(file).toLowerCase()) : undefined
}

// openUse opens a scene or prefab at the object's line.
export async function openUse(object: string, file: string) {
  await api.session.openFile(absolute(file))
  const tab = api.ui.activeTab()
  const state = tab && api.session.stateOf(tab.id)
  if (!state) return
  const at = state.doc.toString().indexOf(`"name": ${JSON.stringify(object)}`)
  if (at >= 0) {
    const line = state.doc.lineAt(at)
    api.session.goto(line.number, at - line.from + 1, 0)
  }
}

export function absolute(rel: string): string {
  const dir = store.status?.project?.dir ?? ''
  const sep = dir.includes('\\') ? '\\' : '/'
  return dir + sep + rel.split('/').join(sep)
}

export async function refresh() {
  try {
    store.settings = (await api.settings()).values
  } catch {
    // the last ones
  }
  try {
    store.status = await api.call<Status>('status')
    store.running = store.status.running
  } catch (err) {
    store.status = null
    api.ui.output(api.ui.failureOf(err).message, 'stderr')
  }
  await loadIndex()
}

export async function loadIndex() {
  if (!isGame()) {
    store.index = null
    return
  }
  try {
    store.index = markRaw(await api.call<Index>('index'))
  } catch (err) {
    api.ui.output(api.ui.failureOf(err).message, 'stderr')
  }
}

// failed says why a call to the engine did not start, and where to fix it.
function failed(title: string, err: unknown) {
  const f = api.ui.failureOf(err)
  if (f.code === 'no-engine') {
    api.ui.toast('error', 'The ascdii program was not found', 'Set it in Settings: the program, or the folder it is in. The engine is at github.com/Cn8001/ASCDII-Engine.')
    api.ui.openSettings()
    return
  }
  api.ui.toastFailure(title, f)
}

// The problems the running game logged, by file, so a new run clears them.
let runProblems: Record<string, Problem[]> = {}

export function clearRunProblems() {
  for (const file of Object.keys(runProblems)) api.ui.setProblems('ascdii-run:' + file, [])
  runProblems = {}
}

export function addRunProblem(p: Problem) {
  const list = (runProblems[p.file] ??= [])
  if (list.some((x) => x.line === p.line && x.message === p.message)) return
  list.push({ ...p, source: 'ASCDII' })
  api.ui.setProblems('ascdii-run:' + p.file, list)
}

// clearOnPlay is the Console's Clear on Play, kept where the Console keeps it.
function clearOnPlay(): boolean {
  try {
    return JSON.parse(localStorage.getItem('bmeditor-ascdii.console') ?? '{}').clearOnPlay !== false
  } catch {
    return true
  }
}

export async function play(t: Target = target()) {
  clearRunProblems()
  if (clearOnPlay()) clearConsole()
  api.ui.showPanel('ascdii-console')
  try {
    await api.call('play', { target: t })
  } catch (err) {
    failed(t === 'editor' ? 'Cannot open the ASCDII editor' : 'Cannot play the game', err)
  }
}

export async function stop() {
  store.stopping = true
  await api.call('stop').catch(() => {})
}

export async function build() {
  try {
    api.ui.showOutput()
    await api.call('build')
    store.building = true
  } catch (err) {
    failed('Cannot build the game', err)
  }
}

export async function setFilesAssets(show: boolean) {
  store.settings = { ...store.settings, filesShowAssets: String(show) }
  await api.saveSettings?.({ filesShowAssets: String(show) })
}

export async function setTarget(t: Target) {
  store.settings = { ...store.settings, playTarget: t }
  await api.saveSettings?.({ playTarget: t })
}
