import { StateEffect, StateField, type Extension, type Range } from '@codemirror/state'
import { Decoration, EditorView, WidgetType, type DecorationSet } from '@codemirror/view'
import type { Tab } from '../bmext/bmeditor'
import { api, call, go, plural, show, store } from './api'

// One line above each type and function of a Go file: how many things use
// it, how many tests reach it, how many types implement it. A click shows
// its card. The numbers come from the index, as of the last time the files
// were read; edits since move them with the code.

interface Item {
  key: string
  line: number
  start: number
  uses: number
  tests: number
  impls: number
}

const setLens = StateEffect.define<Item[]>()

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
  toDOM() {
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
    link.title = 'Mentor: what it is and who uses it'
    link.onmousedown = (e) => {
      e.preventDefault()
      show()
      void go({ kind: 'card', key: this.item.key })
    }
    div.appendChild(link)
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
        ranges.push(Decoration.widget({ widget: new Lens(it, indent), block: true, side: -1 }).range(line.from))
      }
      return Decoration.set(ranges, true)
    }
    return tr.docChanged ? deco.map(tr.changes) : deco
  },
  provide: (f) => EditorView.decorations.from(f),
})

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
  return field
}
