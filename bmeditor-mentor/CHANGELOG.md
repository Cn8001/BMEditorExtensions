# Changelog

Each version of the Mentor extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.0.1

- First version. The coding agent the editor runs works on its own, in
  steps of one behavior each, saying before each what it does, uses,
  creates and leaves alone and when it is done; a step that strays or
  grows ends. At the end of each it stops: Mentor shows what was made,
  where it fits in the project and what to know of it, runs its tests, and
  the agent goes on when you approve, or makes the changes you ask for.
  Its plan waits for you first. The agent's hooks make the stops hold in any permission mode,
  auto mode included. One button, Connect the agent, turns it all on or
  off: on, Mentor adds its MCP server and hooks for the folder and its dot
  turns green; off, it takes them out.
- Three tabs: Architecture, Features and Changes. Name a part of the
  project in Features, such as authentication, and it is followed end to
  end: where it starts, what runs in order, where its data lives, how it
  fails, how to extend it, its tests and where to stop.
- Say what you want to do, such as "add a new authentication provider",
  and the Features tab answers from the code: the interface to implement,
  the types that already do and how they differ, how it is wired in, what
  to change and where, the tests, and where to stop when it breaks.
- How it is used, in the code: a click on the line above a type or a
  function shows, under it, the places that show best how the project
  uses it, as they are written, a click away each.
- The agent's code is painted in the editor, green where it added a
  declaration and amber where it changed one.
- Review in the code: a strip above each file the agent changed, with
  what it changed there, Diff, Reviewed and the checkpoint's Approve; F7
  and Shift+F7 go from one of its changes to the next, Alt+R marks a file
  reviewed and goes on. Ctrl+Alt+U (Explain This) now works.
- The Changes tab lists what the agent changed that still differs from
  HEAD: the files it edited, added or removed, by its tools or its
  commands, each with the declarations added, changed and removed in it. The check marks a file reviewed; a later change to it
  shows again.
- The Explore tab answers questions about the code in plain words with the
  types and functions that fit, and shows any symbol's card, the flow from
  a function and the package map. Explain This (Ctrl+Alt+U) shows the card
  of the declaration at the cursor, and a line above each declaration
  counts its uses. Search by meaning with a local Ollama is optional.
- Mentor opens in a floating window, the bottom panel or the sidebar, as
  the setting Open Mentor in says, and does not open by itself; its
  button in the top bar shows it, with a dot that says whether the agent
  is connected.
- Reads Go code, with Go on the PATH. Needs BMEditor 0.1.6.
