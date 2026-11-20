import { createApp, watch } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'
import { agent, api, apply, call, clear, cursor, fail, home, loadChanges, popup, refreshStatus, reveal, setApi, show, store, view, type AgentState, type Go, type Place, type Ref, type Status } from './api'
import { lens, refreshAll } from './lens'
import { keydown, reviewAndNext, step } from './review'
import { refreshStrips, strip } from './strip'
import { marks, refreshAllMarks } from './marks'
import Sidebar from './Sidebar.vue'
import TopButton from './TopButton.vue'
import Window from './Window.vue'
import './style.css'

// Mentor: understand what a coding agent did to your code, and keep it in
// check. Its view, in the sidebar, the bottom panel or a window of its own,
// answers questions about the code from an index the backend keeps of it.
export default function activate(a: ExtensionAPI) {
  setApi(a)
  // The view can be a popup, a window of its own (BMEditor 0.1.7 and later).
  api.register({ popups: [{ id: popup, title: 'Mentor', component: Sidebar, width: 440, height: 720 }] })

  let lastState = ''
  api.on('mentor/index', (st: Status) => {
    store.status = st
    // The numbers above the code follow each read of the files.
    if (st.state === 'ready' && lastState !== 'ready') {
      refreshAll()
      // What the agent changed follows the files too: the view and the
      // strips above its files show it.
      if (api.ide.folder) void loadChanges()
    }
    lastState = st.state
  })

  // When the agent stops at its plan or a milestone, say so: it waits until
  // the developer answers. Mentor does not take the sidebar or the panel by
  // itself; its view goes home, to the plan or the milestone, for when the
  // developer opens it.
  api.on('mentor/agent', (st: AgentState) => {
    const was = store.agent?.waiting
    store.agent = st
    if (st.waiting && st.waiting !== was) {
      home()
      void loadChanges()
      // The main window says so; a popup only shows it.
      if (!api.ui.inPopup) api.ui.toast('info', st.waiting === 'plan' ? 'The agent waits for you to approve its plan' : `The agent reached a milestone: step ${st.checkpoint?.step}`, 'Mentor in the top bar shows it.')
    }
  })

  // What the main window asks Mentor to show reaches a popup here.
  const goHere = (g: Go | null) => {
    if (g) void apply(g)
  }

  // The agent's state comes as a notification with each change. It is also
  // asked for every few seconds, and when the window comes back, so a
  // notification that went astray, as a window of its own may miss, cannot
  // leave the view behind; when the agent did something since, what it
  // changed is read again too.
  let seen = ''
  const sync = async () => {
    if (document.hidden || store.agent?.level === 'off') return
    const before = store.agent?.waiting
    await agent('state', undefined, true)
    const st = store.agent
    if (!st) return
    const now = `${st.events?.length ?? 0}|${st.waiting ?? ''}|${st.checkpoint?.id ?? ''}|${st.files.length}`
    if (now !== seen) {
      if (seen) void loadChanges()
      seen = now
    }
    if (st.waiting && st.waiting !== before) home()
  }
  setInterval(sync, 3000)
  window.addEventListener('focus', sync)

  const follow = () =>
    watch(
      () => api.ide.folder,
      () => {
        clear()
        lastState = ''
        void refreshStatus()
        if (api.ide.folder) void agent('state')
      },
      { immediate: true },
    )

  // A popup is only the view: the main window keeps the code's side, the
  // lens, the strips, the keys and the commands.
  if (api.ui.inPopup) {
    api.on('mentor/go', goHere)
    follow()
    void call<Go | null>('ui.pending').then(goHere, () => {})
    return
  }

  async function explainThis() {
    const tab = api.ui.activeTab()
    if (!tab) return show()
    const { line } = cursor()
    try {
      const ref = await call<Ref>('at', { file: tab.path, line })
      await reveal({ page: { kind: 'card', key: ref.key } })
    } catch (err) {
      fail('Mentor', err)
    }
  }

  function askFocus() {
    void reveal({ ask: true })
  }

  // place puts Mentor's view where the setting says, and takes it from
  // the other places.
  function place(where: Place) {
    store.place = where
    api.register({
      sidebar: where === 'sidebar' ? [{ id: view, label: 'Mentor', icon: 'mortar-board', component: Sidebar }] : [],
      panels: where === 'panel' ? [{ id: view, label: 'Mentor', component: Sidebar, busy: () => store.reportBusy || store.status?.state === 'indexing' }] : [],
    })
    const ide = api.ide as typeof api.ide & { panelOrder?: string[]; panel?: { tab: string } }
    // The editor lists extensions' panels once, at start.
    if (where === 'panel' && ide.panelOrder && !ide.panelOrder.includes(view)) ide.panelOrder.push(view)
    if (where !== 'panel' && ide.panel?.tab === view) ide.panel.tab = 'problems'
    if (where !== 'sidebar' && ide.sidebarView === view) ide.sidebarView = 'explorer'
    if (where !== 'window') {
      store.windowOpen = false
      if (api.ui.popupOpen?.(popup)) void api.ui.closePopup?.(popup)
    }
  }
  // Until the settings are read Mentor takes no place in the sidebar or the
  // panel, so its button does not show there unless it is chosen.
  place('window')
  createApp(Window).mount(document.body.appendChild(document.createElement('div')))

  api.register({
    editor: (tab) => [lens(tab), strip(tab), marks(tab)],
    toolbar: TopButton,
    keydown: (e) => keydown(e, explainThis),
    shortcuts: [
      ['Next change by the agent', 'F7'],
      ['Mark the file reviewed, go on', 'Alt+R'],
    ],
    commands: [
      { id: 'mentor-next', label: "Mentor: Next Change by the Agent", icon: 'arrow-down', kbd: 'F7', run: () => step(1), when: () => !!api.ide.folder },
      { id: 'mentor-prev', label: "Mentor: Change Before by the Agent", icon: 'arrow-up', kbd: 'Shift+F7', run: () => step(-1), when: () => !!api.ide.folder },
      { id: 'mentor-reviewed', label: 'Mentor: Mark the File Reviewed and Go On', icon: 'check', kbd: 'Alt+R', run: reviewAndNext, when: () => !!api.ide.folder },
      { id: 'mentor-open', label: 'Mentor: Open', icon: 'mortar-board', run: show },
      { id: 'mentor-explain', label: 'Mentor: Explain This', icon: 'mortar-board', kbd: 'Ctrl+Alt+U', run: explainThis, when: () => !!api.ui.activeTab()?.path.endsWith('.go') },
      { id: 'mentor-ask', label: 'Mentor: Ask About the Code', icon: 'search', run: askFocus, when: () => !!api.ide.folder },
      {
        id: 'mentor-changes',
        label: 'Mentor: What Changed',
        icon: 'diff',
        run: () => reveal({ home: true }),
        when: () => !!api.ide.folder,
      },
      { id: 'mentor-reindex', label: 'Mentor: Read the Code Again', icon: 'refresh', run: () => call<Status>('reindex').then((s) => (store.status = s)), when: () => !!api.ide.folder },
    ],
    reset: clear,
  })

  // At start Mentor waits to be opened; a change of place opens its window.
  let started = false
  const readSettings = () =>
    api.settings().then((s) => {
      const where = (['sidebar', 'panel', 'window'] as const).find((p) => p === s.values.openIn) ?? 'window'
      if (where !== store.place) {
        const open = started && where === 'window'
        place(where)
        if (open) show()
      }
      started = true
      const on = s.values.lens !== 'false'
      if (on !== store.lens) {
        store.lens = on
        refreshAll()
      }
    })

  follow()
  watch(
    () => api.ide.settingsOpen,
    (open) => {
      if (!open) {
        void readSettings()
        void refreshStatus()
      }
    },
  )
  void readSettings()

  // The strips above the agent's files follow its changes, and go when
  // Mentor is off.
  const repaint = () => {
    refreshStrips()
    refreshAllMarks()
  }
  watch(() => [store.report, store.agent?.level], repaint)
  watch(() => api.ide.tabs.length, repaint)
}
