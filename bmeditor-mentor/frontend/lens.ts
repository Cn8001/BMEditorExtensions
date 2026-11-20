import { createApp, type App } from 'vue'
import { StateEffect, StateField, type Extension, type Range } from '@codemirror/state'
import { Decoration, EditorView, WidgetType, keymap, type DecorationSet } from '@codemirror/view'
import type { Tab } from '../bmext/bmeditor'
import { api, call, plural, reveal, store } from './api'
import UsagePeek from './UsagePeek.vue'

// One line above each type and function of a Go file: how many things use
// it, how many tests reach it, how many types implement it. A click on the
// uses opens, under the line, how the project uses it: the places that show
// it best, as they are written (UsagePeek); Esc closes it. The i shows its
// card. The numbers come from the index, as of the last time the files were
// read; edits since move them with the code.

interface Item {
  key: string
  line: number
  start: number
  uses: number
  tests: number
  impls: number
}

const setLens = StateEffect.define<Item[]>()

// The peek open in a file: the symbol and the line its lens is on.
type Peek = { key: string; line: number } | null
const setPeek = StateEffect.define<Peek>()

class Lens extends WidgetType {
  constructor(
    readonly item: Item,
    readonly indent: string,
  ) {
    super()
  }
  eq(o: Lens) {
    return o.indent === this.indent && o.item.key === this.item.key && o.item.uses === this.item.uses && o.item.tests === this.item.tests && o.item.impls === this.item.impls
  }
  toDOM(view: EditorView) {
    const div = document.createElement('div')
    div.className = 'mentor-lens'
    const pad = document.createElement('span')
    pad.className = 'pad'
    pad.textContent = this.indent
    div.appendChild(pad)
    const parts = [plural(this.item.uses, 'use')]
    if (this.item.tests) parts.push(plural(this.item.tests, 'test'))
    if (this.item.impls) parts.push(plural(this.item.impls, 'implementation'))
    const link = document.createElement('span')
    link.className = 'link'
    link.textContent = parts.join(' · ')
    link.title = 'How the project uses it, here (Esc closes)'
    link.onmousedown = (e) => {
      e.preventDefault()
      const open = view.state.field(peekField, false)
      const same = open?.key === this.item.key
      view.dispatch({ effects: setPeek.of(same ? null : { key: this.item.key, line: this.item.start }) })
    }
    div.appendChild(link)
    const card = document.createElement('span')
    card.className = 'link card codicon codicon-info'
    card.title = 'Its card in Mentor'
    card.onmousedown = (e) => {
      e.preventDefault()
      void reveal({ page: { kind: 'card', key: this.item.key } })
    }
    div.appendChild(card)
    return div
  }
  ignoreEvent() {
    return false
  }
}

const field = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    for (const e of tr.effects) {
      if (!e.is(setLens)) continue
      const doc = tr.state.doc
      const ranges: Range<Decoration>[] = []
      for (const it of [...e.value].sort((a, b) => a.start - b.start)) {
        if (it.start < 1 || it.start > doc.lines) continue
        const line = doc.line(it.start)
        const indent = line.text.slice(0, line.text.length - line.text.trimStart().length)
        ranges.push(Decoration.widget({ widget: new Lens(it, indent), block: true, side: -2 }).range(line.from))
      }
      return Decoration.set(ranges, true)
    }
    return tr.docChanged ? deco.map(tr.changes) : deco
  },
  provide: (f) => EditorView.decorations.from(f),
})

class PeekView extends WidgetType {
  constructor(
    readonly key: string,
    readonly indent: string,
  ) {
    super()
  }
  eq(o: PeekView) {
    return o.key === this.key && o.indent === this.indent
  }
  toDOM(view: EditorView) {
    const div = document.createElement('div')
    div.className = 'mentor-peek-host'
    div.style.marginLeft = this.indent.replace(/\t/g, '    ').length + 'ch'
    const app = createApp(UsagePeek, { symbol: this.key, close: () => view.dispatch({ effects: setPeek.of(null) }) })
    app.mount(div)
    ;(div as HTMLElement & { _app?: App })._app = app
    return div
  }
  destroy(dom: HTMLElement) {
    ;(dom as HTMLElement & { _app?: App })._app?.unmount()
  }
  ignoreEvent() {
    return true
  }
}

const peekField = StateField.define<Peek>({
  create: () => null,
  update(p, tr) {
    for (const e of tr.effects) if (e.is(setPeek)) return e.value
    if (p && tr.docChanged) {
      // It follows its line as the code above it changes.
      const pos = tr.changes.mapPos(tr.startState.doc.line(Math.min(p.line, tr.startState.doc.lines)).from)
      return { ...p, line: tr.state.doc.lineAt(pos).number }
    }
    return p
  },
  provide: (f) =>
    EditorView.decorations.compute([f], (state) => {
      const p = state.field(f)
      if (!p || p.line < 1 || p.line > state.doc.lines) return Decoration.none
      const line = state.doc.line(p.line)
      const indent = line.text.slice(0, line.text.length - line.text.trimStart().length)
      return Decoration.set([Decoration.widget({ widget: new PeekView(p.key, indent), block: true, side: -1 }).range(line.from)])
    }),
})

const closePeek = keymap.of([
  {
    key: 'Escape',
    run: (view) => {
      if (!view.state.field(peekField, false)) return false
      view.dispatch({ effects: setPeek.of(null) })
      return true
    },
  },
])

// refresh asks the backend for a tab's numbers and shows them.
export async function refresh(tab: Tab) {
  if (!tab.path.endsWith('.go')) return
  let items: Item[] = []
  if (store.lens) {
    try {
      items = await call<Item[]>('lens', { file: tab.path })
    } catch {
      return
    }
  }
  if (api.session.tab(tab.id)) api.session.dispatchTo(tab.id, { effects: setLens.of(items) })
}

export function refreshAll() {
  for (const t of api.ide.tabs) void refresh(t)
}

export function lens(tab: Tab): Extension {
  if (!tab.path.endsWith('.go')) return []
  // The tab's state is made after this returns.
  setTimeout(() => void refresh(tab), 0)
  return [field, peekField, closePeek]
}
