import { reactive } from 'vue'
import type { ExtensionAPI, Tab } from '../bmext/bmeditor'

// What each open file holds that is not saved yet, by tab: the bytes
// changed, and the order they changed in, for undo. Kept here rather than
// in the view, so that Ctrl+S saves a tab whose view is not on screen.

export let api: ExtensionAPI

export function setApi(a: ExtensionAPI) {
  api = a
}

export interface Edits {
  changed: Map<number, number> // offset → the byte it holds now
  undo: { offset: number; before: number | undefined }[] // before: undefined when it was not changed yet
}

const edits = reactive(new Map<string, Edits>())

export function editsOf(tab: Tab): Edits {
  let e = edits.get(tab.id)
  if (!e) {
    e = reactive({ changed: new Map<number, number>(), undo: [] }) as Edits
    edits.set(tab.id, e)
  }
  return e
}

export function set(tab: Tab, offset: number, value: number) {
  const e = editsOf(tab)
  e.undo.push({ offset, before: e.changed.get(offset) })
  e.changed.set(offset, value)
  tab.dirty = true
}

export function undo(tab: Tab): number | undefined {
  const e = editsOf(tab)
  const last = e.undo.pop()
  if (!last) return undefined
  if (last.before === undefined) e.changed.delete(last.offset)
  else e.changed.set(last.offset, last.before)
  tab.dirty = e.changed.size > 0
  return last.offset
}

const toBase64 = (bytes: number[]) => btoa(String.fromCharCode(...bytes))

export function fromBase64(s: string | null): Uint8Array {
  const raw = atob(s ?? '')
  const out = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

// save writes the changed bytes over the file, a run of neighbours at a
// time, and forgets them.
export async function save(tab: Tab): Promise<boolean> {
  const e = editsOf(tab)
  if (!e.changed.size) return true
  const offsets = [...e.changed.keys()].sort((a, b) => a - b)
  const runs: { offset: number; data: string }[] = []
  let start = offsets[0]
  let bytes: number[] = []
  offsets.forEach((o, i) => {
    if (i > 0 && o !== offsets[i - 1] + 1) {
      runs.push({ offset: start, data: toBase64(bytes) })
      start = o
      bytes = []
    }
    bytes.push(e.changed.get(o)!)
  })
  runs.push({ offset: start, data: toBase64(bytes) })
  try {
    await api.call('write', { path: tab.path, changes: runs })
  } catch (err) {
    api.ui.toastFailure(`Cannot save ${tab.name}`, api.ui.failureOf(err))
    return false
  }
  e.changed.clear()
  e.undo.length = 0
  tab.dirty = false
  return true
}
