# PowerShell and cmd

PowerShell (`.ps1`, `.psm1`, `.psd1`) and cmd batch files (`.bat`, `.cmd`)
in BMEditor.

## What it gives

### PowerShell

- **Language server**: PowerShell Editor Services, the one VS Code's
  PowerShell extension runs. You get completion of commands, parameters,
  variables and .NET members; hover with help; go to definition and
  references; rename; the outline; formatting; and PSScriptAnalyzer's
  warnings in Problems.
- **Debugging**: F5 runs the open script under Editor Services' debugger,
  with breakpoints, steps, variables and the call stack. What the script
  prints is in the Debug Console.
- **Snippets**: type `function`, `function-advanced`, `param`, `foreach`,
  `try`, `class`, `pscustomobject`, `splat`, `region`, `comment-help`,
  `describe` (Pester) and more where a statement starts, and take the
  snippet with Tab.
- Highlighting, and `#` and `<# #>` for Toggle Comment.

### cmd

- **Language server** of its own (`bat-ls`):
  - **Completion**: commands, each with its syntax and what it does; labels
    after `goto` and `call :`; variables after `%` or `!`, the file's own
    first, then Windows' (`%USERPROFILE%`...) and the arguments
    (`%~dp0`, `%1`, `%*`).
  - **Hover**: a command's syntax, where a label is (with the comment above
    it), where a variable is set.
  - **Labels**: go to a label from its `goto` or `call`, find its
    references, rename it everywhere, and see the labels as the outline.
  - **Errors**: a `goto` or `call` to a label that is not in the file is an
    error, and a label written twice is a warning.
- **F5 runs** the batch file with cmd. Its output, in UTF-8, goes to the
  Debug Console, and Stop ends it with everything it started. cmd has no
  debugger, so there are no breakpoints.
- **Snippets**: `header`, `sub`, `ifexist`, `iferror`, `for`, `forf`,
  `choice`, `admin`, `wait` and more.
- Highlighting, and `REM` for Toggle Comment.

## Setting up

PowerShell's language server and debugger need PowerShell Editor Services.
Run **PowerShell: Install Editor Services** from the command palette
(Ctrl+Shift+P). It downloads the latest release from
[GitHub](https://github.com/PowerShell/PowerShellEditorServices/releases)
into the extension's folder in your cache (`%LOCALAPPDATA%\bmeditor-powershell`
on Windows). Then open a PowerShell file again. Running the command again
updates it.

It also uses one that is there already:

- the folder `BMEDITOR_PSES` names;
- the one in VS Code's PowerShell extension.

**PowerShell: Show Editor Services** says which one is used.

Editor Services runs in PowerShell 7 (`pwsh`) when it is installed, else
in Windows PowerShell 5.1. `BMEDITOR_POWERSHELL` names another one.

cmd needs nothing.

## Building

```
npm install
bmx dev      # build for this system and install it in BMEditor
bmx check    # install it from a clean clone, as BMEditor does from git
bmx pack     # powershell-<version>.zip for every system
```

From WSL, for BMEditor on Windows:
`bmx dev -target windows/amd64 -dir /mnt/c/Users/<you>/AppData/Roaming/bmeditor/extensions`.
