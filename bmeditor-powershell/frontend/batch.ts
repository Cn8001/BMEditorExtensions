import type { StreamParser, StringStream } from '@codemirror/language'

// Highlighting of cmd batch files: comments (rem, ::), labels, the
// commands and keywords, variables (%name%, !name!, %1, %~dp0, %%i),
// strings, redirections and the text echo prints.

const keywords = new Set(
  'if else for in do goto call exit not exist defined errorlevel equ neq lss leq gtr geq setlocal endlocal shift enabledelayedexpansion disabledelayedexpansion enableextensions disableextensions'.split(' '),
)
const builtins = new Set(
  'assoc break cd chdir cls color copy date del dir echo erase ftype md mkdir mklink move path pause popd prompt pushd rd ren rename rmdir set start time title type ver verify vol'.split(' '),
)

interface State {
  start: boolean // where a command starts
  echo: boolean // in the text echo prints
  command: string // the line's command so far
}

function variable(s: StringStream): boolean {
  // %%i, %~dp0, %~nx1, %1, %*, %name%, %name:a=b%, !name!
  return (
    s.match(/^%%~?[a-zA-Z$]*[a-zA-Z]/) !== null ||
    s.match(/^%~[fdpnxsatz$]*[0-9]/i) !== null ||
    s.match(/^%[0-9*]/) !== null ||
    s.match(/^%[^%\s]+%/) !== null ||
    s.match(/^![^!\s]+!/) !== null
  )
}

export const batch: StreamParser<State> = {
  name: 'batch',
  startState: () => ({ start: true, echo: false, command: '' }),
  token(s, st) {
    if (s.sol()) {
      st.start = true
      st.echo = false
      st.command = ''
    }
    if (s.eatSpace()) return null
    if (st.start) {
      if (s.match(/^@?rem(\s|$)/i) || s.match(/^::/)) {
        s.skipToEnd()
        return 'comment'
      }
      if (s.match(/^:[^\s:+=,;"]+/)) return 'labelName'
      if (s.eat('@')) return 'meta'
    }
    if (variable(s)) return 'variableName.special'
    if (s.match(/^\d?>>?(&\d)?/) || s.match(/^<|^&&?|^\|\|?/)) {
      st.start = true
      st.echo = false
      return 'operator'
    }
    if (s.eat('(') || s.eat(')')) {
      st.start = true
      st.echo = false
      return 'bracket'
    }
    if (st.echo) {
      if (st.command === 'echo' && s.match(/^(on|off)(?=\s|$)/i)) {
        st.command = ''
        return 'keyword'
      }
      st.command = ''
      s.match(/^[^%!&|<>()]+/) || s.next()
      return 'string'
    }
    if (s.eat('"')) {
      while (!s.eol()) {
        if (s.peek() === '%' || s.peek() === '!') break
        if (s.next() === '"') break
      }
      return 'string'
    }
    if (s.match(/^==/) || s.eat('=')) return 'operator'
    if (s.match(/^\/[a-zA-Z?]+:?/)) return 'attributeName' // switches: /s /q /f:
    if (s.match(/^-?\d+\b/)) return 'number'
    const word = s.match(/^[^\s%!&|<>()"=]+/) as RegExpMatchArray | null
    if (!word) {
      s.next()
      return null
    }
    const w = word[0].toLowerCase()
    const wasStart = st.start
    st.start = false
    if (keywords.has(w)) {
      // What follows if, for ... do, else is a command again.
      if (w === 'do' || w === 'else') st.start = true
      if (w === 'goto' || w === 'call') st.command = w
      return 'keyword'
    }
    if (st.command === 'goto' || st.command === 'call') {
      const target = w.startsWith(':') || st.command === 'goto'
      st.command = ''
      return target ? 'labelName' : null
    }
    if (wasStart && (builtins.has(w) || w === 'echo.')) {
      st.command = w
      if (w === 'echo' || w === 'echo.' || w === 'title') st.echo = true
      return 'builtin'
    }
    if (wasStart) return 'variableName.function'
    return null
  },
  languageData: {
    commentTokens: { line: 'rem' },
  },
}
