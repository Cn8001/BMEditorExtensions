# Mentor

A [BMEditor](https://github.com/Cn8001/BMEditor-Releases) extension for
keeping your hold on a project while a coding agent writes much of it: see
each step of its work before the next begins, and know what is there,
where it is and what uses what, without asking the agent.

- **Step by step, in auto mode too.** The agent proposes its plan in
  steps, and waits for you to approve it in Mentor. It then works on its
  own; at the end of each step it stops at a checkpoint. Mentor shows what
  the agent says it did and why, beside what the code shows changed: the
  declarations added, changed and removed, where they are wired in, the
  routes and tests that reach them, and what to look at first. Names it
  uses that the code does not have are flagged. Approve, and the agent
  goes on by itself; ask for changes, and it makes them within the step;
  or skip. The agent's hooks make the stops hold in any permission mode:
  while you have not answered, it changes nothing, and it cannot end its
  turn with a step that has no checkpoint. A step that grows past the
  files a step may change ends early. What the agent did, edit by edit, is
  listed as it happens.
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
- **Uses above the code.** One line over each type and function: how many
  things use it, how many tests reach it. A click shows its card.
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

Mentor connects the agent while the settings Pause the agent at each
step and Connect the agent are both on: for the open folder it adds its
MCP server, through which the agent proposes its plan and opens
checkpoints, and its hooks to the agent's local settings, which stay on
this machine. Turn either off and Mentor takes both out. An agent already
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
