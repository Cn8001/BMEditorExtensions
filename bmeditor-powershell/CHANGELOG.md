# Changelog

Each version of the PowerShell extension, newest first. The section of a
version becomes the notes of its release, and a version is published only
when it has a section here.

## 0.1.1

- Editor Services and its logs go in BMEditor's data folder
  (`extension-data/powershell`) rather than the user's cache; the logs of
  runs older than three days are removed.

## 0.1.0

- PowerShell with PowerShell Editor Services: completion, hover, go to
  definition, references, rename, formatting and PSScriptAnalyzer's
  warnings. PowerShell: Install Editor Services downloads it, or it uses
  VS Code's.
- Debugging PowerShell scripts with F5.
- cmd batch files (`.bat`, `.cmd`) with a language server of their own:
  completion of commands, labels and variables, hover with each command's
  syntax, go to a label, its references and rename, labels as the outline,
  and errors for a `goto` or `call` to a missing label.
- F5 runs a batch file, its output in the Debug Console.
- Snippets for PowerShell and cmd.
