import { completionStatus } from '@codemirror/autocomplete'
import { EditorSelection, Prec, StateEffect, StateField, type EditorState, type Extension, type Transaction } from '@codemirror/state'
import { Decoration, EditorView, ViewPlugin, WidgetType, keymap, type DecorationSet, type ViewUpdate } from '@codemirror/view'

// Ghost text: the model's suggestion drawn faint at the cursor, taken with
// Tab (or a word of it with Ctrl+Right), put away with Esc. It is asked for
// when typing pauses, or with Alt+\.

export interface Ghost {
  pos: number
  text: string
}

// What the extension gives the plugin: when to ask, and how.
export interface Asker {
  enabled: () => boolean
  delay: () => number
  ask: (state: EditorState, pos: number) => { result: Promise<string>; cancel: () => void }
}

const setGhost = StateEffect.define<Ghost | null>()

// After a change, a suggestion lives on only if what was typed is its start.
export function follow(ghost: Ghost | null, tr: Transaction): Ghost | null {
  if (!ghost) return null
  if (tr.docChanged) {
    let typed: string | null = null
    let at = -1
    let ok = true
    tr.changes.iterChanges((fromA, toA, _fromB, _toB, inserted) => {
      if (typed !== null || fromA !== toA || fromA !== ghost.pos) ok = false
      typed = inserted.toString()
      at = fromA
    })
    if (!ok || typed === null || at !== ghost.pos) return null
    const t = typed as string
    if (!t || !ghost.text.startsWith(t) || t.length >= ghost.text.length) return null
    return { pos: ghost.pos + t.length, text: ghost.text.slice(t.length) }
  }
  if (tr.selection) {
    const sel = tr.state.selection.main
    if (!sel.empty || sel.head !== ghost.pos) return null
  }
  return ghost
}

const ghostField = StateField.define<Ghost | null>({
  create: () => null,
  update(value, tr) {
    for (const e of tr.effects) if (e.is(setGhost)) return e.value
    return follow(value, tr)
  },
  provide: (f) =>
    EditorView.decorations.from(f, (g): DecorationSet => (g ? Decoration.set([Decoration.widget({ widget: new GhostWidget(g.text), side: 1 }).range(g.pos)]) : Decoration.none)),
})

class GhostWidget extends WidgetType {
  constructor(readonly text: string) {
    super()
  }
  eq(other: GhostWidget) {
    return other.text === this.text
  }
  toDOM() {
    const span = document.createElement('span')
    span.className = 'cm-ghost-text'
    span.textContent = this.text
    return span
  }
  ignoreEvent() {
    return true
  }
}

// The next word of a suggestion: what Ctrl+Right takes.
export function nextWord(text: string): string {
  const m = /^(\s*[\w$]+|\s*[^\w\s]+|\s+)/.exec(text)
  return m ? m[0] : text
}

function accept(view: EditorView, part?: (t: string) => string): boolean {
  const g = view.state.field(ghostField, false)
  if (!g) return false
  const text = part ? part(g.text) : g.text
  view.dispatch({
    changes: { from: g.pos, insert: text },
    selection: EditorSelection.cursor(g.pos + text.length),
    userEvent: 'input.complete',
    effects: setGhost.of(text === g.text ? null : { pos: g.pos + text.length, text: g.text.slice(text.length) }),
    scrollIntoView: true,
  })
  return true
}

function dismiss(view: EditorView): boolean {
  if (!view.state.field(ghostField, false)) return false
  view.dispatch({ effects: setGhost.of(null) })
  return true
}

// Where a suggestion makes sense: at the end of a line, or before only
// closing brackets, quotes and punctuation.
export function goodSpot(state: EditorState): boolean {
  const sel = state.selection.main
  if (!sel.empty) return false
  const line = state.doc.lineAt(sel.head)
  const after = line.text.slice(sel.head - line.from)
  return /^[\s)\]}>"'`;,:]*$/.test(after)
}

export function ghostText(asker: Asker): Extension {
  const plugin = ViewPlugin.fromClass(
    class {
      timer = 0
      pending: { cancel: () => void } | null = null

      constructor(readonly view: EditorView) {}

      update(u: ViewUpdate) {
        const typed = u.transactions.some((tr) => tr.isUserEvent('input.type') || tr.isUserEvent('delete'))
        if (u.docChanged || u.selectionSet) this.stop()
        // A suggestion that typing follows needs no new one.
        if (typed && !u.state.field(ghostField, false)) this.schedule(asker.delay())
      }

      stop() {
        window.clearTimeout(this.timer)
        this.pending?.cancel()
        this.pending = null
      }

      schedule(ms: number) {
        window.clearTimeout(this.timer)
        this.timer = window.setTimeout(() => this.request(false), ms)
      }

      request(explicit: boolean) {
        const view = this.view
        if (!asker.enabled() || !view.hasFocus) return
        if (!explicit && (!goodSpot(view.state) || completionStatus(view.state) !== null)) return
        if (!view.state.selection.main.empty) return
        this.pending?.cancel()
        const state = view.state
        const pos = state.selection.main.head
        const call = asker.ask(state, pos)
        this.pending = call
        call.result.then(
          (text) => {
            if (this.pending !== call) return
            this.pending = null
            if (view.state !== state || !text) return // typed on meanwhile
            view.dispatch({ effects: setGhost.of({ pos, text }) })
          },
          () => {
            if (this.pending === call) this.pending = null
          },
        )
      }

      destroy() {
        this.stop()
      }
    },
  )

  const askNow = (view: EditorView) => {
    view.plugin(plugin)?.request(true)
    return true
  }

  return [
    ghostField,
    plugin,
    Prec.highest(
      keymap.of([
        { key: 'Tab', run: (v) => accept(v) },
        { key: 'Ctrl-ArrowRight', mac: 'Alt-ArrowRight', run: (v) => accept(v, nextWord) },
        { key: 'Escape', run: dismiss },
        { key: 'Alt-\\', run: askNow },
      ]),
    ),
    EditorView.baseTheme({
      '.cm-ghost-text': { opacity: '0.45', fontStyle: 'italic', whiteSpace: 'pre', pointerEvents: 'none' },
    }),
  ]
}
