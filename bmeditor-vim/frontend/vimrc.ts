// Reads a vimrc into what Vim mode can apply: key mappings, unmaps and
// options. A vimrc is Vimscript, and most of it (plugins, autocommands,
// functions, highlighting) means nothing here, so a line this does not
// understand is skipped rather than reported: a real vimrc then works as it
// is, with the parts that make sense in an editor tab.

export type Mode = 'normal' | 'visual' | 'insert' | 'operatorPending'

export type Action =
  | { kind: 'map'; lhs: string; rhs: string; mode?: Mode; noremap: boolean }
  | { kind: 'unmap'; lhs: string; mode?: Mode }
  | { kind: 'set'; name: string; value: string | boolean | 'toggle' }

// Vim's map commands with the shortest abbreviation each accepts.
// codemirror-vim has no select mode, so x is the whole of visual mode.
const mapCommands: [name: string, min: string, mode: Mode | undefined, noremap: boolean][] = [
  ['map', 'map', undefined, false],
  ['nmap', 'nm', 'normal', false],
  ['vmap', 'vm', 'visual', false],
  ['xmap', 'xm', 'visual', false],
  ['omap', 'om', 'operatorPending', false],
  ['imap', 'im', 'insert', false],
  ['noremap', 'no', undefined, true],
  ['nnoremap', 'nn', 'normal', true],
  ['vnoremap', 'vn', 'visual', true],
  ['xnoremap', 'xn', 'visual', true],
  ['onoremap', 'ono', 'operatorPending', true],
  ['inoremap', 'ino', 'insert', true],
]

const unmapCommands: [name: string, min: string, mode: Mode | undefined][] = [
  ['unmap', 'unm', undefined],
  ['nunmap', 'nun', 'normal'],
  ['vunmap', 'vu', 'visual'],
  ['xunmap', 'xu', 'visual'],
  ['ounmap', 'ou', 'operatorPending'],
  ['iunmap', 'iu', 'insert'],
]

// Words that open and close a block. What is inside one (a function, an
// if has('nvim'), an augroup) runs under conditions this cannot check.
const opens = [['function', 'fu'], ['if', 'if'], ['for', 'for'], ['while', 'wh'], ['try', 'try']]
const closes = [['endfunction', 'endf'], ['endif', 'en'], ['endfor', 'endfo'], ['endwhile', 'endw'], ['endtry', 'endt']]

function abbreviates(word: string, name: string, min: string): boolean {
  return word.length >= min.length && name.startsWith(word)
}

function isOneOf(word: string, list: string[][]): boolean {
  return list.some(([name, min]) => abbreviates(word, name, min))
}

// Vim's own key names, as a key press is named in codemirror-vim.
const keyNames: Record<string, string> = {
  esc: 'Esc', cr: 'CR', return: 'CR', enter: 'CR', space: 'Space', tab: 'Tab',
  bs: 'BS', backspace: 'BS', del: 'Del', delete: 'Del', ins: 'Ins', insert: 'Ins',
  up: 'Up', down: 'Down', left: 'Left', right: 'Right', home: 'Home', end: 'End',
  pageup: 'PageUp', pagedown: 'PageDown',
}
const literal: Record<string, string> = { lt: '<', bar: '|', bslash: '\\', nop: '' }
// In the order codemirror-vim writes them; Vim's M- is Alt, D- is Cmd.
const modifierOrder = ['C', 'A', 'M', 'S']
const modifierNames: Record<string, string> = { c: 'C', a: 'A', m: 'A', d: 'M', s: 'S' }

function normalizeKey(name: string): string | undefined {
  const parts = name.split('-')
  let key = parts.pop() ?? ''
  if (key === '' && parts.length > 0) {
    parts.pop()
    key = '-' // <C-->
  }
  const mods: string[] = []
  for (const p of parts) {
    const m = modifierNames[p.toLowerCase()]
    if (!m) return undefined
    if (!mods.includes(m)) mods.push(m)
  }
  const lower = key.toLowerCase()
  if (mods.length === 0 && lower in literal) return literal[lower]
  if (lower in keyNames) key = keyNames[lower]
  else if (/^f\d{1,2}$/.test(lower)) key = 'F' + lower.slice(1)
  else if (key.length === 1) {
    if (mods.includes('C')) key = key.toLowerCase() // <C-D> is <C-d> in Vim
  } else return undefined
  if (mods.length === 0) return key.length === 1 ? key : `<${key}>`
  mods.sort((a, b) => modifierOrder.indexOf(a) - modifierOrder.indexOf(b))
  return `<${mods.join('-')}-${key}>`
}

// keys rewrites Vim's key notation as codemirror-vim reads it, or is
// undefined when it names something only Vim has (<Plug>, <SID>, <Cmd>).
function keys(text: string, leader: string, localLeader: string): string | undefined {
  text = text.replace(/<leader>/gi, () => leader).replace(/<localleader>/gi, () => localLeader)
  let ok = true
  const out = text.replace(/<([^<>\s]+)>/g, (whole, name: string) => {
    const k = normalizeKey(name)
    if (k === undefined) ok = false
    return k ?? whole
  })
  return ok ? out : undefined
}

// The rest of a map line after its lhs: up to a | that ends the command,
// with \| standing for a | that is part of it.
function rhsOf(text: string): string {
  let out = ''
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '\\' && text[i + 1] === '|') {
      out += '|'
      i++
    } else if (text[i] === '|') break
    else out += text[i]
  }
  return out.trim()
}

