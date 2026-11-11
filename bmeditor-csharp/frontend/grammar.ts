import type { StreamParser, StringStream } from '@codemirror/language'

// C# highlighting with the colours Visual Studio gives: namespaces, types,
// methods and members, not only keywords and strings. The editor's own C#
// grammar (CodeMirror's clike) leaves every name plain; the language server
// does not send colours. Names are told apart by how they are written and
// what follows them:
//
//   using UnityEngine.UI;          namespace
//   public class Player : Base     type (declared, and after :)
//   Vector3 pos; List<int> xs      PascalCase name that is not called: type
//   void Update() / Move(x)        name before ( or <T>(: method
//   transform.position             name after .: member
//   transform.Rotate(...)          member before (: method
//   [SerializeField]               attribute: type

const keywords = new Set(
  (
    'abstract as base break case catch checked class const continue default delegate do else enum event explicit extern ' +
    'false finally fixed for foreach goto if implicit in interface internal is lock namespace new null operator out override ' +
    'params private protected public readonly ref return sealed sizeof stackalloc static struct switch this throw true try ' +
    'typeof unchecked unsafe using virtual volatile while add alias async await by descending dynamic equals from get global ' +
    'group init into join let managed nameof nint not notnull nuint on or and orderby partial record remove required scoped ' +
    'select set unmanaged var when where with yield file'
  ).split(' '),
)
const builtinTypes = new Set('bool byte char decimal double float int long object sbyte short string uint ulong ushort void'.split(' '))
const declares = new Set(['class', 'struct', 'interface', 'enum', 'record', 'delegate'])
const atoms = new Set(['true', 'false', 'null'])

interface State {
  comment: boolean // in /* */
  raw: number // in a """ raw string: the number of quotes that close it, else 0
  verbatim: boolean // in a @"" string across lines
  namespace: boolean // after using or namespace, up to ; or {
  declare: boolean // the next name is a type being declared
  member: boolean // the last token was a .
  created: boolean // the last word was new: a type follows
  attribute: boolean // inside [ ] at the start of a line
  depth: number // { } depth, for indentation
}

