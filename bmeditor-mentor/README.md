# Mentor

A [BMEditor](https://github.com/Cn8001/BMEditor-Releases) extension for
keeping your hold on a Go project while Claude Code writes much of it:
see each step of its work before the next begins, and know what is there,
where it is and what uses what, without asking the agent.

- **Step by step, in auto mode too.** Claude Code proposes its plan in
  steps, and waits for you to approve it in Mentor. It then works on its
  own; at the end of each step it stops at a checkpoint. Mentor shows what
  Claude says it did and why, beside what the code shows changed: the
  declarations added, changed and removed, where they are wired in, the
  routes and tests that reach them, and what to look at first. Names it
  uses that the code does not have are flagged. Approve, and Claude goes
  on by itself; ask for changes, and it makes them within the step; or
  skip. Claude Code's hooks make the stops hold in any permission mode:
  while you have not answered, it changes nothing, and it cannot end its
  turn with a step that has no checkpoint. A step that grows past the
  files a step may change ends early. What Claude did, edit by edit, is
  listed as it happens.
- **What changed.** Below the checkpoints, the Changes tab reads what
  differs from HEAD (an agent's work, or yours) declaration by declaration: what was added,
  changed or removed, grouped into the units that change together. Each
  says where it is wired into the code that did not change, what it relies
  on, which routes and tests reach it, how far it can reach, and what to
  look at first: a changed signature others use, an interface its
  implementations must follow, something removed, code no test reaches.
  Diff opens a file beside HEAD; Reviewed marks a unit as looked at, and a
  later change to it shows again.
- **Ask in plain words.** "Which struct holds a user's login?", "where is
  the session made?": the Mentor view in the sidebar answers with the types
  and functions that fit, best first, each with what it is for, how much it
  is used and where, and what sets it apart from the look-alikes listed
  with it (the fields only it has).
- **Explain This (Ctrl+Alt+U).** The card of the declaration at the cursor:
  its fields and methods, what it implements and what implements it, who
  makes it, who calls it (through an interface too), real code that uses
  it, the tests that reach it, and how far a change to it can reach.
- **Flows.** From a function, the calls it makes in the order of its code,
  down through the layers; a call through an interface opens into the
  implementations that may run, marked as decided at run time.
- **The map.** The packages by the part of the service they seem to be
  (API, service, repository, ...), each with what it imports, and inside a
  package its types and functions with what they use and what uses them.
- **Uses above the code.** One line over each type and function: how many
  things use it, how many tests reach it. A click shows its card.

Everything comes from the type checker (`go/packages`), read on your
machine and kept up to date as files change; nothing is guessed, and a
guess (a route found by the shape of a call, a package's layer by its name)
says so. The index lives in the editor's data folder, one SQLite file per
folder. With Semantic search on, a local Ollama adds results by meaning.

It needs Go on the PATH, as the editor's Go support does.

## Setting up Claude Code

Run **Mentor: Connect Claude Code** (or Connect in the Changes tab). It
adds Mentor's hooks to the folder's `.claude/settings.local.json`, which
stays on this machine, and Mentor's MCP server to Claude Code for this
folder (`claude mcp add --scope local`). Start Claude Code again in the
folder. Disconnect takes both out.

Mentor says how far a stop holds: with the hooks, Claude waits; with the
MCP server alone, Claude follows the steps but nothing makes it wait;
with neither, Mentor shows what changed and does not claim Claude paused.
When the editor is closed, the hooks let Claude through.

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
