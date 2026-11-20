import { watch } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'
import { agent, api, call, clear, connect, cursor, fail, go, loadChanges, refreshStatus, setApi, showSidebar, store, view, type AgentState, type Ref, type Status } from './api'
import { lens, refreshAll } from './lens'
import Sidebar from './Sidebar.vue'
import './style.css'

// Mentor: understand what a coding agent did to your code, and keep it in
// check. Its view in the sidebar answers questions about the code from an
// index the backend keeps of it.
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

  // When the agent stops at a plan or a checkpoint, show it: it waits
  // until the developer answers.
  api.on('mentor/agent', (st: AgentState) => {
    const was = store.agent?.waiting
    store.agent = st
    if (st.waiting && st.waiting !== was) {
      showSidebar()
      store.tab = 'changes'
      void loadChanges()
      api.ui.toast('info', st.waiting === 'plan' ? 'The agent waits for you to approve its plan' : `The agent waits at the checkpoint of step ${st.checkpoint?.step}`)
    }
  })

  async function explainThis() {
    const tab = api.ui.activeTab()
    showSidebar()
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
    showSidebar()
    store.tab = 'explore'
    // The view mounts when the sidebar shows it.
    setTimeout(() => (document.querySelector('.mentor .q') as HTMLInputElement | null)?.focus(), 50)
  }

  api.register({
    sidebar: [{ id: view, label: 'Mentor', icon: 'mortar-board', component: Sidebar }],
    editor: lens,
    commands: [
      { id: 'mentor-explain', label: 'Mentor: Explain This', icon: 'mortar-board', kbd: 'Ctrl+Alt+U', run: explainThis, when: () => !!api.ui.activeTab()?.path.endsWith('.go') },
      { id: 'mentor-ask', label: 'Mentor: Ask About the Code', icon: 'search', run: askFocus, when: () => !!api.ide.folder },
      {
        id: 'mentor-connect',
        label: 'Mentor: Connect the Agent',
        icon: 'hubot',
        run: async () => {
          showSidebar()
          store.tab = 'changes'
          await connect(true)
        },
        when: () => !!api.ide.folder,
      },
      {
        id: 'mentor-changes',
        label: 'Mentor: What Changed',
        icon: 'diff',
        run: () => {
          showSidebar()
          store.tab = 'changes'
          void loadChanges()
        },
        when: () => !!api.ide.folder,
      },
      { id: 'mentor-reindex', label: 'Mentor: Read the Code Again', icon: 'refresh', run: () => call<Status>('reindex').then((s) => (store.status = s)), when: () => !!api.ide.folder },
    ],
    reset: clear,
  })

  const readSettings = () =>
    api.settings().then((s) => {
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
