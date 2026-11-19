import { reactive } from 'vue'
import type { Contributions, Problem, ProjectToolbar } from '../bmext/bmeditor'
import { api, basename, call, fail, log, store } from './api'

// Run, Build and Test of a .NET solution, by dotnet in the backend: in the
// command palette, and in the top bar's Run and Debug group from BMEditor
// 0.1.6. Unity builds its own projects, so a Unity project or package keeps
// the editor's group.

type Verb = 'run' | 'build' | 'test'

export const dotnet = reactive({ running: '' as '' | Verb, stopping: false })

const problemKey = 'csharp:dotnet'
// MSBuild repeats each message in its summary, so they are kept by place.
const problems = new Map<string, Problem>()

function clearProblems() {
  problems.clear()
  api.ui.setProblems(problemKey, [])
}

const dotnetProject = () => store.workspace?.kind === 'folder' && store.solutions.length > 0
const idle = () => !dotnet.running && dotnetProject()

async function start(verb: Verb) {
  clearProblems()
  api.ui.showOutput?.()
  try {
    await call('dotnet.' + verb, { path: store.solutionPath })
  } catch (err) {
    fail(`Cannot ${verb} ${basename(store.solutionPath) || 'the solution'}`, err)
  }
}

async function stop() {
  dotnet.stopping = true
  await call('dotnet.stop').catch(() => {})
}

const ended: Record<Verb, { failed: string; passed?: string }> = {
  build: { failed: 'The build failed', passed: 'Build succeeded' },
  test: { failed: 'Tests failed', passed: 'Tests passed' },
  run: { failed: 'The program stopped with an error' },
}

export function listenDotnet() {
  api.on('dotnet/output', (l: { stream: 'stdout' | 'stderr'; text: string }) => log(l.text, l.stream === 'stderr' ? 'stderr' : 'info'))
  api.on('dotnet/problem', (p: Problem) => {
    problems.set(`${p.file}:${p.line}:${p.col}:${p.message}`, p)
    api.ui.setProblems(problemKey, [...problems.values()])
  })
  api.on('dotnet/started', (s: { what: Verb }) => {
    dotnet.running = s.what
    log(`> dotnet ${s.what} ${basename(store.solutionPath)}`)
  })
  api.on('dotnet/exit', (e: { what: Verb; error: string }) => {
    const stopped = dotnet.stopping
    dotnet.running = ''
    dotnet.stopping = false
    if (stopped) return
    const note = ended[e.what]
    if (e.error) api.ui.toast('error', note.failed, problems.size ? 'See Problems and Output.' : 'See Output.')
    else if (note.passed) api.ui.toast('info', note.passed)
  })
}

export function resetDotnet() {
  clearProblems()
}

export function dotnetToolbar(): ProjectToolbar {
  const target = () => basename(store.solutionPath)
  return {
    when: dotnetProject,
    label: 'C#',
    logo: api.base + 'icon.svg',
    run: { label: 'Run', icon: 'play', title: () => `dotnet run ${target()} (Ctrl+F5)`, run: () => start('run'), enabled: idle },
    build: { label: 'Build', icon: 'package', title: () => `dotnet build ${target()} (Ctrl+Shift+B)`, run: () => start('build'), enabled: idle },
    test: { label: 'Test', icon: 'beaker', title: () => `dotnet test ${target()}`, run: () => start('test'), enabled: idle },
    menu: [{ label: 'Show Output', icon: 'output', run: () => api.ui.showOutput?.() }],
    stop: { run: stop, enabled: () => !!dotnet.running },
  }
}

// The palette's, for an editor whose top bar has no group for extensions.
export function dotnetCommands(): NonNullable<Contributions['commands']> {
  return [
    { id: 'csharp-dotnet-run', label: 'C#: Run (dotnet run)', icon: 'play', when: idle, run: () => start('run') },
    { id: 'csharp-dotnet-build', label: 'C#: Build (dotnet build)', icon: 'package', when: idle, run: () => start('build') },
    { id: 'csharp-dotnet-test', label: 'C#: Test (dotnet test)', icon: 'beaker', when: idle, run: () => start('test') },
    { id: 'csharp-dotnet-stop', label: 'C#: Stop', icon: 'debug-stop', when: () => !!dotnet.running, run: stop },
  ]
}
