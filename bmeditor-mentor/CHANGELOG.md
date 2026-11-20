# Changelog

Each version of the Mentor extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.0.1

- First version. The coding agent proposes its plan in steps of one
  behavior each, saying what each does, uses, creates and leaves alone and
  when it is done, and waits for you to approve it. It then works on its
  own; a step that strays or grows ends. At the end of each step, a
  milestone, it stops and tells you what it made, where it sits in the
  architecture and what to know to carry on without it, beside the
  project's shape with its work on it: the packages it touched, the parts
  it added, the dependencies that came or went. Mentor runs the tests. Ask
  it a question, which it answers without changing anything, or tell it
  what to change; then Continue. The agent's hooks make the stops hold in
  any permission mode. One button, Connect the agent, turns it all on or
  off.
- One view: the agent's work, its plan or milestone and the project's
  shape, with the files it changed folded below; and one box to ask about
  the project. Name a part of it, such as authentication, and it is
  followed end to end: where it starts, what runs in order, where its data
  lives, how it fails, its tests and where to stop. Say what you want to
  do, such as "add a new authentication provider", and the answer adds the
  interface to implement, the types that already do and what to change.
  Or ask which type or function fits, and open its card, its flow or the
  package map.
- In the code: the agent's declarations painted, green where it added one
  and amber where it changed one; a strip above each file it changed, F7
  to its next change and Alt+R to mark a file reviewed; a line above each
  declaration counting its uses, which opens how the project uses it; and
  Explain This (Ctrl+Alt+U).
- Mentor opens in a floating window, the bottom panel or the sidebar; its
  button in the top bar shows it, with a dot that says whether the agent
  is connected.
- Reads Go code, with Go on the PATH. Needs BMEditor 0.1.6.
