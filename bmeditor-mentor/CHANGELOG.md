# Changelog

Each version of the Mentor extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.0.1

- First version. Claude Code, as the editor runs it, works on its own and
  stops at the end of each step of its plan: Mentor shows what the step
  changed beside what Claude says it did and why, and Claude goes on when
  you approve, or makes the changes you ask for. Its plan waits for you
  first. Claude Code's hooks make the stops hold in any permission mode,
  auto mode included; Connect Claude Code sets them up for the folder.
- The Changes tab: what differs from HEAD, declaration by declaration, in
  units that change together, each with where it is wired in, what it
  relies on, the routes and tests that reach it, how far it reaches and
  its risks. Reviewed marks a unit; a later change to it shows again.
- The Explore tab answers questions about the code in plain words with the
  types and functions that fit, and shows any symbol's card, the flow from
  a function and the package map. Explain This (Ctrl+Alt+U) shows the card
  of the declaration at the cursor, and a line above each declaration
  counts its uses. Semantic search with a local Ollama is optional.
- Needs BMEditor 0.1.6 and Go.
