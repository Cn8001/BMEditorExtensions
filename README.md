# BMEditor Extensions

This repository contains extensions for [BMEditor](https://github.com/Cn8001/BMEditor-Releases),
a desktop code editor built on Wails, Vue and CodeMirror. BMEditor provides
Go support out of the box; support for other languages and tools is
delivered through extensions. BMEditor can be downloaded from the
[releases page](https://github.com/Cn8001/BMEditor-Releases/releases).

Each directory is a self-contained extension with its own manifest, build,
documentation and license. The extensions are maintained in a single
repository for convenience only and do not depend on one another.

## Extensions

| Extension | Description |
|---|---|
| [bmeditor-web](bmeditor-web) | JavaScript, TypeScript, CSS, SCSS, Less, HTML, Svelte, Vue and SVG, each with its language server, and an image viewer panel |
| [bmeditor-vim](bmeditor-vim) | Vim key bindings in every editor tab: modes, motions, registers, macros and the `:` command line |

Further details are provided in the README of each extension.

The [bmext](bmext) directory is the Go module
`github.com/Cn8001/BMEditorExtensions/bmext`: the extension format, the
package manager BMEditor installs extensions with, the backend SDK (`host`)
and the frontend kit (`frontend/`). See [bmext/README.md](bmext/README.md).

## Installation

1. In BMEditor, open Extensions (Ctrl+Shift+X).
2. Enter the install URL of the extension:

   | Extension | Install URL (latest version) |
   |---|---|
   | bmeditor-web | `https://github.com/Cn8001/BMEditorExtensions/releases/download/web/latest/web.zip` |
   | bmeditor-vim | `https://github.com/Cn8001/BMEditorExtensions/releases/download/vim/latest/vim.zip` |

3. Reload the window when prompted.

The extensions are distributed as prebuilt archives on the
[releases page](https://github.com/Cn8001/BMEditorExtensions/releases), so
installation requires neither Node.js nor Go. The URLs above always point
to the latest stable version, and Update in BMEditor fetches the newest
version from the same URL.

Every version remains available as a release of its own. A specific
version is installed from its release URL, which does not change:

```
https://github.com/Cn8001/BMEditorExtensions/releases/download/<name>/v<version>/<name>-<version>.zip
https://github.com/Cn8001/BMEditorExtensions/releases/download/vim/v0.1.0/vim-0.1.0.zip
```

The versions of one extension are listed by filtering the releases page by
its name, such as `vim/`. The notes of each release describe the changes
and the minimum BMEditor version.

## Releases

### Naming

| Item | Format | Example |
|---|---|---|
| Version release and tag | `<name>/v<version>` | `vim/v0.1.0` |
| Version archive | `<name>-<version>.zip` | `vim-0.1.0.zip` |
| Latest release and tag | `<name>/latest` | `vim/latest` |
| Latest archive | `<name>.zip` | `vim.zip` |

`<name>` is the `name` field of the extension's `bmext.json` (`web`, `vim`).

### Publishing a version

1. Increment `version` in the extension's `bmext.json`, following
   [semantic versioning](https://semver.org).
2. Add a section for the version to the extension's `CHANGELOG.md`, such as
   `## 0.2.0` or `## 0.2.0 (2026-11-04)`.
3. Update `engines.bmeditor` in `bmext.json` if the version needs a newer
   BMEditor.
4. Push to `master`.

The [Publish](.github/workflows/publish.yml) workflow then runs the Go
tests, builds each extension with `make zip` and runs
[scripts/publish.sh](scripts/publish.sh), which:

- skips the extension when its version is already released, so a push
  without a version change publishes nothing;
- fails when the version is not newer than every released version, or has
  no `CHANGELOG.md` section;
- creates the release `<name>/v<version>` with its archive, using the
  `CHANGELOG.md` section, the minimum BMEditor version and the install URLs
  as release notes;
- makes the version the latest by replacing the archive of the release
  `<name>/latest` ([scripts/set-latest.sh](scripts/set-latest.sh)).

Pull requests are tested and built but not published. The `build` entry is
removed from the packaged manifest, so BMEditor installs the archive as it
is.

### Rules

- A version release is created once and never modified or replaced.
- A prerelease version, such as `0.3.0-beta.1`, is published as a GitHub
  prerelease and does not become the latest, so users who follow the latest
  URL stay on the stable version.
- No release is marked as the repository's "Latest" on GitHub, since each
  extension has a latest release of its own.
- The "immutable releases" setting of the repository must remain disabled,
  because the archive of each `<name>/latest` release is replaced on every
  stable version.

### Rolling back

When a published version is broken, run the
[Rollback](.github/workflows/rollback.yml) workflow from the Actions tab
with the extension and an earlier stable version. It makes that version the
latest again ([scripts/rollback.sh](scripts/rollback.sh)), and users
following the latest URL receive it with their next Update. Version
releases are not modified. The fix is then published as a new, higher
version, which becomes the latest as usual.

### Minimum BMEditor version

`engines.bmeditor` in `bmext.json` (for example `">=0.1.0"`) states the
oldest BMEditor the version works with and is shown in its release notes.
BMEditor does not read this field at present; it is informational.

### Repository size

Build output is stored only as release assets, which do not count towards
the repository size; no binaries are committed to the repository.

## Building

Local builds are needed only for development. All extensions use the same
Makefile targets:

```
cd bmeditor-web
make dist    # builds the installable extension into dist/
make zip     # packages dist/ as <name>-<version>.zip
make clean   # removes dist/, node_modules/ and generated archives
```

`make dist` installs dependencies with `npm ci`, type checks the frontend
with `vue-tsc` and bundles it with Vite. For bmeditor-web, it additionally
cross compiles the `vue-ls` wrapper for Linux, macOS and Windows on amd64 and
arm64, and therefore requires Go in addition to Node.js.

The Go tests of bmeditor-web are run as follows:

```
cd bmeditor-web
go test ./...
```

A local build is installed by selecting its `dist` directory or zip file
with the folder button in the Extensions view.

## Extension Structure

Each extension follows the same layout:

| Path | Purpose |
|---|---|
| `bmext.json` | Manifest: name, version, minimum BMEditor version, languages, language servers, frontend script and build command |
| `CHANGELOG.md` | Changes of each version, used as release notes |
| `frontend/` | TypeScript and Vue sources that run in the editor window |
| `bmext/` | Extension kit copied from the `bmext` module at the root of this repository |
| `Makefile` | `dist`, `zip` and `clean` targets |
| `LICENSE` | License of the extension |

Languages, file icons, grammars and language server commands are declared in
the manifest. Frontend code is required only for functionality the manifest
cannot express, such as panels, commands or additional syntax highlighting.

The frontend is built as an IIFE whose default export is invoked once by
BMEditor at startup with the extension API. Vue and the CodeMirror packages
are not bundled; they are resolved from `window.bmeditor.shared` so that the
extension uses the editor's own instances. These packages are listed under
`devDependencies` for type information and the Vue compiler, and their
versions must match those used by the editor.

The files in `bmext/` (`vite.mjs` and `bmeditor.d.ts`) are copies of
`bmext/frontend/` at the root of this repository and must be updated in
every extension whenever the extension API changes. The manifest format, the
frontend API and packaging are documented in
[Writing extensions](https://github.com/Cn8001/BMEditor-Releases/blob/master/docs/extensions.md).

## Adding an Extension

1. Copy an existing extension directory. bmeditor-vim is the minimal example.
2. Update `name`, `displayName`, `version`, `description` and `engines.bmeditor` in `bmext.json`.
3. Update the `name` passed to `bmext()` in `vite.config.mjs`.
4. Update the package name in `package.json` and the archive name in the `Makefile`.
5. Replace `CHANGELOG.md` with a section for the first version.
6. Add the directory to the `matrix.dir` list in `.github/workflows/publish.yml`
   and to the `extension` options in `.github/workflows/rollback.yml`.

The `name` in `bmext.json` determines the release names, the tags and the
archive names, and must not change after the first release.

## License

All extensions in this repository are released under the MIT License. See
[LICENSE](LICENSE); each extension includes its own copy.
