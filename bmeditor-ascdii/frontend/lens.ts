import { StateEffect, StateField, type EditorState, type Extension } from '@codemirror/state'
import { Decoration, EditorView, WidgetType, type DecorationSet } from '@codemirror/view'
import type { Tab } from '../bmext/bmeditor'
import { openUse, relative, store, type Use } from './api'

// Code Vision, as Rider has it for Unity: over a script's fields table, the
// objects that use the script, a click away; over each field, the values
// the Inspector set for it in those objects; over each hook, when the
// engine calls it.

export const refreshLens = StateEffect.define<null>()

const when: Record<string, string> = {
  start: 'Engine hook: after the script loads, and after every hot reload',
  update: 'Engine hook: every frame',
  key: 'Engine hook: on each key press, before update',
  on_collision: 'Engine hook: when a contact begins',
  on_trigger: 'Engine hook: when a trigger contact begins',
  on_destroy: 'Engine hook: when Lua destroys the object',
  draw: 'Engine hook: every frame, to draw more shapes',
}

interface Part {
  text: string
  use?: Use
  dim?: boolean
}

class Lens extends WidgetType {
  constructor(
    readonly parts: Part[],
    readonly indent: string, // the line's own leading whitespace
  ) {
    super()
  }
  eq(other: Lens) {
    return other.indent === this.indent && JSON.stringify(other.parts.map((p) => p.text)) === JSON.stringify(this.parts.map((p) => p.text))
  }
  toDOM() {
    const div = document.createElement('div')
    div.className = 'ascdii-lens'
    // The line's whitespace, unseen, in the editor's font, lines the lens
    // up with the code whatever mixes tabs and spaces.
    const pad = document.createElement('span')
    pad.className = 'pad'
    pad.textContent = this.indent
    div.appendChild(pad)
    for (const p of this.parts) {
      const span = document.createElement('span')
      span.textContent = p.text
      span.className = p.dim ? 't dim' : 't'
      const use = p.use
      if (use) {
        span.className = 't link'
        span.title = `${use.object} in ${use.in}`
        span.onmousedown = (e) => {
          e.preventDefault()
          void openUse(use.object, use.in)
        }
      }
      div.appendChild(span)
    }
    return div
  }
  ignoreEvent() {
    return false
  }
}

const scene = (p: string) => p.slice(p.lastIndexOf('/') + 1).replace(/\.(scene|prefab)\.json$/, '')

function show(v: unknown): string {
  if (typeof v === 'number') return String(Math.round(v * 1000) / 1000)
  if (typeof v === 'string') return v.length > 24 ? JSON.stringify(v.slice(0, 22) + '…') : JSON.stringify(v)
  if (Array.isArray(v)) return `(${v.map(show).join(', ')})`
  if (v && typeof v === 'object') return '{…}'
  return String(v)
}

// joined lists parts with separators, the first few, then how many more.
function joined(items: Part[][], most = 4): Part[] {
  const out: Part[] = []
  items.slice(0, most).forEach((p, i) => {
    if (i) out.push({ text: ' · ', dim: true })
    out.push(...p)
  })
  if (items.length > most) out.push({ text: ` · ${items.length - most} more`, dim: true })
  return out
}

function build(state: EditorState, tab: Tab): DecorationSet {
  const rel = relative(tab.path)
  const uses = rel === null ? [] : (store.index?.uses[rel] ?? [])
  const known = rel !== null && !!store.index
  const deco: ReturnType<Decoration['range']>[] = []
  const add = (lineNo: number, parts: Part[]) => {
    const line = state.doc.line(lineNo)
    const indent = line.text.slice(0, line.text.length - line.text.trimStart().length)
    deco.push(Decoration.widget({ widget: new Lens(parts, indent), block: true, side: -1 }).range(line.from))
  }
  let inFields = false
  let depth = 0
  for (let n = 1; n <= state.doc.lines; n++) {
    const text = state.doc.line(n).text
    if (!inFields && /^fields\s*=\s*\{/.test(text)) {
      if (known) {
        add(
          n,
          uses.length
            ? [{ text: `Used by ${uses.length} object${uses.length === 1 ? '' : 's'}: `, dim: true }, ...joined(uses.map((u) => [{ text: `${u.object || '(unnamed)'} (${scene(u.in)})`, use: u }]))]
            : [{ text: 'Not used in any scene or prefab', dim: true }],
        )
      }
      inFields = true
      depth = 0
    }
    if (inFields) {
      const key = depth === 1 && /^\s*([A-Za-z_]\w*)\s*=[^=]/.exec(text)
      if (key) {
        const set = uses.filter((u) => u.values && key[1] in u.values)
        if (set.length) add(n, [{ text: 'Set in the Inspector: ', dim: true }, ...joined(set.map((u) => [{ text: `${u.object}: `, use: u }, { text: show(u.values![key[1]]) }]))])
      }
      for (const c of text.replace(/--.*$/, '')) {
        if (c === '{') depth++
        else if (c === '}') depth--
      }
      if (depth <= 0) inFields = false
      continue
    }
    const hook = /^function\s+([A-Za-z_]\w*)\s*\(/.exec(text)
    if (hook && when[hook[1]]) add(n, [{ text: when[hook[1]], dim: true }])
  }
  return Decoration.set(deco, true)
}

export function lens(tab: Tab): Extension {
  return StateField.define<DecorationSet>({
    create: (state) => build(state, tab),
    update: (deco, tr) => (tr.docChanged || tr.effects.some((e) => e.is(refreshLens)) ? build(tr.state, tab) : deco),
    provide: (f) => EditorView.decorations.from(f),
  })
}
