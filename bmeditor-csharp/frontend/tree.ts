import type { Project, Solution, SolutionItem } from './api'

// The Solution Explorer's tree: the solution, its folders and projects,
// each project's references and files, folders from the files' paths.

export interface Node {
  id: string
  label: string
  icon: string
  kind: 'solution' | 'sfolder' | 'project' | 'refs' | 'ref' | 'folder' | 'file' | 'error'
  path?: string // a file to open
  dir?: string // a folder New Script can write in
  detail?: string
  title?: string // on hover, where the path is not enough
  project?: Project // a project's, for its Properties
  children?: Node[]
  // lazy makes the children the first time they are asked for (kids): a
  // project's files, of which a large solution has tens of thousands.
  lazy?: () => Node[]
}

// kids is a node's children, made now if they were left for later.
export function kids(n: Node): Node[] {
  if (n.lazy) {
    n.children = n.lazy()
    n.lazy = undefined
  }
  return n.children ?? []
}

export function expandable(n: Node): boolean {
  return !!n.lazy || !!n.children?.length
}

export interface Row {
  node: Node
  depth: number
  open: boolean
}

function joinPath(dir: string, rel: string, sep: string): string {
  return dir + sep + rel.split('/').join(sep)
}

function dirOf(path: string): string {
  return path.slice(0, Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')))
}

function baseOf(path: string): string {
  return path.slice(Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1)
}

const fileIcons: Record<string, string> = {
  cs: 'file-code',
  asmdef: 'json',
  asmref: 'json',
  json: 'json',
  md: 'markdown',
  txt: 'file-text',
  uss: 'symbol-color',
  uxml: 'code',
  shader: 'symbol-color',
  hlsl: 'symbol-color',
  cginc: 'symbol-color',
  xml: 'code',
}

function fileIcon(name: string): string {
  return fileIcons[name.split('.').pop()!.toLowerCase()] ?? 'file'
}

// A collator made once compares many times faster than localeCompare,
// which a project of thousands of files sorts with.
const collator = new Intl.Collator(undefined, { sensitivity: 'base' })

function byLabel(a: Node, b: Node) {
  const folder = (n: Node) => (n.kind === 'folder' || n.kind === 'sfolder' ? 0 : 1)
  return folder(a) - folder(b) || collator.compare(a.label, b.label)
}

function projectNode(item: SolutionItem, sep: string): Node {
  const id = 'p:' + (item.path ?? item.name)
  if (!item.project) {
    return { id, label: item.name, icon: 'warning', kind: 'error', path: item.path, detail: item.error ?? 'not a C# project' }
  }
  const p: Project = item.project
  const node: Node = { id, label: p.name, icon: 'project', kind: 'project', dir: dirOf(p.path), path: p.path, project: p, title: [p.name, p.targetFramework, p.sdk ? 'SDK-style' : ''].filter(Boolean).join(' · ') }
  node.lazy = () => projectChildren(id, p, sep)
  return node
}

// projectChildren is a project's references and files, in folders.
function projectChildren(id: string, p: Project, sep: string): Node[] {
  const dir = dirOf(p.path)
  // References by kind, each group closed: a Unity project has a hundred
  // assemblies of the engine and the framework, which nobody reads through.
  const groups: Node[] = [
    { id: id + ':refs:p', label: 'Projects', icon: 'project', kind: 'refs' as const, children: (p.projectReferences ?? []).map((r) => ({ id: id + ':pr:' + r, label: baseOf(r).replace(/\.csproj$/i, ''), icon: 'project', kind: 'ref' as const, path: r })) },
    { id: id + ':refs:k', label: 'Packages', icon: 'package', kind: 'refs' as const, children: (p.packageReferences ?? []).map((r) => ({ id: id + ':pk:' + r.name, label: r.name, icon: 'package', kind: 'ref' as const, detail: r.version })) },
    { id: id + ':refs:a', label: 'Assemblies', icon: 'library', kind: 'refs' as const, children: (p.references ?? []).map((r) => ({ id: id + ':r:' + r.name, label: r.name, icon: 'library', kind: 'ref' as const, detail: r.path ? baseOf(dirOf(r.path)) : undefined })) },
  ].filter((g) => g.children!.length)
  for (const g of groups) g.detail = String(g.children!.length)
  const refs = groups.length === 1 ? groups[0].children! : groups
  const refCount = groups.reduce((n, g) => n + g.children!.length, 0)
  // The framework only on hover: on every row it crowded the names out.
  const root: Node = { id, label: p.name, icon: 'project', kind: 'project', dir, path: p.path, project: p, title: [p.name, p.targetFramework, p.sdk ? 'SDK-style' : ''].filter(Boolean).join(' · '), children: [] }
  if (refs.length) root.children!.push({ id: id + ':refs', label: 'References', icon: 'references', kind: 'refs', detail: String(refCount), children: refs })
  const folders = new Map<string, Node>()
  const folder = (relDir: string): Node => {
    if (!relDir) return root
    let f = folders.get(relDir)
    if (f) return f
    const parentRel = relDir.includes('/') ? relDir.slice(0, relDir.lastIndexOf('/')) : ''
    const name = relDir.slice(relDir.lastIndexOf('/') + 1)
    f = { id: id + ':d:' + relDir, label: name, icon: 'folder', kind: 'folder', children: [] }
    if (!relDir.startsWith('..')) f.dir = joinPath(dir, relDir, sep)
    folders.set(relDir, f)
    folder(parentRel).children!.push(f)
    return f
  }
  for (const file of p.files) {
    // A linked file (../x.cs) goes under one Linked folder.
    const rel = file.rel.startsWith('../') ? 'Linked/' + file.rel.replace(/^(\.\.\/)+/, '') : file.rel
    const relDir = rel.includes('/') ? rel.slice(0, rel.lastIndexOf('/')) : ''
    const name = rel.slice(rel.lastIndexOf('/') + 1)
    folder(relDir).children!.push({ id: id + ':f:' + file.rel, label: name, icon: fileIcon(name), kind: 'file', path: file.path, detail: file.kind === 'compile' ? undefined : file.kind })
  }
  const sort = (n: Node) => {
    if (!n.children) return
    const refsNode = n.children.filter((c) => c.kind === 'refs')
    n.children = [...refsNode, ...n.children.filter((c) => c.kind !== 'refs').sort(byLabel)]
    n.children.forEach(sort)
  }
  sort(root)
  compact(root)
  return root.children!
}

// compact joins a chain of folders that each hold only the next into one
// row, Assets/Scripts/Gameplay, as Rider and VS Code show them.
function compact(n: Node) {
  for (const [i, c] of (n.children ?? []).entries()) {
    let f = c
    while (f.kind === 'folder' && f.children?.length === 1 && f.children[0].kind === 'folder') {
      const next = f.children[0]
      f = { ...next, label: f.label + '/' + next.label }
    }
    n.children![i] = f
    compact(f)
  }
}

function itemNode(item: SolutionItem, sep: string): Node {
  if (item.kind === 'project') return projectNode(item, sep)
  const children = [
    ...(item.children ?? []).map((c) => itemNode(c, sep)),
    ...(item.files ?? []).map((f) => ({ id: 's:f:' + f, label: baseOf(f), icon: fileIcon(f), kind: 'file' as const, path: f })),
  ]
  return { id: 's:' + item.name + ':' + children.length, label: item.name, icon: 'folder-library', kind: 'sfolder', children }
}

// countProjects counts a solution's projects, and those that could be read.
function countProjects(items: SolutionItem[]): { all: number; read: number } {
  let all = 0
  let read = 0
  for (const it of items) {
    if (it.kind === 'project') {
      all++
      if (it.project) read++
    }
    const c = countProjects(it.children ?? [])
    all += c.all
    read += c.read
  }
  return { all, read }
}

// isPackageProject says whether every file of a project is in Unity's
// package cache: a registry, git or built-in package, which Visual Studio
// leaves out of a Unity solution by default. An embedded or a local
// package is the project's own and stays.
export function isPackageProject(p: Project): boolean {
  return p.files.length > 0 && p.files.every((f) => /[\\/]Library[\\/]PackageCache[\\/]/i.test(f.path))
}

// withoutPackages takes package projects out of items, into packages, and
// drops the solution folders that held nothing else.
function withoutPackages(items: SolutionItem[], packages: SolutionItem[]): SolutionItem[] {
  const out: SolutionItem[] = []
  for (const it of items) {
    if (it.kind === 'project') {
      if (it.project && isPackageProject(it.project)) packages.push(it)
      else out.push(it)
      continue
    }
    const children = withoutPackages(it.children ?? [], packages)
    if (children.length || it.files?.length) out.push({ ...it, children })
  }
  return out
}

export function solutionTree(s: Solution, sep: string, showPackages: boolean): Node {
  const packages: SolutionItem[] = []
  const own = withoutPackages(s.items, packages)
  const n = countProjects(own)
  const children = own.map((it) => itemNode(it, sep))
  if (showPackages && packages.length) {
    children.push({ id: 's:packages', label: 'Packages', icon: 'package', kind: 'sfolder', detail: String(packages.length), children: packages.map((it) => itemNode(it, sep)) })
  }
  // As Visual Studio says it: (3 of 4 projects) when one did not load.
  const detail = n.read === n.all ? `${n.all} project${n.all === 1 ? '' : 's'}` : `${n.read} of ${n.all} projects`
  return { id: 'solution:' + s.path, label: `Solution '${s.name}'`, icon: 'repo', kind: 'solution', path: s.path, dir: dirOf(s.path), detail, children }
}

// packageCount is how many package projects a solution has.
export function packageCount(s: Solution): number {
  const packages: SolutionItem[] = []
  withoutPackages(s.items, packages)
  return packages.length
}

// pathTo is the ids from root down to the node of a file, or null when the
// solution does not have it.
export function pathTo(root: Node, path: string): string[] | null {
  const want = path.toLowerCase()
  const visit = (n: Node, above: string[]): string[] | null => {
    if (n.kind === 'file' && n.path?.toLowerCase() === want) return [...above, n.id]
    // Only the project that has the file makes its children for this.
    if (n.kind === 'project' && n.project && !n.project.files.some((f) => f.path.toLowerCase() === want)) return null
    for (const c of kids(n)) {
      const found = visit(c, [...above, n.id])
      if (found) return found
    }
    return null
  }
  return visit(root, [])
}

// A project's file and reference names, lower case, one a line: made once,
// then every filter text is one search of it.
const namesOf = new WeakMap<Project, string>()

function mentions(p: Project, f: string): boolean {
  let names = namesOf.get(p)
  if (names === undefined) {
    names = [...p.files.map((x) => baseOf(x.rel)), ...(p.projectReferences ?? []).map(baseOf), ...(p.packageReferences ?? []).map((r) => r.name), ...(p.references ?? []).map((r) => r.name)]
      .join('\n')
      .toLowerCase()
    namesOf.set(p, names)
  }
  return names.includes(f)
}

// rows are the visible rows: open nodes' children, or with a filter, the
// files whose name has it and the nodes above them, all open.
export function rows(root: Node, open: Set<string>, filter: string, limit = Infinity): Row[] {
  const out: Row[] = []
  for (const r of walk(root, open, filter)) {
    if (out.length >= limit) break
    out.push(r)
  }
  return out
}

// walk makes the rows one at a time, as they are asked for: the list takes
// a page more as it scrolls, so a filter most files match costs only the
// rows seen, and nothing is left out.
export function* walk(root: Node, open: Set<string>, filter: string): Generator<Row> {
  const f = filter.trim().toLowerCase()
  const keep = new Map<Node, boolean>()
  const matches = (n: Node): boolean => {
    if (keep.has(n)) return keep.get(n)!
    const self = n.kind === 'file' || n.kind === 'ref' ? n.label.toLowerCase().includes(f) : false
    // A project not made yet is looked into first by its files' and
    // references' names, so only one that has a match makes its children.
    if (n.lazy && n.project && !mentions(n.project, f)) {
      keep.set(n, false)
      return false
    }
    const any = self || kids(n).some(matches)
    keep.set(n, any)
    return any
  }
  function* visit(n: Node, depth: number): Generator<Row> {
    if (f && !matches(n)) return
    const isOpen = f ? expandable(n) : open.has(n.id)
    yield { node: n, depth, open: isOpen }
    if (isOpen) for (const c of kids(n)) yield* visit(c, depth + 1)
  }
  yield* visit(root, 0)
}
