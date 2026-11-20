# Mentor

A [BMEditor](https://github.com/Cn8001/BMEditor-Releases) extension for
keeping your hold on a Go project while a coding agent writes much of it:
know what is there, where it is, and what uses what, without asking the
agent.

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
