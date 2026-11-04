# Vim for BMEditor

Vim keys in every editor tab, Go and every other language alike: normal,
insert and visual modes (with block visual on Ctrl+V), motions and text
objects, counts, registers, marks, macros, `/` search, `.` repeat, and the
`:` command line, with the mode shown at the bottom of the editor. It is
the Vim mode of CodeMirror, the editor BMEditor is built on.

## The command line

| Command | Does |
|---|---|
| `:w` | save |
| `:q` | close the tab |
| `:wq`, `:x` | save and close |
| `:s/old/new/g`, `:%s/...` | replace |
| `:noh` | clear the search highlight |

## On and off

**Vim: Toggle Vim Mode** in the command palette turns it off and back on,
in every tab at once. BMEditor remembers the choice.

## Keys BMEditor keeps

A few Ctrl keys are BMEditor's before they reach Vim: Ctrl+B (sidebar),
Ctrl+O (open folder), Ctrl+P (go to file), Ctrl+W (close tab), Ctrl+J
(bottom panel) and Ctrl+G (go to line). Ctrl+D, Ctrl+U, Ctrl+R, Ctrl+V and
the rest are Vim's.

## Install

In BMEditor, open Extensions (Ctrl+Shift+X), enter

```
https://github.com/Cn8001/BMEditorExtensions/releases/download/vim/latest/vim.zip
```

and reload the window when asked. This URL always points to the latest
stable version, and Update fetches the newest one from it. Nothing is built
on your machine.

Every version stays available. A fixed version is installed from its own
release, `https://github.com/Cn8001/BMEditorExtensions/releases/download/vim/v<version>/vim-<version>.zip`; the versions are listed on
the [releases page](https://github.com/Cn8001/BMEditorExtensions/releases)
under `vim/`, and [CHANGELOG.md](CHANGELOG.md) describes each one.

To install a local build instead, run `make dist` (Node needed), click the
folder button in Extensions and pick `dist`, or run `make zip` and pick the
zip.

## License

MIT. See [LICENSE](LICENSE).
