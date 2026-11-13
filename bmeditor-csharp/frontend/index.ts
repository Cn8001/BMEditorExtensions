import { watch } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'
import { api, call, checkMeta, clear, dirname, fail, loadWorkspace, log as say, openPath, panels, setApi, store, type LogState } from './api'
import { languages } from './languages'
import Inspector from './Inspector.vue'
import SolutionPanel from './SolutionPanel.vue'
import SolutionExplorer from './SolutionExplorer.vue'
import AssembliesExplorer from './AssembliesExplorer.vue'
import UnityPanel from './UnityPanel.vue'

// C# for Unity and UPM packages: a Solution Explorer with the assembly
// graph, an Inspector for assembly definitions and package manifests,
// Unity's compile errors in Problems, .meta tools, and completion of
// Unity's messages. The language server is cmd/cs-ls's, from the manifest.

// The GUID selected, or under the cursor, in the active tab.
function guidAtCursor(): string {
  const tab = api.ui.activeTab()
  const state = tab && api.session.stateOf(tab.id)
  if (!state) return ''
  const sel = state.selection.main
  if (!sel.empty) return state.sliceDoc(sel.from, sel.to).trim().replace(/^GUID:/i, '')
  const line = state.doc.lineAt(sel.head)
  const at = sel.head - line.from
  for (const m of line.text.matchAll(/[0-9a-fA-F]{32}/g)) {
    if (m.index! <= at && at <= m.index! + 32) return m[0]
  }
  return ''
}

// Unity's caches and build output at the project's root, as Rider's Unity
// view leaves them out.
const unityFolders = new Set(['library', 'temp', 'logs', 'obj', 'usersettings', '.vs', '.idea', '.utmp'])

// generated says whether Unity wrote a file or folder for the IDE or for
// itself: nobody edits it by hand.
function generated(e: { name: string; path: string; isDir: boolean }): boolean {
  const w = store.workspace
  if (!w || (w.kind !== 'unity' && w.kind !== 'upm')) return false
  if (!e.isDir) return /\.(csproj|sln|slnx|meta)$/i.test(e.name)
  const parent = e.path.slice(0, e.path.length - e.name.length - 1)
  return w.kind === 'unity' && parent.toLowerCase() === w.root.toLowerCase() && unityFolders.has(e.name.toLowerCase())
}

