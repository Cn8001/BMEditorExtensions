import { EditorView } from '@codemirror/view'

// Closing tags as VS Code writes them, for the markup of Svelte, Vue and
// SVG: typing the > of <div> adds </div> after the cursor, and typing </
// finishes the innermost tag still open. HTML has this from the editor.

const voids = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'source', 'track', 'wbr'])
const tag = /<(\/?)([A-Za-z][\w:.-]*)(?:\s[^<>]*?)?(\/?)>/g

// openTags are the tags opened in text and not yet closed, outermost first.
export function openTags(text: string): string[] {
  const stack: string[] = []
  for (const [, closing, name, selfClosing] of text.matchAll(tag)) {
    if (closing) {
      const i = stack.lastIndexOf(name)
      if (i >= 0) stack.length = i
    } else if (!selfClosing && !voids.has(name.toLowerCase())) {
      stack.push(name)
    }
  }
  return stack
}

// inside is the <script> or <style> the end of before is in, if any.
function inside(before: string): string | null {
  const lower = before.toLowerCase()
  for (const part of ['script', 'style']) {
    const open = lower.lastIndexOf('<' + part)
    if (open >= 0 && open > lower.lastIndexOf('</' + part) && lower.indexOf('>', open) >= 0) return part
  }
  return null
}

// closingFor is what to insert when ch is typed after before, or null to
// leave the key to the editor.
export function closingFor(before: string, ch: string): { insert: string; cursor: number } | null {
  // In <script> and <style> a < is code, as in Array<string>: only their own
  // closing tag is finished there.
  const part = inside(before)
  if (part) return ch === '/' && before.endsWith('<') ? { insert: `/${part}>`, cursor: part.length + 2 } : null
  if (ch === '>') {
    const m = /<([A-Za-z][\w:.-]*)(?:\s[^<>]*)?$/.exec(before)
    if (!m || before.endsWith('/') || voids.has(m[1].toLowerCase())) return null
    return { insert: `></${m[1]}>`, cursor: 1 }
  }
  if (ch === '/' && before.endsWith('<')) {
    const name = openTags(before.slice(0, -1)).pop()
    return name ? { insert: `/${name}>`, cursor: name.length + 2 } : null
  }
  return null
}

export const autoCloseTags = EditorView.inputHandler.of((view, from, to, text) => {
  if (view.composing || from !== to || (text !== '>' && text !== '/')) return false
  const before = view.state.sliceDoc(Math.max(0, from - 100_000), from)
  const close = closingFor(before, text)
  if (!close) return false
  view.dispatch({ changes: { from, insert: close.insert }, selection: { anchor: from + close.cursor }, userEvent: 'input.type' })
  return true
})
