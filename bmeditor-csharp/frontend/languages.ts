import { LanguageSupport, StreamLanguage } from '@codemirror/language'
import { css } from '@codemirror/legacy-modes/mode/css'
import { json } from '@codemirror/legacy-modes/mode/javascript'
import { xml } from '@codemirror/legacy-modes/mode/xml'
import { unityCompletion } from './completion'
import './completion.css'

// Highlighting for the languages the manifest declares without a grammar
// the editor carries: assembly definitions are JSON, MSBuild files and
// UXML are XML, USS is CSS. C# keeps the editor's grammar and gains the
// completion of Unity's messages.
const support = (parser: Parameters<typeof StreamLanguage.define>[0]) => new LanguageSupport(StreamLanguage.define(parser))

const grammars = {
  asmdef: support(json),
  msbuild: support(xml),
  uss: support(css),
}

export const languages = [
  ...Object.entries(grammars).map(([id, lang]) => ({ id, support: () => lang })),
  { id: 'csharp', support: () => unityCompletion },
]
