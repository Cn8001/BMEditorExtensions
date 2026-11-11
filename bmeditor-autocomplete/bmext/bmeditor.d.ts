// The API a bmeditor extension's script gets: its default export is called
// with it once, when the editor starts.
//
//   export default function activate(api: ExtensionAPI) {
//     api.register({ panels: [...], commands: [...] })
//   }
//
// Kept in step with frontend/src/lib/extensionHost.ts in the editor; version
// says which shape this is.

import type { Component } from 'vue'
import type { EditorState, Extension, TransactionSpec } from '@codemirror/state'

export interface Tab {
  id: string
  path: string
  name: string
  language: string
  version: number // rises with every edit
  dirty: boolean
}

export interface Failure {
  code: string
  message: string
  line?: number
  col?: number
}

export interface Problem {
  file: string
  line: number
  col: number
  severity: 'error' | 'warning' | 'info'
  message: string
  source?: string
}

export interface Contributions {
  // Behaviour for a language the manifest declares: more editor support
  // (highlighting from the backend, completion), and hooks for its tabs.
  languages?: { id: string; support?: (tab: Tab) => Extension; changed?: (tab: Tab) => void; closed?: (tab: Tab) => void }[]
  // More editor support for every tab, whatever its language (BMEditor
  // 0.1.5 and later): inline completion, a gutter.
  editor?: (tab: Tab) => Extension
  panels?: { id: string; label: string; component: Component; busy?: () => boolean }[]
  toolbar?: Component // gets `compact` (0 to 3) as the top bar narrows
  commands?: { id: string; label: string; kbd?: string; icon?: string; run: () => unknown; when?: () => boolean }[]
  keydown?: (e: KeyboardEvent) => boolean // true when handled
  shortcuts?: [label: string, keys: string][]
  reset?: () => void // the project closed or another opened
}

export type CancellablePromise<T> = Promise<T> & { cancel: () => void }

export interface ExtensionAPI {
  version: 1
  name: string
  base: string // URL prefix of the extension's own files
  register(c: Contributions): void
  // call runs a method of the extension's backend; a rejection carries a
  // Failure (read it with ui.failureOf). cancel() cancels it there too.
  call<T = unknown>(method: string, params?: unknown): CancellablePromise<T>
  on(method: string, fn: (params: any) => void): () => void // backend notifications
  settings(): Promise<{ values: Record<string, string>; secrets: Record<string, boolean> }>
  session: {
    tab(id: string): Tab | undefined
    textOf(id: string): string
    stateOf(id: string): EditorState | undefined
    dispatchTo(id: string, spec: TransactionSpec): void
    reconfigureLanguage(language: string): void
    activate(id: string): void
    goto(line: number, col: number, length?: number): void
    // openFile opens a file in a tab, or switches to its tab, and goes to
    // pos when given. quiet leaves out the "Cannot open" notice on failure.
    openFile(path: string, pos?: { line: number; col: number; length?: number }, quiet?: boolean): Promise<void>
  }
  ide: { folder: string; activeId: string; tabs: Tab[]; settingsOpen: boolean; [key: string]: unknown }
  ui: {
    toast(kind: 'info' | 'error', title: string, detail?: string): void
    toastFailure(title: string, f: Failure): void
    failureOf(err: unknown): Failure
    setProblems(key: string, problems: Problem[]): void
    showPanel(id: string): void
    activeTab(): Tab | undefined
    openSettings(): void
    // output writes text, one line or several, to the extension's channel
    // in Output (stream info by default), where its backend's stderr goes
    // too; showOutput opens Output on that channel.
    output(text: string, stream?: 'info' | 'stdout' | 'stderr'): void
    showOutput(): void
  }
  paths: { relativeTo(root: string, path: string): string }
  components: { CodeView: Component } // props: text, language ('sql' | 'json' | 'plain')
  theme: { palette: Record<string, string> }
}
