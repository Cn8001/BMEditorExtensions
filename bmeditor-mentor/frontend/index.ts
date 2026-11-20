import { createApp, watch } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'
import { agent, api, call, clear, cursor, fail, go, loadChanges, refreshStatus, setApi, show, store, view, type AgentState, type Place, type Ref, type Status } from './api'
import { lens, refreshAll } from './lens'
import Sidebar from './Sidebar.vue'
import TopButton from './TopButton.vue'
import Window from './Window.vue'
import './style.css'

// Mentor: understand what a coding agent did to your code, and keep it in
// check. Its view, in the sidebar, the bottom panel or a window of its own,
// answers questions about the code from an index the backend keeps of it.
export default function activate(a: ExtensionAPI) {
  setApi(a)

  let lastState = ''
  api.on('mentor/index', (st: Status) => {
    store.status = st
    // The numbers above the code follow each read of the files.
    if (st.state === 'ready' && lastState !== 'ready') {
      refreshAll()
      // What changed follows the files too, while it is shown.
      if (store.tab === 'changes' || store.report) void loadChanges()
    }
    lastState = st.state
  })

  // When the agent stops at a plan or a checkpoint, say so: it waits until
  // the developer answers. Mentor does not take the sidebar or the panel by
  // itself; it shows the Changes tab when the developer opens it.
  api.on('mentor/agent', (st: AgentState) => {
    const was = store.agent?.waiting
    store.agent = st
    if (st.waiting && st.waiting !== was) {
      store.tab = 'changes'
      void loadChanges()
      api.ui.toast('info', st.waiting === 'plan' ? 'The agent waits for you to approve its plan' : `The agent waits at the checkpoint of step ${st.checkpoint?.step}`, 'Mentor in the top bar shows it.')
    }
  })

  async function explainThis() {
    const tab = api.ui.activeTab()
    show()
    if (!tab) return
    const { line } = cursor()
    try {
      const ref = await call<Ref>('at', { file: tab.path, line })
      await go({ kind: 'card', key: ref.key })
    } catch (err) {
      fail('Mentor', err)
    }
  }

  function askFocus() {
    show()
    store.tab = 'explore'
    // The view mounts when it is shown.
    setTimeout(() => (document.querySelector('.mentor .q') as HTMLInputElement | null)?.focus(), 50)
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
    if (where !== 'window') store.windowOpen = false
  }
  // Until the settings are read Mentor takes no place in the sidebar or the
  // panel, so its button does not show there unless it is chosen.
  place('window')
  createApp(Window).mount(document.body.appendChild(document.createElement('div')))

  api.register({
    editor: lens,
    toolbar: TopButton,
    commands: [
      { id: 'mentor-open', label: 'Mentor: Open', icon: 'mortar-board', run: show },
      { id: 'mentor-explain', label: 'Mentor: Explain This', icon: 'mortar-board', kbd: 'Ctrl+Alt+U', run: explainThis, when: () => !!api.ui.activeTab()?.path.endsWith('.go') },
      { id: 'mentor-ask', label: 'Mentor: Ask About the Code', icon: 'search', run: askFocus, when: () => !!api.ide.folder },
      {
        id: 'mentor-changes',
        label: 'Mentor: What Changed',
        icon: 'diff',
        run: () => {
          show()
          store.tab = 'changes'
          void loadChanges()
        },
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
}
