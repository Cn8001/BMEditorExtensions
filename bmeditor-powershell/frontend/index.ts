import { LanguageSupport, StreamLanguage } from '@codemirror/language'
import { powerShell } from '@codemirror/legacy-modes/mode/powershell'
import { watch } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'
import { batch } from './batch'
import { batchCompletion, powershellCompletion } from './snippets'
import './snippets.css'

// PowerShell and cmd. PowerShell's language server and debugger are
// PowerShell Editor Services (cmd/ps-ls), which the extension installs on
// request; cmd's are its own (cmd/bat-ls, cmd/bat-run). Here: highlighting,
// snippets, and installing Editor Services.

interface Status {
  installed: boolean
  bundle: { dir: string; version?: string; from: string }
  shell?: string
  shellError?: string
}

const powershellLanguage = new LanguageSupport(StreamLanguage.define(powerShell), [powershellCompletion])
const batchLanguage = new LanguageSupport(StreamLanguage.define(batch), [batchCompletion])

export default function activate(api: ExtensionAPI) {
  const say = (text: string) => api.ui.output?.(text)
  let installing = false
  let told = false

  api.on('pses/progress', (p: { line: string }) => say(p.line))

  const install = async () => {
    if (installing) return
    installing = true
    api.ui.showOutput?.()
    say('Installing PowerShell Editor Services…')
    try {
      const st = await api.call<Status>('install')
      api.ui.toast('info', `PowerShell Editor Services ${st.bundle.version ?? ''} installed`, 'Open a PowerShell file again to start it.')
    } catch (err) {
      const f = api.ui.failureOf(err)
      say(f.message)
      api.ui.toastFailure('Cannot install PowerShell Editor Services', f)
    } finally {
      installing = false
    }
  }

  const status = async () => {
    try {
      const st = await api.call<Status>('status')
      if (st.shellError) say(st.shellError)
      if (st.installed) say(`PowerShell Editor Services ${st.bundle.version ?? ''} (${st.bundle.from}): ${st.bundle.dir}\nPowerShell: ${st.shell ?? '-'}`)
      else say('PowerShell Editor Services is not installed: PowerShell: Install Editor Services installs it.')
      return st
    } catch (err) {
      say(api.ui.failureOf(err).message)
      return null
    }
  }

  api.register({
    languages: [
      { id: 'powershell', support: () => powershellLanguage },
      { id: 'batch', support: () => batchLanguage },
    ],
    commands: [
      { id: 'powershell-install', label: 'PowerShell: Install Editor Services', icon: 'cloud-download', run: install },
      {
        id: 'powershell-status',
        label: 'PowerShell: Show Editor Services',
        icon: 'info',
        run: async () => {
          api.ui.showOutput?.()
          await status()
        },
      },
      { id: 'powershell-output', label: 'PowerShell: Show Output', icon: 'output', when: () => !!api.ui.showOutput, run: () => api.ui.showOutput() },
    ],
  })

  // The first PowerShell file opened without Editor Services says how to
  // get it, once.
  watch(
    () => api.ui.activeTab()?.language,
    async (lang) => {
      if (lang !== 'powershell' || told) return
      told = true
      const st = await status()
      if (st && !st.installed) {
        api.ui.toast('info', 'PowerShell has no language server yet', 'Run PowerShell: Install Editor Services (Ctrl+Shift+P) for completion, errors and debugging.')
      }
    },
    { immediate: true },
  )
}
