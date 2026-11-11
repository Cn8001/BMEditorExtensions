import { watch } from 'vue'
import type { ExtensionAPI, Tab } from '../bmext/bmeditor'
import { getReady, removeDownloads, toggle } from './actions'
import { ghostText, type Asker } from './ghost'
import { api, refresh, setApi, setting, state, type Status } from './state'
import StatusButton from './Status.vue'

// Inline completion from a code model on this machine: llama.cpp with a
// model it downloads on request, or Ollama. Every tab gets ghost text
// (the editor's `editor` contribution); the backend asks the model.

const maxOther = 3 // other open files sent along
const otherChars = 6000

function disabledLanguages(): Set<string> {
  return new Set(
    setting('disabledLanguages')
      .split(',')
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean),
  )
}

let lastFailure = ''

function askerFor(tab: Tab): Asker {
  return {
    enabled: () => {
      const t = api.session.tab(tab.id) ?? tab
      return state.on && !!state.status?.ready && !state.status.busy && !disabledLanguages().has(t.language)
    },
    delay: () => Math.max(50, Number(setting('debounceMs', '300')) || 300),
    ask: (s, pos) => {
      const t = api.session.tab(tab.id) ?? tab
      const extra = api.ide.tabs
        .filter((o) => o.id !== t.id && o.language === t.language)
        .slice(0, maxOther)
        .map((o) => ({ path: o.path, text: api.session.textOf(o.id).slice(0, otherChars) }))
      const call = api.call<{ text: string }>('complete', {
        path: t.path,
        language: t.language,
        prefix: s.doc.sliceString(Math.max(0, pos - 16000), pos),
        suffix: s.doc.sliceString(pos, Math.min(s.doc.length, pos + 6000)),
        extra,
      })
      state.thinking++
      const result = call
        .then((r) => {
          state.error = ''
          return r?.text ?? ''
        })
        .catch((err) => {
          const f = api.ui.failureOf(err)
          // Before BMEditor 0.1.6 a cancel came back as INTERNAL, by its name only.
          if (f.code === 'CANCELLED' || (err as Error)?.name === 'CancelError') throw err
          if (f.code === 'not-ready' || f.code === 'off') void refresh()
          else {
            state.error = f.message
            // Say each new failure once in Output, not every keystroke.
            if (f.message !== lastFailure) api.ui.output?.(f.message, 'stderr')
            lastFailure = f.message
          }
          throw err
        })
        .finally(() => state.thinking--)
      return { result, cancel: () => call.cancel() }
    },
  }
}

export default function activate(a: ExtensionAPI) {
  setApi(a)

  api.on('autocomplete/progress', (p: { line: string; stream?: 'stderr' }) => api.ui.output?.(p.line, p.stream ?? 'info'))
  api.on('autocomplete/status', (st: Status) => {
    state.status = st
    if (st.ready) api.ui.toast('info', 'Autocomplete is ready', 'Suggestions come as you type: Tab takes one.')
  })

  api.register({
    editor: (tab) => ghostText(askerFor(tab)),
    toolbar: StatusButton,
    commands: [
      { id: 'autocomplete-ready', label: 'Autocomplete: Get Model Ready', icon: 'cloud-download', run: () => getReady() },
      { id: 'autocomplete-toggle', label: 'Autocomplete: Pause or Resume Suggestions', icon: 'sparkle', run: toggle },
      { id: 'autocomplete-cancel', label: 'Autocomplete: Cancel Download', icon: 'debug-stop', when: () => !!state.status?.busy, run: () => api.call('cancel') },
      { id: 'autocomplete-remove', label: 'Autocomplete: Remove Downloaded Models', icon: 'trash', run: removeDownloads },
      { id: 'autocomplete-settings', label: 'Autocomplete: Settings', icon: 'settings-gear', run: () => api.ui.openSettings() },
      { id: 'autocomplete-output', label: 'Autocomplete: Show Output', icon: 'output', when: () => !!api.ui.showOutput, run: () => api.ui.showOutput() },
    ],
    shortcuts: [
      ['Take a suggestion', 'Tab'],
      ['Ask for a suggestion', 'Alt+\\'],
    ],
  })

  void refresh().then(() => {
    const st = state.status
    if (st && !st.ready && !st.busy && st.provider !== 'off' && st.provider !== 'ollama') {
      api.ui.toast('info', 'Autocomplete needs a model', 'Run Autocomplete: Get Model Ready to download Qwen2.5-Coder 0.5B (0.5 GB), or pick another in Settings.')
    }
  })

  // Settings may have changed when their page closes.
  watch(
    () => api.ide.settingsOpen,
    (open) => {
      if (!open) void refresh()
    },
  )
}
