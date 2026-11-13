import { snippetCompletion, type Completion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete'
import { LanguageSupport, StreamLanguage, syntaxTree } from '@codemirror/language'
import { json } from '@codemirror/legacy-modes/mode/javascript'
import { EditorState, type Extension } from '@codemirror/state'
import { hoverTooltip } from '@codemirror/view'
import { builtins, componentMembers, drawerMembers, fieldTypes, globals, hooks, keywords, objectMembers, tables, vec3Members, type Entry } from './luaApi'

// Completion and hover for the engine's Lua API: globals, hooks, the fields
// table, and the members after a dot or a colon. Lua has no types, so after
// a colon every handle's methods are offered, except on the known tables
// and on d, the drawer of draw(self, d).

function quiet(state: EditorState, pos: number): boolean {
  const name = syntaxTree(state).resolveInner(pos, -1).name
  return /comment|string/i.test(name)
}

const completion = (e: Entry, boost = 0): Completion => ({
  label: e.name,
  type: e.type,
  detail: e.sig && e.sig !== e.name ? e.sig.slice(e.sig.indexOf(e.name) + e.name.length) : undefined,
  info: e.sig ? `${e.sig}\n${e.info}` : e.info,
  boost,
})

// fieldNames are the keys of the script's fields table, which its self has.
export function fieldNames(text: string): string[] {
  const m = /^fields\s*=\s*\{/m.exec(text)
  if (!m) return []
  const out: string[] = []
  let depth = 1
  let line = ''
  for (let i = m.index + m[0].length; i < text.length && depth > 0; i++) {
    const c = text[i]
    if (c === '{') depth++
    else if (c === '}') depth--
    if (c === '\n') {
      line = ''
      continue
    }
    line += c
    const key = depth === 1 && c === '=' && /^\s*([A-Za-z_]\w*)\s*=$/.exec(line)
    if (key && text[i + 1] !== '=') out.push(key[1])
  }
  return out
}

// ownNames are what the script declares: its functions, locals, and what
// it keeps on self.
export function ownNames(text: string): { names: string[]; self: string[] } {
  const names = new Set<string>()
  const self = new Set<string>(fieldNames(text))
  for (const m of text.matchAll(/\b(?:local\s+function|function|local)\s+([A-Za-z_]\w*)/g)) names.add(m[1])
  for (const m of text.matchAll(/\blocal\s+([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)+)/g)) for (const n of m[1].split(',')) names.add(n.trim())
  for (const m of text.matchAll(/\bself\.([A-Za-z_]\w*)\s*=[^=]/g)) self.add(m[1])
  return { names: [...names], self: [...self] }
}

function membersAfter(recv: string, op: string): Entry[] {
  if (op === '.' && tables[recv]) return tables[recv]
  if (op === ':' && recv === 'd') return drawerMembers
  const all = [...objectMembers, ...componentMembers, ...vec3Members]
  const seen = new Set<string>()
  return all.filter((e) => (op === ':' ? e.type === 'method' : e.type === 'property') && !seen.has(e.name) && seen.add(e.name))
}

function source(ctx: CompletionContext): CompletionResult | null {
  if (quiet(ctx.state, ctx.pos)) return null
  const word = ctx.matchBefore(/[A-Za-z_]\w*$/)
  const from = word ? word.from : ctx.pos
  if (!word && !ctx.explicit && !/[.:]$/.test(ctx.state.sliceDoc(ctx.pos - 1, ctx.pos))) return null
  const line = ctx.state.doc.lineAt(ctx.pos)
  const before = line.text.slice(0, from - line.from)
  const text = ctx.state.doc.toString()

  const member = /([A-Za-z_]\w*)\s*([.:])\s*$/.exec(before)
  if (member) {
    const [, recv, op] = member
    const options = membersAfter(recv, op).map((e) => completion(e))
    if (recv === 'self' && op === '.') {
      for (const n of ownNames(text).self) options.push({ label: n, type: 'property', detail: 'on self', boost: 2 })
    }
    return { from, options, validFor: /^\w*$/ }
  }
  if (/[.:]\s*$/.test(before)) return null // a member of something unknown

  // function <name>: the hooks, written out with their parameters.
  if (/^\s*function\s+$/.test(before)) {
    return {
      from,
      options: hooks.map((h) => snippetCompletion(`${h.sig}\n\t\${}\nend`, { label: h.name, type: 'function', detail: h.sig!.slice(h.name.length), info: h.info })),
      validFor: /^\w*$/,
    }
  }

  const own = ownNames(text)
  const options: Completion[] = [
    ...globals.map((e) => completion(e, 1)),
    ...fieldTypes.map((e) => completion(e)),
    ...builtins.map((e) => completion(e, -1)),
    ...keywords.map((e) => completion(e, -2)),
    ...Object.keys(tables)
      .filter((t) => !globals.some((g) => g.name === t))
      .map((t) => ({ label: t, type: 'namespace', info: `Lua 5.1 ${t} library` })),
    ...own.names.filter((n) => !globals.some((g) => g.name === n)).map((n) => ({ label: n, type: 'variable', boost: 2 })),
  ]
  if (/^\s*$/.test(before) && !/^fields\s*=/m.test(text)) {
    options.push(snippetCompletion('fields = {\n\t${name} = ${1},\n}', { label: 'fields', type: 'keyword', detail: ' = { ... }', info: 'The values the Inspector edits.', boost: 3 }))
  }
  return { from, options, validFor: /^\w*$/ }
}

// lookup is the entry a word names, read with what comes before it.
export function lookup(word: string, before: string): Entry | undefined {
  const member = /([A-Za-z_]\w*)\s*([.:])\s*$/.exec(before)
  if (member) return membersAfter(member[1], member[2]).find((e) => e.name === word)
  if (/\bfunction\s+$/.test(before)) return hooks.find((e) => e.name === word)
  return [...globals, ...fieldTypes, ...builtins].find((e) => e.name === word)
}

const hover = hoverTooltip((view, pos) => {
  if (quiet(view.state, pos)) return null
  const line = view.state.doc.lineAt(pos)
  const at = pos - line.from
  let start = at
  let end = at
  while (start > 0 && /\w/.test(line.text[start - 1])) start--
  while (end < line.text.length && /\w/.test(line.text[end])) end++
  if (start === end) return null
  const entry = lookup(line.text.slice(start, end), line.text.slice(0, start))
  if (!entry) return null
  return {
    pos: line.from + start,
    end: line.from + end,
    above: true,
    create: () => {
      const dom = document.createElement('div')
      dom.className = 'ascdii-hover'
      if (entry.sig) {
        const sig = document.createElement('code')
        sig.textContent = entry.sig
        dom.appendChild(sig)
      }
      const info = document.createElement('div')
      info.textContent = entry.info
      dom.appendChild(info)
      return { dom }
    },
  }
})

// The .ascdiiproj is JSON; the editor carries no JSON grammar to name.
export const jsonSupport: Extension = new LanguageSupport(StreamLanguage.define(json))

export const luaSupport: Extension = [EditorState.languageData.of(() => [{ autocomplete: source }]), hover]
