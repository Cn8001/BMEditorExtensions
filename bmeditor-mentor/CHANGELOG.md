# Changelog

Each version of the Mentor extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.1.1

- The Changes tab: what differs from HEAD, declaration by declaration
  (added, changed, removed), in units that change together, each with
  where it is wired in, what it relies on, the routes and tests that reach
  it, how far it reaches, and its risks: a changed signature others use, a
  changed interface, something removed, code no test reaches. Diff shows a
  file beside HEAD; Reviewed marks a unit, and a later change to it shows
  again. Mentor: What Changed opens it.

## 0.1.0

- First version: the Mentor view answers questions about a Go project in
  plain words with the types and functions that fit, and shows any
  symbol's card, the flow from a function and the map of the packages, all
  from the type checker. Explain This (Ctrl+Alt+U) shows the card of the
  declaration at the cursor, and a line above each declaration counts its
  uses. Semantic search with a local Ollama is optional. Needs BMEditor
  0.1.6.