export default function activate(a: ExtensionAPI) {
  setApi(a)

  let logNote = ''
  const problems = (log: LogState) => {
    store.log = log
    // Say once in Output when the Editor.log is missing or another project's.
    const note = !log.exists ? `No Unity Editor.log at ${log.path}` : !log.matches ? `Unity's Editor.log is for another project: ${log.project}` : ''
    if (note && note !== logNote) say(note)
    logNote = note
    api.ui.setProblems(
      'unity',
      log.messages.map((m) => ({ file: m.file, line: m.line, col: m.col, severity: m.severity, message: `${m.code}: ${m.message}`, source: 'Unity' })),
    )
  }
  api.on('unity/problems', problems)
  api.on('unity/log', (p: { lines: string[] }) => {
    store.logLines.push(...p.lines)
    if (store.logLines.length > 2000) store.logLines.splice(0, store.logLines.length - 2000)
  })

  const show = (id: string) => {
    api.ui.showPanel(id)
  }
  // An editor with a sidebar for extensions (0.1.5, which brought DropMenu
  // too) has the Solution Explorer, its assembly graph and the Inspector
  // there, as Visual Studio does; an older one has them in the bottom panel.
  const sidebar = !!api.components.DropMenu
  // In a C# project the Explorer offers the solution, and where there are
  // assembly definitions the assemblies, beside the files, as Rider does:
  // its Files | Solution | Assemblies switch.
  const views = { solution: 'csharp-solution-explorer', assemblies: 'csharp-assemblies' }
  const csharpProject = () => !!store.workspace && (store.workspace.kind !== 'folder' || store.solutions.length > 0)
  const unityProject = () => store.workspace?.kind === 'unity' || store.workspace?.kind === 'upm'
  const showExplorer = (view = views.solution) => {
    store.ui.solutionTab = view === views.assemblies ? 'graph' : 'solution'
    if (sidebar) {
      api.ide.sidebarOpen = true
      api.ide.sidebarView = 'explorer'
      api.ide.explorerAlt = view
    } else show(panels.solution)
  }

  void api.settings().then((s) => (store.showGenerated = s.values.explorerShowGenerated === 'true'))

  api.register({
    languages,
    explorerHide: (e) => !store.showGenerated && generated(e),
    sidebar: [
      { id: views.solution, label: 'Solution Explorer', icon: 'list-tree', component: SolutionExplorer, replaces: 'explorer', when: csharpProject },
      { id: views.assemblies, label: 'Assemblies', icon: 'type-hierarchy', component: AssembliesExplorer, replaces: 'explorer', when: unityProject },
    ],
    panels: sidebar
      ? [{ id: panels.unity, label: 'Unity', component: UnityPanel, busy: () => store.busy.meta }]
      : [
          { id: panels.solution, label: 'Solution', component: SolutionPanel, busy: () => store.busy.solution || store.busy.assemblies },
          { id: panels.inspector, label: 'Inspector', component: Inspector },
          { id: panels.unity, label: 'Unity', component: UnityPanel, busy: () => store.busy.meta },
        ],
    commands: [
      {
        id: 'csharp-solution',
        label: 'C#: Solution Explorer',
        icon: 'list-tree',
        kbd: 'Ctrl+Alt+L',
        run: () => showExplorer(),
      },
      {
        id: 'csharp-assemblies',
        label: 'C#: Assembly Graph',
        icon: 'type-hierarchy',
        run: () => showExplorer(views.assemblies),
      },
      {
        id: 'csharp-show-solution',
        label: 'C#: Show This Solution',
        icon: 'list-tree',
        when: () => /\.(sln|slnx|csproj)$/i.test(api.ui.activeTab()?.path ?? ''),
        run: () => {
          const path = api.ui.activeTab()!.path
          if (!store.solutions.includes(path)) store.solutions.unshift(path)
          store.solutionPath = path
          showExplorer()
        },
      },
      { id: 'csharp-inspector', label: 'C#: Inspector', icon: 'settings', run: () => (sidebar ? showExplorer() : show(panels.inspector)) },
      { id: 'csharp-output', label: 'C#: Show Output', icon: 'output', when: () => !!api.ui.showOutput, run: () => api.ui.showOutput() },
      {
        id: 'csharp-new-script',
        label: 'C#: New Script…',
        icon: 'new-file',
        when: () => !!store.workspace,
        run: () => {
          const tab = api.ui.activeTab()
          store.ui.newScriptDir = tab ? dirname(tab.path) : store.workspace!.root
          showExplorer()
        },
      },
      {
        id: 'unity-console',
        label: 'Unity: Compile Errors',
        icon: 'output',
        when: () => store.workspace?.kind === 'unity',
        run: () => {
          store.ui.unityTab = 'console'
          show(panels.unity)
        },
      },
      {
        id: 'unity-meta',
        label: 'Unity: Check .meta Files',
        icon: 'file-symlink-file',
        when: () => !!store.workspace,
        run: () => {
          store.ui.unityTab = 'meta'
          show(panels.unity)
          void checkMeta()
        },
      },
      {
        id: 'unity-guid-find',
        label: 'Unity: Find Asset by GUID',
        icon: 'search',
        when: () => !!store.workspace,
        run: async () => {
          const guid = guidAtCursor()
          if (!guid) return api.ui.toast('info', 'Select a GUID, or put the cursor on one, first.')
          try {
            const r = await call<{ path: string }>('guid.find', { guid })
            await openPath(r.path)
          } catch (err) {
            fail('Find asset by GUID', err)
          }
        },
      },
      {
        id: 'unity-guid-copy',
        label: 'Unity: Copy GUID of This File',
        icon: 'copy',
        when: () => !!api.ui.activeTab(),
        run: async () => {
          const tab = api.ui.activeTab()!
          try {
            const r = await call<{ guid: string }>('guid.of', { path: tab.path })
            await navigator.clipboard.writeText(r.guid)
            api.ui.toast('info', `Copied ${r.guid}`)
          } catch (err) {
            fail('Copy GUID', err)
          }
        },
      },
    ],
    reset: () => {
      clear()
      api.ui.setProblems('unity', [])
    },
  })

  // The open folder decides everything: read it again when it changes.
  watch(
    () => api.ide.folder,
    () => {
      clear()
      api.ui.setProblems('unity', [])
      loadWorkspace().catch((err) => fail('C#', err))
    },
    { immediate: true },
  )
}