// calledNext says whether a ( follows, with a generic argument list maybe.
function calledNext(s: StringStream): boolean {
  return /^\s*(<[\w\s,.<>?[\]]*>)?\s*\(/.test(s.string.slice(s.pos))
}

function string(s: StringStream, st: State, verbatim: boolean): string {
  // Interpolation holes are coloured as the string; good enough to read.
  while (!s.eol()) {
    const c = s.next()
    if (verbatim) {
      if (c === '"') {
        if (s.peek() === '"') s.next()
        else {
          st.verbatim = false
          return 'string'
        }
      }
    } else if (c === '\\') s.next()
    else if (c === '"') return 'string'
  }
  st.verbatim = verbatim
  return 'string'
}

export const csharp: StreamParser<State> = {
  name: 'csharp',
  startState: () => ({ comment: false, raw: 0, verbatim: false, namespace: false, declare: false, member: false, created: false, attribute: false, depth: 0 }),
  copyState: (st) => ({ ...st }),
  token(s, st) {
    if (st.comment) {
      if (s.skipTo('*/')) {
        s.match('*/')
        st.comment = false
      } else s.skipToEnd()
      return 'comment'
    }
    if (st.raw) {
      const close = '"'.repeat(st.raw)
      if (s.skipTo(close)) {
        s.match(close)
        st.raw = 0
      } else s.skipToEnd()
      return 'string'
    }
    if (st.verbatim) return string(s, st, true)
    if (s.sol()) st.attribute = false
    if (s.eatSpace()) return null

    if (s.match('//')) {
      s.skipToEnd()
      return 'comment'
    }
    if (s.match('/*')) {
      st.comment = true
      return this.token!(s, st)
    }
    if (s.sol() || /^\s*$/.test(s.string.slice(0, s.start))) {
      if (s.match(/^#\s*\w+/)) {
        s.skipToEnd()
        return 'meta'
      }
    }
    // Strings: """raw""", $"", @"", $@"", @$"", "" and 'c'.
    const raw = s.match(/^\$*("{3,})/) as RegExpMatchArray | null
    if (raw) {
      st.raw = raw[1].length
      return this.token!(s, st)
    }
    if (s.match(/^(\$@|@\$)"/)) return string(s, st, true)
    if (s.match(/^@"/)) return string(s, st, true)
    if (s.match(/^\$"/) || s.eat('"')) return string(s, st, false)
    if (s.match(/^'(\\.|[^'\\])+'/)) return 'string'
    if (s.match(/^(0x[\da-f_]+|0b[01_]+|(\d[\d_]*)?\.?\d[\d_]*(e[+-]?\d+)?)(ul|lu|u|l|f|d|m)?/i)) {
      st.member = false
      return 'number'
    }

    const c = s.peek()!
    if (c === '[' && /^\s*$/.test(s.string.slice(0, s.pos))) st.attribute = true
    if (/[{};]/.test(c)) {
      s.next()
      st.namespace = false
      st.declare = false
      st.member = false
      st.created = false
      if (c === '{') st.depth++
      else if (c === '}') st.depth = Math.max(0, st.depth - 1)
      return c === ';' ? 'punctuation' : 'bracket'
    }
    if (c === '.' || (c === '?' && s.string.charAt(s.pos + 1) === '.')) {
      s.match('?.') || s.next()
      st.member = true
      return 'punctuation'
    }

    const word = s.match(/^@?[A-Za-z_]\w*/) as RegExpMatchArray | null
    if (!word) {
      s.next()
      st.member = false
      st.created = false
      if (c === ']') st.attribute = false
      return /[()[\]]/.test(c) ? 'bracket' : /[,:]/.test(c) ? 'punctuation' : 'operator'
    }
    const w = word[0].replace(/^@/, '')
    const afterDot = st.member
    st.member = false
    const created = st.created
    st.created = false

    if (st.namespace) return 'namespace'
    if (st.declare) {
      st.declare = false
      return 'typeName.definition'
    }
    if (!afterDot && keywords.has(w) && !word[0].startsWith('@')) {
      if (w === 'using' || w === 'namespace') {
        // using var x = ...; and using (...) are statements, not namespaces.
        st.namespace = !/^\s*(var\b|\(|static\s+\w+\s*=|[\w.]+\s*=)/.test(s.string.slice(s.pos)) || /^\s*static\b/.test(s.string.slice(s.pos))
      }
      if (declares.has(w)) st.declare = true
      if (w === 'new') st.created = true
      return atoms.has(w) ? 'atom' : 'keyword'
    }
    if (builtinTypes.has(w)) return 'keyword'
    if (st.attribute || created) return 'typeName'
    if (calledNext(s)) return afterDot ? 'propertyName.function' : 'variableName.function'
    // System.IO.File: a PascalCase name with a . after it is a type or namespace.
    if (afterDot) return /^[A-Z]/.test(w) && /^\s*\./.test(s.string.slice(s.pos)) ? 'typeName' : 'propertyName'
    if (/^[A-Z]/.test(w)) {
      // ALL_CAPS is a constant.
      if (/^[A-Z][A-Z0-9_]+$/.test(w) && w.length > 1) return 'variableName.constant'
      return 'typeName'
    }
    return 'variableName'
  },
  indent(st, textAfter, cx) {
    const depth = st.depth - (/^\s*}/.test(textAfter) ? 1 : 0)
    return Math.max(0, depth) * cx.unit
  },
  languageData: {
    commentTokens: { line: '//', block: { open: '/*', close: '*/' } },
    closeBrackets: { brackets: ['(', '[', '{', '"', "'"] },
    indentOnInput: /^\s*}$/,
  },
}
