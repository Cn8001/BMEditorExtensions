import { createApp } from 'vue'
import { StateEffect, StateField, type Extension } from '@codemirror/state'
import { showPanel, type Panel } from '@codemirror/view'
import type { Tab } from '../bmext/bmeditor'
import { api, store } from './api'
import { fileOf } from './review'
import ReviewStrip from './ReviewStrip.vue'

// A strip above each file the agent changed: what it changed here, a way
// on to the next, Reviewed, and the checkpoint's answer while the agent
// waits. It shows while the file is in the agent's changes.

const setShown = StateEffect.define<boolean>()

export function strip(tab: Tab): Extension {
  const panel = (): Panel => {
    const dom = document.createElement('div')
    dom.className = 'mentor-strip-host'
    const app = createApp(ReviewStrip, { path: tab.path })
    app.mount(dom)
    return { dom, top: true, destroy: () => app.unmount() }
  }
  const field = StateField.define<boolean>({
    create: () => false,
    update(on, tr) {
      for (const e of tr.effects) if (e.is(setShown)) on = e.value
      return on
    },
    provide: (f) => showPanel.from(f, (on) => (on ? panel : null)),
  })
  // The tab's state is made after this returns, with the strip hidden.
  shown.delete(tab.id)
  setTimeout(() => refreshStrip(tab), 0)
  return field
}

function wanted(tab: Tab): boolean {
  return store.agent?.level !== 'off' && !!fileOf(tab.path)
}

const shown = new Map<string, boolean>()

export function refreshStrip(tab: Tab) {
  const on = wanted(tab)
  if (shown.get(tab.id) === on || !api.session.tab(tab.id)) return
  shown.set(tab.id, on)
  api.session.dispatchTo(tab.id, { effects: setShown.of(on) })
}

export function refreshStrips() {
  for (const t of api.ide.tabs) refreshStrip(t)
}
