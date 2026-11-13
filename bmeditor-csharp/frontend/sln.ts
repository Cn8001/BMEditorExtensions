import type { StreamParser } from '@codemirror/language'

// Visual Studio's .sln text format: the format line, Project("{type}") =
// "Name", "path", "{guid}" ... EndProject, and Global sections of
// key = value lines.

const keywords = /^(Project|EndProject|ProjectSection|EndProjectSection|Global|EndGlobal|GlobalSection|EndGlobalSection)\b/
const phases = /^(preSolution|postSolution|preProject|postProject)\b/
const guid = /^\{[0-9A-Fa-f]{8}(-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}\}/

interface State {
  value: boolean // past the = of a key = value line
}

export const sln: StreamParser<State> = {
  name: 'sln',
  startState: () => ({ value: false }),
  token(s, st) {
    if (s.sol()) {
      st.value = false
      if (s.match(/^Microsoft Visual Studio Solution File.*/)) return 'meta'
    }
    if (s.eatSpace()) return null
    if (s.peek() === '#') {
      s.skipToEnd()
      return 'comment'
    }
    if (s.match(keywords)) return 'keyword'
    if (s.match(phases)) return 'atom'
    if (s.match(/^"[^"]*"?/)) return 'string'
    if (s.match(guid)) return 'number'
    if (s.eat('=')) {
      st.value = true
      return 'operator'
    }
    if (s.match(/^[(),.|]/)) return null
    if (s.match(/^[^\s=(),"{|.]+/)) return st.value ? 'string' : 'propertyName'
    s.next()
    return null
  },
  languageData: { commentTokens: { line: '#' } },
}
