import { StateEffect, StateField, type Extension, type Range } from '@codemirror/state'
import { Decoration, EditorView, type DecorationSet } from '@codemirror/view'
import type { Tab } from '../bmext/bmeditor'
import { api, store } from './api'
import { fileOf } from './review'

// The agent's code, painted in the code itself: a declaration it added is
// green, one it changed is amber, from its first line to its last, so a
// file opened shows what the agent wrote with nothing to click. One you
// marked reviewed keeps only its edge. The paint goes when the change is
// committed or put back, and while Mentor is off.

interface Span {
  from: number // first line, 1-based
  to: number // last line
  kind: 'added' | 'modified'
  reviewed: boolean
}

const setSpans = StateEffect.define<Span[]>()

const paint = {
  added: Decoration.line({ class: 'mentor-agent mentor-added' }),
  modified: Decoration.line({ class: 'mentor-agent mentor-modified' }),
  addedSeen: Decoration.line({ class: 'mentor-agent mentor-added mentor-seen' }),
  modifiedSeen: Decoration.line({ class: 'mentor-agent mentor-modified mentor-seen' }),
}

const field = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    for (const e of tr.effects) {
      if (!e.is(setSpans)) continue
      const doc = tr.state.doc
      const ranges: Range<Decoration>[] = []
      const painted = new Set<number>()
      for (const s of e.value) {
        const d = s.kind === 'added' ? (s.reviewed ? paint.addedSeen : paint.added) : s.reviewed ? paint.modifiedSeen : paint.modified
        for (let n = Math.max(1, s.from); n <= Math.min(s.to, doc.lines); n++) {
          // A method inside a type, or a field: the outer paint wins.
          if (painted.has(n)) continue
          painted.add(n)
          ranges.push(d.range(doc.line(n).from))
        }
      }
      return Decoration.set(ranges, true)
    }
    return tr.docChanged ? deco.map(tr.changes) : deco
  },
  provide: (f) => EditorView.decorations.from(f),
})

function spansOf(path: string): Span[] {
  if (store.agent?.level === 'off') return []
  const f = fileOf(path)
  if (!f || f.status === 'D') return []
  const out: Span[] = []
  for (const c of f.changes) {
    if (c.change === 'removed' || !c.start) continue
    out.push({ from: c.start, to: c.end || c.start, kind: c.change, reviewed: c.review === 'reviewed' })
  }
  // Wider first, so a type's paint covers its fields and methods.
  return out.sort((a, b) => b.to - b.from - (a.to - a.from))
}

const last = new Map<string, string>()

export function refreshMarks(tab: Tab) {
  if (!api.session.tab(tab.id)) return
  const spans = spansOf(tab.path)
  const sig = JSON.stringify(spans)
  if (last.get(tab.id) === sig) return
  last.set(tab.id, sig)
  api.session.dispatchTo(tab.id, { effects: setSpans.of(spans) })
}

export function refreshAllMarks() {
  for (const t of api.ide.tabs) refreshMarks(t)
}

export function marks(tab: Tab): Extension {
  last.delete(tab.id)
  setTimeout(() => refreshMarks(tab), 0)
  return field
}
