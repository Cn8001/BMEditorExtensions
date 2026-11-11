import { LanguageSupport, StreamLanguage } from '@codemirror/language'
import { css } from '@codemirror/legacy-modes/mode/css'
import { json } from '@codemirror/legacy-modes/mode/javascript'
import { xml } from '@codemirror/legacy-modes/mode/xml'
import { unityCompletion } from './completion'
import './completion.css'
import { csharp } from './grammar'

// Highlighting for the languages the manifest declares without a grammar
// the editor carries: assembly definitions are JSON, MSBuild files and
// UXML are XML, USS is CSS. C# gets a grammar of its own (grammar.ts),
// which colours namespaces, types, methods and members as Visual Studio
// does, and the completion of Unity's messages.
const support = (parser: Parameters<typeof StreamLanguage.define>[0]) => new LanguageSupport(StreamLanguage.define(parser))

const grammars = {
  asmdef: support(json),
  msbuild: support(xml),
  uss: support(css),
}

const csharpLanguage = new LanguageSupport(StreamLanguage.define(csharp), [unityCompletion])

export const languages = [
  ...Object.entries(grammars).map(([id, lang]) => ({ id, support: () => lang })),
  { id: 'csharp', support: () => csharpLanguage },
]
