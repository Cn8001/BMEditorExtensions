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
  children?: Node[]
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

function byLabel(a: Node, b: Node) {
  const folder = (n: Node) => (n.kind === 'folder' || n.kind === 'sfolder' ? 0 : 1)
  return folder(a) - folder(b) || a.label.localeCompare(b.label, undefined, { sensitivity: 'base' })
}

function projectNode(item: SolutionItem, sep: string): Node {
  const id = 'p:' + (item.path ?? item.name)
  if (!item.project) {
    return { id, label: item.name, icon: 'warning', kind: 'error', path: item.path, detail: item.error ?? 'not a C# project' }
  }
  const p: Project = item.project
  const dir = dirOf(p.path)
  const refs: Node[] = [
    ...(p.projectReferences ?? []).map((r) => ({ id: id + ':pr:' + r, label: baseOf(r).replace(/\.csproj$/i, ''), icon: 'project', kind: 'ref' as const, path: r })),
    ...(p.packageReferences ?? []).map((r) => ({ id: id + ':pk:' + r.name, label: r.name, icon: 'package', kind: 'ref' as const, detail: r.version })),
    ...(p.references ?? []).map((r) => ({ id: id + ':r:' + r.name, label: r.name, icon: 'library', kind: 'ref' as const, detail: r.path ? baseOf(dirOf(r.path)) : undefined })),
  ]
  const root: Node = { id, label: p.name, icon: 'project', kind: 'project', dir, detail: [p.targetFramework, p.sdk ? 'SDK' : ''].filter(Boolean).join(' · '), children: [] }
  if (refs.length) root.children!.push({ id: id + ':refs', label: 'References', icon: 'references', kind: 'refs', detail: String(refs.length), children: refs })
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
  return root
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
  let detail = n.read === n.all ? `${n.all} project${n.all === 1 ? '' : 's'}` : `${n.read} of ${n.all} projects`
  if (packages.length && !showPackages) detail += ` · ${packages.length} package projects hidden`
  return { id: 'solution:' + s.path, label: `Solution '${s.name}'`, icon: 'repo', kind: 'solution', path: s.path, dir: dirOf(s.path), detail, children }
}

// pathTo is the ids from root down to the node of a file, or null when the
// solution does not have it.
export function pathTo(root: Node, path: string): string[] | null {
  const want = path.toLowerCase()
  const visit = (n: Node, above: string[]): string[] | null => {
    if (n.kind === 'file' && n.path?.toLowerCase() === want) return [...above, n.id]
    for (const c of n.children ?? []) {
      const found = visit(c, [...above, n.id])
      if (found) return found
    }
    return null
  }
  return visit(root, [])
}

// rows are the visible rows: open nodes' children, or with a filter, the
// files whose name has it and the nodes above them, all open.
export function rows(root: Node, open: Set<string>, filter: string): Row[] {
  const out: Row[] = []
  const f = filter.trim().toLowerCase()
  const keep = new Map<Node, boolean>()
  const matches = (n: Node): boolean => {
    if (keep.has(n)) return keep.get(n)!
    const self = n.kind === 'file' || n.kind === 'ref' ? n.label.toLowerCase().includes(f) : false
    const any = self || (n.children ?? []).some(matches)
    keep.set(n, any)
    return any
  }
  const visit = (n: Node, depth: number) => {
    if (f && !matches(n)) return
    const isOpen = f ? !!n.children?.length : open.has(n.id)
    out.push({ node: n, depth, open: isOpen })
    if (isOpen) for (const c of n.children ?? []) visit(c, depth + 1)
  }
  visit(root, 0)
  return out
}
