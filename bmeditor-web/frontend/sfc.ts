import type { StreamParser, StringStream } from '@codemirror/language'
import { html } from '@codemirror/legacy-modes/mode/xml'
import { javascript, typescript } from '@codemirror/legacy-modes/mode/javascript'
import { css } from '@codemirror/legacy-modes/mode/css'

// Single-file components, as Svelte and Vue write them: HTML with the code
// of <script> (TypeScript with lang="ts"), the CSS of <style>, and
// expressions in the markup, {x} in Svelte and {{ x }} in Vue. Each part is
// read by the editor's own tokenizer for it.

type Part = 'script' | 'ts' | 'style'
const parts: Record<Part, StreamParser<any>> = { script: javascript, ts: typescript, style: css }

interface State {
  unit: number
  html: any
  part: Part | null // inside <script> or <style>
  partState: any
  opening: Part | null // a <script or <style tag, waiting for its >
  expr: any // inside an expression, a JavaScript state
  depth: number // braces opened inside the expression
}

const copy = (s: any) => {
  const out: any = {}
  for (const k in s) out[k] = Array.isArray(s[k]) ? s[k].slice() : s[k]
  return out
}

// open and close are what marks an expression: { } or {{ }}. Svelte's
// {#if}, {:else}, {/each} and {@html} are keywords.
function component(name: string, open: string, close: string): StreamParser<State> {
  function inPart(stream: StringStream, state: State): string | null {
    const tag = state.part === 'style' ? '</style' : '</script'
    const end = stream.string.toLowerCase().indexOf(tag, stream.pos)
    if (end === stream.pos) {
      state.part = null
      return html.token(stream, state.html)
    }
    const style = parts[state.part!].token(stream, state.partState)
    // Stop at the closing tag, which belongs to the HTML.
    if (end > -1 && stream.pos > end) stream.backUp(stream.pos - end)
    return style
  }

  function inExpr(stream: StringStream, state: State): string | null {
    if (state.depth === 0 && stream.match(close)) {
      state.expr = null
      return 'keyword'
    }
    const style = javascript.token(stream, state.expr)
    const text = stream.current()
    if (text === '{') state.depth++
    else if (text === '}') state.depth--
    return style
  }

  return {
    name,
    startState: (unit) => ({ unit, html: html.startState!(unit), part: null, partState: null, opening: null, expr: null, depth: 0 }),
    copyState: (s) => ({ ...s, html: copy(s.html), partState: s.partState && copy(s.partState), expr: s.expr && copy(s.expr) }),
    token(stream, state) {
      if (state.expr) return inExpr(stream, state)
      if (state.part) return inPart(stream, state)
      if (stream.match(open)) {
        const block = open === '{' && stream.match(/^[#:/@][a-z]+/)
        state.expr = javascript.startState!(state.unit)
        state.depth = 0
        return block ? 'keyword' : 'bracket'
      }
      const style = html.token(stream, state.html)
      // Plain text runs on to the next tag; an expression in it starts its own token.
      const at = style === null ? stream.current().indexOf(open) : -1
      if (at > 0) stream.backUp(stream.current().length - at)
      const text = stream.current()
      if (/^(script|style)$/i.test(text) && /<\s*$/.test(stream.string.slice(0, stream.start))) {
        const lower = text.toLowerCase()
        state.opening = lower === 'style' ? 'style' : /\blang=["']ts["']/.test(stream.string) ? 'ts' : 'script'
      } else if (state.opening && text === '>') {
        state.part = state.opening
        state.partState = parts[state.part].startState!(state.unit)
        state.opening = null
      } else if (text === '/>') {
        state.opening = null
      }
      return style
    },
    languageData: { commentTokens: { block: { open: '<!--', close: '-->' } } },
  }
}

export const svelte = component('svelte', '{', '}')
export const vue = component('vue', '{{', '}}')
