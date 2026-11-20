# Mentor

A [BMEditor](https://github.com/Cn8001/BMEditor-Releases) extension for
keeping your hold on a project while a coding agent writes much of it: see
each step of its work before the next begins, and know what is there,
where it is and what uses what, without asking the agent.

- **Step by step, in auto mode too.** The agent plans in steps, each one
  behavior you can understand on its own, and says of each before it
  starts what it does, the code it uses, what it creates, what it leaves
  alone and when it is done; names the code does not have are flagged.
  The plan waits for you. Then the agent works on its own, and keeps to
  what it said: an edit where it said it would not go, or a step grown
  well past the files it named, ends the step.
- **A milestone tells you what was done, and shows the whole.** At the
  end of each step the agent stops and tells you, in its words, what it
  made, where it sits in the architecture, and what you need to know to
  carry on without it. Beside that, the project's shape: every package by
  layer, the ones it touched lit, what it added, and the dependencies it
  added or took away, the ones that cut across the layers said so. Mentor
  runs the tests itself. At the plan and at a milestone you can ask the
  agent a question, which it answers without changing anything, or tell it
  what to change; then Continue. The code is one click further, when you
  want it. The agent's hooks make the stops hold in any permission mode.
- **What the agent changed.** Below the checkpoints, the Changes tab
  lists the files the agent edited that still differ from HEAD, each with
  the declarations added, changed and removed in it. A file opens beside
  HEAD, a declaration opens where it is, and its card shows who uses it
  and its tests. The check marks a file reviewed; a later change to it
  shows again. A file leaves the list once it is committed or put back.
  Files the agent adds, removes or moves are listed too, as are those its
  commands change (rm, mv, gofmt -w): what differs from HEAD after a
  command and did not before. Your own edits are not listed, unless you
  make them while one of the agent's commands runs.
- **The agent's code, painted in the editor.** A declaration the agent
  added is green, one it changed is amber, from its first line to its
  last: open a file and see what the agent wrote, with nothing to click.
  One you mark reviewed keeps only its edge; the paint goes when the change
  is committed.
- **Review in the code.** Above each file the agent changed, a strip
  says what it changed there, its declarations a click away, with Diff,
  Reviewed and, while the agent waits at a checkpoint, Approve. **F7** goes
  to the agent's next change, file after file, **Shift+F7** to the one
  before, and **Alt+R** marks the file reviewed and goes to the next one,
  so a step is read without leaving the editor.
- **Three tabs.** Architecture: the packages by layer, what imports what,
  and the ways in. Features: a part of the project, or something to do to
  it. Changes: what the agent changed, its plan and checkpoints.
- **Understand a feature.** Name it, "authentication", and the Features
  tab follows it end to end through the code: where it starts (the route
  or main), what runs in order (through interfaces too, marked), where its
  data lives, how it fails, how to extend it, its tests and where to stop
  in the debugger. Every line is a place in the code, a click away.
- **Say what you want to do.** "Add a new authentication provider": the
  Features tab answers from what the code says for certain. The interface
  to implement and what it must have; the types that already do, how
  they differ, and the one to start from; how it is wired in, from the
  route down to the store; what to change and where, in order; the tests
  that reach it; and where to stop in the debugger when it breaks. Every
  line is a place in the code, a click away, and nothing is written by a
  model.
- **Ask in plain words.** "Which struct holds a user's login?", "where is
  the session made?": the Explore tab answers with the types and functions
  that fit, best first, each with what it is for, how much it is used and
  where, and what sets it apart from the look-alikes listed with it.
- **Explain This (Ctrl+Alt+U).** The card of the declaration at the
  cursor: its fields and methods, what it implements and what implements
  it, who makes it, who calls it (through an interface too), real code
  that uses it, the tests that reach it, and how far a change to it can
  reach.
- **Flows and the map.** From a function, the calls it makes in the order
  of its code, into the implementations an interface call may run, marked
  as decided at run time. The packages by the part of the service they
  seem to be, each with what it imports.
- **How it is used, in the code.** One line over each type and function
  says how many things use it and how many tests reach it. A click on it
  opens, right there under the line, the places in the project that show
  best how it is used, as they are written: a struct being built, a method
  called, a field read, from code before tests and from other packages
  first. A click on one goes there; Esc closes it. The ⓘ shows its card.
- **Where you want it.** Mentor opens in a window of its own that you
  move and size over the editor, as a tab of the bottom panel, or in the
  sidebar, as Open Mentor in says; its button is in the sidebar only when
  you choose the sidebar. The Mentor button in the top bar shows it, as
  does **Mentor: Open**, and its dot says whether the agent is connected:
  green when checkpoints hold. Mentor does not open by itself: when the
  agent waits for you, a notice says so and the button shows it.

Everything comes from the compiler's view of the code, read on your
machine and kept up to date as files change; nothing is guessed, and a
guess (a route found by the shape of a call, a package's layer by its
name) says so. The index lives in the editor's data folder, one SQLite
file per folder. With Search by meaning on, a local Ollama adds results
by meaning.

Languages: Go is read with its type checker, which needs Go on the PATH.
Other languages come through their language servers.

## Connecting the agent

Connect the agent, one On/Off button in Settings (and Turn off / Turn on
in Mentor), is all there is to it. On, Mentor adds to the open folder its
MCP server, through which the agent proposes its plan and opens
checkpoints, and its hooks, in the agent's local settings, which stay on
this machine; the dot beside Mentor in the top bar turns green. Off, it
takes both out, and the agent works without Mentor. An agent already
running picks the change up when it starts again.

Mentor says how far a stop holds: with the hooks, the agent waits; with
the MCP server alone, it follows the steps but nothing makes it wait; with
neither, Mentor shows what changed and does not claim the agent paused.
When the editor is closed, the hooks let the agent through.

## Developing

```
npm install
bmx dev     # build for this system and install it in BMEditor, then reload the window
bmx check   # install it from a clean clone, as BMEditor does from git
bmx pack    # mentor-<version>.zip, for every system, to publish
```

`go test ./...` runs the tests, on a small service in
`internal/golang/testdata/authsvc`. `MENTOR_REAL=/path/to/repo go test
./internal/engine -run TestRealFolder -v` reads a real repository and logs
what it found and how long it took.