// The value of a let: a quoted string, '' being a ' in single quotes and
// "\<Space>" naming a key in double ones. Anything after it is a comment.
function stringValue(text: string): string | undefined {
  const q = text[0]
  if (q !== "'" && q !== '"') return undefined
  let out = ''
  for (let i = 1; i < text.length; i++) {
    const c = text[i]
    if (q === "'" && c === "'") {
      if (text[i + 1] !== "'") return out
      out += "'"
      i++
    } else if (q === '"' && c === '"') {
      return out
    } else if (q === '"' && c === '\\' && i + 1 < text.length) {
      const key = /^<[^<>\s]+>/.exec(text.slice(i + 1))
      if (key) {
        out += key[0]
        i += key[0].length
      } else {
        out += text[i + 1] === 't' ? '\t' : text[i + 1]
        i++
      }
    } else out += c
  }
  return undefined // not closed
}

function setAction(arg: string): Action | undefined {
  const eq = arg.search(/[+\-^]?=/)
  if (eq >= 0) {
    if (arg[eq] !== '=') return undefined // += and the like need the old value's type
    return { kind: 'set', name: arg.slice(0, eq), value: arg.slice(eq + 1).replace(/\\(.)/g, '$1') }
  }
  if (arg.endsWith('?') || arg.endsWith('&') || arg.startsWith('all')) return undefined
  if (arg.endsWith('!')) return { kind: 'set', name: arg.slice(0, -1), value: 'toggle' }
  if (arg.startsWith('inv')) return { kind: 'set', name: arg.slice(3), value: 'toggle' }
  if (arg.startsWith('no')) return { kind: 'set', name: arg.slice(2), value: false }
  return { kind: 'set', name: arg, value: true }
}

// lines joins Vim's continuation lines, which start with a backslash.
function lines(text: string): string[] {
  const out: string[] = []
  for (const raw of text.split(/\r?\n/)) {
    const t = raw.trimStart()
    if (t.startsWith('"\\ ')) continue
    if (t.startsWith('\\') && out.length > 0) out[out.length - 1] += t.slice(1)
    else out.push(raw)
  }
  return out
}

export function parse(text: string): Action[] {
  const actions: Action[] = []
  let leader = '\\'
  let localLeader = '\\'
  let depth = 0
  for (const raw of lines(text)) {
    const line = raw.trim().replace(/^:+\s*/, '')
    if (line === '' || line.startsWith('"')) continue
    const [, word = '', rest = ''] = /^([A-Za-z]+!?)\s*(.*)$/.exec(line) ?? []
    const bare = word.replace(/!$/, '').toLowerCase()

    // if has('x') | set y | endif opens and closes on one line.
    const oneLine = /\|\s*end\w*\s*$/.test(line)
    if (isOneOf(bare, opens)) depth += oneLine ? 0 : 1
    else if (isOneOf(bare, closes)) depth = Math.max(0, depth - 1)
    else if (bare === 'augroup' || bare === 'aug') depth = /^end\b/i.test(rest) ? Math.max(0, depth - 1) : depth + 1
    if (depth > 0 || isOneOf(bare, opens) || isOneOf(bare, closes)) continue

    if (bare === 'let') {
      const m = /^(g:)?(mapleader|maplocalleader)\s*=\s*(.*)$/.exec(rest)
      const v = m ? stringValue(m[3]) : undefined
      if (m && v !== undefined) {
        const k = keys(v, leader, localLeader) ?? v
        if (m[2] === 'mapleader') leader = k === ' ' ? '<Space>' : k
        else localLeader = k === ' ' ? '<Space>' : k
      }
      continue
    }

    if (['set', 'se', 'setglobal', 'setg', 'setlocal', 'setl'].includes(bare)) {
      for (const arg of rest.replace(/\s+".*$/, '').split(/(?<!\\)\s+/)) {
        const a = arg && setAction(arg)
        if (a) actions.push(a)
      }
      continue
    }

    const bang = word.endsWith('!')
    const map = mapCommands.find(([name, min]) => abbreviates(bare, name, min))
    const unmap = unmapCommands.find(([name, min]) => abbreviates(bare, name, min))
    if (!map && !unmap) continue
    // map! is insert and command-line mode, and only insert is here; nmap!
    // and the like are no commands.
    if (bang && (map ?? unmap)![2] !== undefined) continue

    let args = rest
    let expr = false
    for (;;) {
      const m = /^<(silent|buffer|nowait|unique|special|script|expr)>\s*/i.exec(args)
      if (!m) break
      if (m[1].toLowerCase() === 'expr') expr = true
      args = args.slice(m[0].length)
    }
    if (expr) continue // its rhs is Vimscript to evaluate
    const split = /^(\S+)\s*(.*)$/.exec(args)
    if (!split) continue
    const lhs = keys(split[1], leader, localLeader)
    if (!lhs) continue

    if (unmap) {
      actions.push({ kind: 'unmap', lhs, mode: bang ? 'insert' : unmap[2] })
      continue
    }
    const rhsText = rhsOf(split[2])
    if (rhsText === '') continue // lists the mappings in Vim
    const rhs = keys(rhsText, leader, localLeader)
    if (rhs === undefined) continue
    actions.push({ kind: 'map', lhs, rhs, mode: bang ? 'insert' : map![2], noremap: map![3] })
  }
  return actions
}
