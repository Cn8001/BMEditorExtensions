import { watch } from 'vue'
import { Compartment, Prec, StateEffect, StateField, type Extension } from '@codemirror/state'
import { Vim, vim } from '@replit/codemirror-vim'
import type { ExtensionAPI } from '../bmext/bmeditor'

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

export default function activate(api: ExtensionAPI) {
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
    ],
    shortcuts: [['Vim: normal mode', 'Esc'], ['Vim: command line', ':']],
  })
}
