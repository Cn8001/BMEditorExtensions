# Changelog

Each version of the Vim extension, newest first. The section of a version
becomes the notes of its release, and a version is published only when it
has a section here.

## 0.2.0

- Your vimrc: its mappings, `unmap`s, `mapleader` and the options this Vim
  mode has are applied in every tab, and the rest of it is skipped. It is
  `~/.vimrc` (or `~/_vimrc`, `~/.vim/vimrc`) unless the new **vimrc**
  setting names another, for the editor or for a project.
- **Vim: Reload vimrc** reads it again after an edit.

## 0.1.0

First published version.

- Vim key bindings in every editor tab: normal, insert and visual modes,
  block visual mode, motions, text objects, counts, registers, marks,
  macros, search and `.` repeat.
- The `:` command line with `:w`, `:q`, `:wq`, `:x`, `:s` and `:noh`.
- **Vim: Toggle Vim Mode** turns Vim mode off and on in every tab; the
  choice is remembered.
