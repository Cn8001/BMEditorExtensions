import { watch } from 'vue'
import type { ExtensionAPI, Problem, Tab } from '../bmext/bmeditor'
import { addRunProblem, api, build, hiddenInFiles, isGame, loadIndex, log, play, refresh, setApi, setFilesAssets, stop, store } from './api'
import Console from './Console.vue'
import { lens, refreshLens } from './lens'
import GameExplorer from './GameExplorer.vue'
import PlayBar from './PlayBar.vue'
import { jsonSupport, luaSupport } from './lua'
import './style.css'

// ASCDII Engine games: an Explorer view of the project's assets, Lua with
// the engine's API, and Play, which runs the game in its window or opens
// the project in the engine's editor. The engine reloads a script that is
// saved while the game runs, so there is nothing to restart.

const checks = new Map<string, number>()

// check parses a script as it is typed, with the engine's own Lua.
function check(tab: Tab) {
  window.clearTimeout(checks.get(tab.id))
  checks.set(
    tab.id,
    window.setTimeout(async () => {
      checks.delete(tab.id)
      const text = api.session.textOf(tab.id)
      try {
        const found = await api.call<Problem[]>('check', { file: tab.path, text })
        api.ui.setProblems('ascdii-lua:' + tab.path, found.map((p) => ({ ...p, file: tab.path, source: 'Lua' })))
      } catch {
        // the backend is gone; the next edit asks again
      }
    }, 300),
  )
}

export default function activate(a: ExtensionAPI) {
  setApi(a)

  api.on('ascdii/output', (l: { stream: 'stdout' | 'stderr'; text: string }) => {
    api.ui.output(l.text, l.stream)
    log(l.text)
  })
  api.on('ascdii/problem', (p: Problem) => addRunProblem(p))
  api.on('ascdii/started', (s: { what: string }) => {
    if (s.what === 'build') store.building = true
    else store.running = s.what
    const note = s.what === 'build' ? 'Building the game' : `Started ${s.what}`
    api.ui.output(note)
    log(note)
  })
  api.on('ascdii/exit', (e: { what: string; error: string }) => {
    if (e.what === 'build') {
      store.building = false
      if (e.error) api.ui.toast('error', 'The build failed', 'See Output.')
      else api.ui.toast('info', 'The game is built', "In the project's build folder.")
      return
    }
    store.running = ''
    if (e.error && !store.stopping) {
      api.ui.toast('error', `${e.what} stopped with an error`, 'See Output.')
      api.ui.showOutput()
    }
    store.stopping = false
  })
  api.on('ascdii/changed', () => void loadIndex())

  api.register({
    languages: [
      {
        id: 'lua',
        support: (tab) => [luaSupport, lens(tab)],
        changed: check,
        closed: (tab) => api.ui.setProblems('ascdii-lua:' + tab.path, []),
      },
      { id: 'ascdiiproj', support: () => jsonSupport },
    ],
    panels: [{ id: 'ascdii-console', label: 'Console', component: Console, busy: () => !!store.running }],
    sidebar: [{ id: 'ascdii-game', label: 'Game', icon: 'game', component: GameExplorer, replaces: 'explorer', when: isGame }],
    toolbar: PlayBar,
    explorerHide: hiddenInFiles,
    commands: [
      { id: 'ascdii-play', label: 'ASCDII: Play', icon: 'play', when: () => isGame() && !store.running, run: () => play() },
      { id: 'ascdii-play-window', label: 'ASCDII: Play in a Window', icon: 'play', when: () => isGame() && !store.running, run: () => play('window') },
      { id: 'ascdii-editor', label: 'ASCDII: Open in the ASCDII Editor', icon: 'game', when: () => isGame() && !store.running, run: () => play('editor') },
      { id: 'ascdii-stop', label: 'ASCDII: Stop', icon: 'debug-stop', when: () => !!store.running, run: () => stop() },
      { id: 'ascdii-build', label: 'ASCDII: Build', icon: 'package', when: () => isGame() && !store.building, run: () => build() },
      { id: 'ascdii-refresh', label: 'ASCDII: Refresh Assets', icon: 'refresh', when: isGame, run: () => refresh() },
      { id: 'ascdii-output', label: 'ASCDII: Show Output', icon: 'output', run: () => api.ui.showOutput() },
      {
        id: 'ascdii-settings',
        label: 'ASCDII: Project Settings',
        icon: 'settings',
        when: isGame,
        run: () => void api.session.openFile(store.status!.project!.file),
      },
      {
        id: 'ascdii-files-assets',
        label: 'ASCDII: Show or Hide Assets in Files',
        icon: 'eye',
        when: isGame,
        run: () => void setFilesAssets(store.settings.filesShowAssets !== 'true'),
      },
    ],
    reset: () => void refresh(),
  })

  // Code Vision reads the index: a new one redraws it in the open scripts.
  watch(
    () => store.index,
    () => {
      for (const t of api.ide.tabs) if (t.language === 'lua') api.session.dispatchTo(t.id, { effects: refreshLens.of(null) })
    },
  )

  void refresh()
  // The engine's path may have changed when Settings closes.
  watch(
    () => api.ide.settingsOpen,
    (open) => {
      if (!open) void refresh()
    },
  )
}
