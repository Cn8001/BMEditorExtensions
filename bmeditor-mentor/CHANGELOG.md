# Changelog

Each version of the Mentor extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.0.1

- First version. The coding agent the editor runs works on its own and
  stops at the end of each step of its plan: Mentor shows what the step
  changed beside what the agent says it did and why, and the agent goes on
  when you approve, or makes the changes you ask for. Its plan waits for
  you first. The agent's hooks make the stops hold in any permission mode,
  auto mode included. Mentor connects the agent while the setting Connect
  the agent is on, adding its MCP server and hooks, and takes them out
  when it is off.
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
