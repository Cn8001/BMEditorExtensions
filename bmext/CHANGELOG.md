# Changelog

Each version of bmext, newest first: the extension format, the package
manager, the SDKs and bmx. The version is in VERSION; its section here
becomes the notes of its release (scripts/release-bmext.sh), and a version
is released only when it has one.

## 0.2.6

- A menu row can carry a button at its end (`MenuItem.action`), such as
  delete, chosen by its own id.
- `explorerHide` leaves a file or folder out of the Explorer: one the
  project's tools generate, which nobody edits.
- `bmx check` keeps every system's programs when it installs, so it finds
  them all again (`Manager.AllSystems`).

Both SDK additions need BMEditor 0.1.5.

## 0.2.5

- Update installs only something newer: a zip asked for with its ETag
  that is unchanged, or holds the version installed, and a git ref at the
  commit installed, return `pkgmgr.ErrUpToDate` and leave the extension
  as it is.
- An install keeps only this system's programs (`bin/x-<os>-<arch>`), and
  from a URL downloads `<name>-<os>-<arch>.zip` beside `<name>.zip` when
  there is one. `bmx pack` writes one for each system as well, and the
  release scripts publish them.
- `bmext.DataDir`, `ExtensionsDir` and `ExtensionData`: the one local
  folder the editor keeps everything but settings in. The editor names it
  to every program it starts as `BMEDITOR_DATA`; an extension keeps its
  downloads in `$BMEDITOR_DATA/extension-data/<name>`. `bmx dev` installs
  there.
- Clone folders a terminal left behind are removed after a day.

## 0.2.4

- A setting's `options` are its only values, shown as a dropdown, and
  `when` shows it only while other settings have the values it names:
  `"when": {"provider": "ollama"}`. BMEditor 0.1.5 shows both.
- `api.saveSettings(values)` changes some of the extension's global
  settings and keeps the rest, for choosing one outside Settings.
- `api.components.DropMenu`, the top bar's menu, for a toolbar button
  that opens one.

## 0.2.3

- Remove moves the extension's folder aside before deleting it. A folder
  a running program holds (on Windows, one another BMEditor window runs
  from) is left whole, with an error that says so, instead of half
  deleted and broken. Replace says the same.
- Install and Remove clear what earlier ones left behind: folders moved
  aside and stages of installs that stopped half way.

## 0.2.2

- `editor?: (tab) => Extension` in `register`: CodeMirror support for
  every tab, whatever its language, such as inline completion. BMEditor
  0.1.5 applies it.

## 0.2.1

- `api.ui.output(text, stream?)` and `api.ui.showOutput()` write to and
  open the extension's channel in Output, where BMEditor 0.1.5 also puts
  its backend's stderr and the stderr of its language servers.
- `pkgmgr.Manager.Release` is called before an installed extension is
  replaced or removed, so the editor can stop its programs first: on
  Windows a running backend or language server kept Update and a new
  install from replacing the folder. Replacing also retries briefly.
- Installing an extension's source folder, one whose manifest has a build
  step, installs what the build wrote (`dist/`) rather than the source,
  which lacks the programs and the script. If it is not built yet, the
  install says to build it first.

## 0.2.0

- `bmx dev -target os/arch` builds for another system, with `-dir` to
  install there: BMEditor on Windows from WSL, say.
- `api.session.openFile(path, pos?)` is in `bmeditor.d.ts`: it opens a file
  in a tab, or switches to it, and goes to a position.

## 0.1.0

- The extension format (`bmext.json`), the package manager that installs
  extensions from a folder, a zip or a git repository, the backend SDK
  (`host`) and the frontend kit (`frontend/`).
- **bmx**, the tool for writing an extension: `bmx init` makes one that
  builds on its own, with a copy of the SDK; `bmx dev` builds it for your
  system and installs it in BMEditor; `bmx check` installs it from a clean
  clone of its files, as BMEditor does from git; `bmx pack` makes the zip
  to publish.
