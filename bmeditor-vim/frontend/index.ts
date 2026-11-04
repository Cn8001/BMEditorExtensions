import { watch } from 'vue'
import { Compartment, Prec, StateEffect, StateField, type Extension } from '@codemirror/state'
import { Vim, vim } from '@replit/codemirror-vim'
import type { ExtensionAPI } from '../bmext/bmeditor'
import { parse, type Action } from './vimrc'

// Vim keys in every tab. BMEditor gives an extension no hook for all its
// editors, so each tab's state gets Vim added to it once, through
// appendConfig, which stays with the state as the tab is switched to and
// from. The compartment lets the command turn it off and on again.

const slot = new Compartment()
const added = StateField.define<boolean>({ create: () => true, update: (v) => v })
const storageKey = 'bmext-vim:on'

let on = true
try {
  on = localStorage.getItem(storageKey) !== 'off'
} catch {
  // No storage: start on.
}

// Prec.highest, so a key Vim knows is Vim's before the editor's keymaps.
const keys = (): Extension => (on ? Prec.highest(vim({ status: true })) : [])

// The editor's own shortcut, pressed for the user: :w saves as Ctrl+S does.
function press(key: string) {
  window.dispatchEvent(new KeyboardEvent('keydown', { key, ctrlKey: true, bubbles: true, cancelable: true }))
}

function commandLine() {
  Vim.defineEx('write', 'w', () => press('s'))
  Vim.defineEx('quit', 'q', () => press('w'))
  Vim.defineEx('wq', 'wq', () => {
    press('s')
    setTimeout(() => press('w'), 200)
  })
  Vim.defineEx('xit', 'x', () => {
    press('s')
    setTimeout(() => press('w'), 200)
  })
}

// The mappings the vimrc made. Only these can be unmapped: Vim's own keys
// are no mappings in Vim either, and Vim.mapclear, which a reload uses to
// start over, assumes none of them is gone.
const mapped = new Set<string>()

// applyVimrc puts a vimrc's actions in place of the last one's and answers
// how many took. Options this Vim mode does not have are skipped.
function applyVimrc(actions: Action[]): number {
  Vim.mapclear()
  mapped.clear()
  let n = 0
  for (const a of actions) {
    const id = a.kind === 'set' ? '' : `${a.mode ?? ''}\0${a.lhs}`
    try {
      if (a.kind === 'map') {
        if (a.noremap) Vim.noremap(a.lhs, a.rhs, a.mode as string)
        else Vim.map(a.lhs, a.rhs, a.mode as string)
        mapped.add(id)
      } else if (a.kind === 'unmap') {
        if (!mapped.delete(id)) continue
        Vim.unmap(a.lhs, a.mode as string)
      } else {
        const now: unknown = Vim.getOption(a.name)
        if (now instanceof Error) continue
        const flag = typeof now === 'boolean'
        if (flag !== (typeof a.value === 'boolean' || a.value === 'toggle')) continue
        Vim.setOption(a.name, a.value === 'toggle' ? !now : a.value)
      }
      n++
    } catch {
      // One Vim refuses, as an ex mapping with a mode: the rest still apply.
    }
  }
  return n
}

type VimrcFile = { path: string; text: string }

export default function activate(api: ExtensionAPI) {
  let loads = 0
  // load reads the vimrc again; tell says the outcome, for the command.
  const load = async (tell: boolean) => {
    const mine = ++loads
    let f: VimrcFile
    try {
      f = await api.call<VimrcFile>('vimrc')
    } catch (err) {
      api.ui.toastFailure('Vim: vimrc not loaded', api.ui.failureOf(err))
      return
    }
    if (mine !== loads) return // a later load answers for the newer settings
    const n = applyVimrc(f.path ? parse(f.text) : [])
    if (!tell) return
    if (f.path) api.ui.toast('info', `vimrc loaded, ${n} ${n === 1 ? 'line' : 'lines'} applied`, f.path)
    else api.ui.toast('info', 'No vimrc found', 'Set one in Settings, or put it at ~/.vimrc')
  }
  void load(false)
  // The setting may have changed; the window hears of it only this way.
  watch(
    () => api.ide.settingsOpen,
    (open, was) => {
      if (was && !open) void load(false)
    },
  )

  const fit = () => {
    for (const tab of api.ide.tabs) {
      const state = api.session.stateOf(tab.id)
      if (state && !state.field(added, false)) {
        api.session.dispatchTo(tab.id, { effects: StateEffect.appendConfig.of([added, slot.of(keys())]) })
      }
    }
  }
  watch(() => [api.ide.tabs.length, api.ide.activeId], fit, { immediate: true })
  commandLine()

  api.register({
    commands: [
      {
        id: 'vim-toggle',
        label: 'Vim: Toggle Vim Mode',
        icon: 'keyboard',
        run: () => {
          on = !on
          try {
            localStorage.setItem(storageKey, on ? 'on' : 'off')
          } catch {
            // The choice holds until the window reloads.
          }
          for (const tab of api.ide.tabs) api.session.dispatchTo(tab.id, { effects: slot.reconfigure(keys()) })
          api.ui.toast('info', on ? 'Vim mode on' : 'Vim mode off')
        },
      },
      {
        id: 'vim-reload-vimrc',
        label: 'Vim: Reload vimrc',
        icon: 'refresh',
        run: () => load(true),
      },
    ],
    // Another project may name another vimrc.
    reset: () => void load(false),
    shortcuts: [['Vim: normal mode', 'Esc'], ['Vim: command line', ':']],
  })
}
