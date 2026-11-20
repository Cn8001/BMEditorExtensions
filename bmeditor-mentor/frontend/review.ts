import { api, cursor, diff, loadChanges, markReviewed, openAt, store, type AgentFile, type Change } from './api'

// Reviewing what the agent changed without leaving the code: F7 and
// Shift+F7 go to the next and the one before of its changed declarations,
// file after file, and Alt+R marks the file reviewed and goes on to the
// next. The strip above each file it changed (strip.ts) says where you are.

// A stop is a declaration the agent added or changed, or a file it changed
// that has none (not Go, or deleted), in the order of the Changes tab.
export interface Stop {
  file: string
  line: number
  change?: Change
}

export function stops(): Stop[] {
  const out: Stop[] = []
  for (const f of store.report?.files ?? []) {
    const live = f.changes.filter((c) => c.change !== 'removed')
    if (live.length && f.status !== 'D') for (const c of live) out.push({ file: f.path, line: c.line, change: c })
    else out.push({ file: f.path, line: 1 })
  }
  return out
}

export function fileOf(path: string): AgentFile | undefined {
  return store.report?.files.find((f) => f.path === path)
}

// live are a file's declarations that are still there, which a review mark
// is kept for.
export function live(f: AgentFile): Change[] {
  return f.changes.filter((c) => c.change !== 'removed')
}

export function reviewed(f: AgentFile): boolean {
  const l = live(f)
  return l.length > 0 && l.every((c) => c.review === 'reviewed')
}

// before says whether stop s comes before the place path:line, in the
// order of the stops.
function before(list: Stop[], s: Stop, path: string, line: number): number {
  const order = (p: string) => {
    const i = list.findIndex((x) => x.file === p)
    return i >= 0 ? i : list.filter((x) => x.file < p).length - 0.5
  }
  const a = order(s.file)
  const b = order(path)
  if (a !== b) return a - b
  return s.line - line
}

export async function goTo(s: Stop) {
  const f = fileOf(s.file)
  if (f?.status === 'D') await diff(s.file)
  else await openAt(s.file, s.line)
}

// step goes to the next stop after the cursor (dir 1) or the one before
// it (dir -1), round to the start at the end.
export async function step(dir: 1 | -1) {
  if (!store.report) await loadChanges()
  const list = stops()
  if (!list.length) {
    api.ui.toast('info', 'Nothing the agent changed differs from HEAD')
    return
  }
  const tab = api.ui.activeTab()
  const line = cursor().line
  let i = -1
  if (tab) {
    if (dir > 0) i = list.findIndex((s) => before(list, s, tab.path, line) > 0)
    else
      for (let j = list.length - 1; j >= 0; j--)
        if (before(list, list[j], tab.path, line) < 0) {
          i = j
          break
        }
  }
  if (i < 0) i = dir > 0 ? 0 : list.length - 1
  await goTo(list[i])
}

// reviewAndNext marks the file in front reviewed, as it is now, and goes
// on to the first stop of the next file not reviewed.
export async function reviewAndNext() {
  const tab = api.ui.activeTab()
  const f = tab && fileOf(tab.path)
  if (f && live(f).length && !reviewed(f)) await markReviewed(live(f).map((c) => c.key))
  const files = store.report?.files ?? []
  const at = f ? files.indexOf(f) : -1
  const next = [...files.slice(at + 1), ...files.slice(0, Math.max(at, 0))].find((x) => x !== f && !reviewed(x))
  if (!next) {
    api.ui.toast('info', 'You have seen all the agent changed')
    return
  }
  const first = stops().find((s) => s.file === next.path)
  if (first) await goTo(first)
}

// keydown takes Mentor's keys, which the editor leaves free.
export function keydown(e: KeyboardEvent, explain: () => unknown): boolean {
  if (e.key === 'F7' && !e.ctrlKey && !e.altKey && !e.metaKey) {
    void step(e.shiftKey ? -1 : 1)
    return true
  }
  if (e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey && e.code === 'KeyR' && api.ide.folder) {
    void reviewAndNext()
    return true
  }
  if (e.ctrlKey && e.altKey && !e.shiftKey && e.code === 'KeyU') {
    void explain()
    return true
  }
  return false
}
