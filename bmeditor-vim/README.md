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

## Your vimrc

The mappings in your vimrc work here too. Without a setting it is the one
Vim reads: `~/.vimrc`, `~/_vimrc` or `~/.vim/vimrc`, the first that is
there. To use another, set **vimrc** in Settings (Ctrl+,), or in Project
Settings for one project, where a relative path is in the project.

What is applied:

| Line | Example |
|---|---|
| `map`, `nmap`, `vmap`, `xmap`, `omap`, `imap`, `map!` | `nmap <leader>q :q<CR>` |
| the `noremap` forms of each | `inoremap jk <Esc>` |
| `unmap` and its forms, for a mapping the vimrc made | `nunmap Y` |
| `let mapleader`, `let maplocalleader` | `let mapleader = " "` |
| `set`, for the options this Vim mode has | `set langmap=...`, `set tw=100` |

The rest of a vimrc is Vimscript that means nothing in an editor tab
(plugins, `autocmd`, `syntax`, functions, highlighting) and is skipped
without a word, so a vimrc made for Vim works as it is. So is a mapping
inside `if`, `function` or `augroup`, which runs only under conditions
BMEditor cannot check, and one with `<expr>`, `<Plug>`, `<SID>` or `<Cmd>`.

The vimrc is read when BMEditor starts, when a project opens and when
Settings close. After editing it, run **Vim: Reload vimrc** from the
command palette, which also says how many lines took.

## On and off

**Vim: Toggle Vim Mode** in the command palette turns it off and back on,
in every tab at once. BMEditor remembers the choice.

## Keys BMEditor keeps

A few Ctrl keys are BMEditor's before they reach Vim: Ctrl+B (sidebar),
Ctrl+O (open folder), Ctrl+P (go to file), Ctrl+W (close tab), Ctrl+J
(bottom panel) and Ctrl+G (go to line). Ctrl+D, Ctrl+U, Ctrl+R, Ctrl+V and
the rest are Vim's. A vimrc mapping on one of BMEditor's keys never runs.

## Install

In BMEditor, open Extensions (Ctrl+Shift+X), enter

```
https://github.com/Cn8001/BMEditorExtensions/releases/download/vim/latest/vim.zip
```

and reload the window when asked. Earlier versions are on the
[releases page](https://github.com/Cn8001/BMEditorExtensions/releases).

To build it yourself, run `make dist` (Node and Go needed) and pick `dist` with
the folder button in Extensions.

## License

MIT. See [LICENSE](LICENSE).
