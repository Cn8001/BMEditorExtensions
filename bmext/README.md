# bmext: bmeditor extensions

An extension is a folder with a `bmext.json` manifest at its root. Most of
what it adds is declared there; code is optional:

- a **frontend script** that runs in the editor's window and adds panels, a
  toolbar group, commands and keys;
- a **backend program** the editor starts on first use and talks to over
  stdin and stdout (JSON-RPC, as language servers do);
- **language servers**, on PATH (`clangd`) or carried by the extension;
- **debuggers**: debug adapters (`gdb -i dap`, `debugpy`, ...) that F5
  starts for the extension's languages.

An extension is its author's work, not bmeditor's: people install it at
their own risk, and bmeditor's author is not responsible for it or for
what it does (TERMS.md in the editor's repository, and About, Terms).

This module holds the format (`bmext`), the package manager (`pkgmgr`) that
installs extensions, the backend SDK (`host`) and the frontend kit
(`frontend/`). `examples/c` is a whole extension in one manifest.

## Writing one with bmx

`bmx` (`cmd/bmx`) makes and builds an extension. Install it with
`go install github.com/Cn8001/BMEditorExtensions/bmext/cmd/bmx@latest`,
download it for your system from the
[latest release](https://github.com/Cn8001/BMEditorExtensions/releases/latest)
(`bmx-linux-amd64`, `bmx-windows-amd64.exe`, ...), or build it from a
clone (`go -C bmext install ./cmd/bmx`). Then, in an empty folder:

```
bmx init [-backend]   # bmext.json, a Makefile, frontend/index.ts (and cmd/<name>-host), a copy of the SDK in bmext/
npm install
bmx dev               # build for this system only and install it in the editor; then reload the window
bmx check             # install it from a clean git clone of its files, as the editor does, and check every file the manifest names is there
bmx pack              # build for every system: <name>-<version>.zip, to publish
```

The extension builds on its own, from its own repository: `bmext/` is a
copy of this module's `frontend/` and `host/`, not a reference to it.
`bmx init` in an existing extension updates that copy and leaves the
other files alone; `bmx check` says when the copy is out of date.

## Installing

In the editor: Extensions (Ctrl+Shift+X), then a source in the box:

| Source | Example |
| --- | --- |
| git, any server, https or ssh | `github.com/me/ext`, `git@gitlab.com:team/ext.git`, `https://git.example.org/ext.git` |
| a tag or branch | `github.com/me/ext#v1.2.0` |
| a folder | `/home/me/src/ext/dist` |
| a .zip, file or URL | `https://example.org/ext-1.2.0.zip` |

In the editor, git clone runs in a terminal inside the Extensions view, as
in a shell: for a private repository git asks there for a username and
password or token, an ssh key's passphrase, or whether to trust a host, and
uses the credentials already set up first. (`pkgmgr.Install` called on its
own runs git without a terminal and asks nothing.) Each install
records its source, so Update fetches the same one again. Extensions live
in the config folder: `~/.config/bmeditor/extensions/<name>` on Linux,
`%AppData%\bmeditor\extensions` on Windows. A change takes effect when the
window reloads.

A repository whose manifest has `build` is built after cloning (`make dist`,
say) and the folder named by `build.output` is installed. Without `build`,
the repository itself is the extension.

## The manifest

```json
{
  "name": "c",
  "displayName": "C",
  "version": "0.1.0",
  "description": "C and its headers.",
  "icon": "icon.svg",
  "languages": [
    {
      "id": "c",
      "label": "C",
      "extensions": [".c", ".h"],
      "fileNames": [],
      "icon": "c.svg",
      "codicon": "symbol-structure",
      "color": "#c3cf9b",
      "lineComment": "//",
      "blockComment": ["/*", "*/"],
      "grammar": "c",
      "server": { "command": ["clangd"], "install": "apt install clangd", "options": {} }
    }
  ],
  "settings": [
    { "key": "apiKey", "label": "API key", "type": "secret" },
    { "key": "baseUrl", "label": "Base URL", "scope": "both", "default": "http://localhost:8090" }
  ],
  "projectKinds": [
    {
      "kind": "c",
      "label": "C",
      "title": "C program",
      "text": "A main.c and a Makefile.",
      "folder": "hello",
      "detect": ["*.c"],
      "template": "templates/c",
      "open": ["main.c"],
      "settings": {}
    }
  ],
  "debuggers": [
    {
      "id": "gdb",
      "label": "GDB",
      "languages": ["c"],
      "command": ["gdb", "-i", "dap"],
      "transport": "stdio",
      "install": "gdb 14 or newer",
      "prepare": ["cc", "-g", "-O0", "-o", "${fileDirname}/${fileBasenameNoExtension}", "${file}"],
      "cleanup": ["${fileDirname}/${fileBasenameNoExtension}"],
      "launch": { "program": "${fileDirname}/${fileBasenameNoExtension}", "cwd": "${fileDirname}" }
    }
  ],
  "frontend": { "script": "frontend/index.js", "style": "frontend/style.css" },
  "backend": { "command": "bin/my-host" },
  "build": { "command": ["make", "dist"], "output": "dist" }
}
```

- `name`: lower-case letters, digits and dashes; the install folder's name.
- `languages[].grammar`: one the editor carries: c, cpp, csharp, java,
  kotlin, scala, dart, objectivec, rust, python, javascript, typescript,
  lua, ruby, swift, cmake, shell, yaml, toml, sql. A script can bring its own.
- `languages[].server.command`: a program on PATH, or `./bin/name` inside
  the extension (found as `bin/name-GOOS-GOARCH[.exe]`, then `bin/name`).
  The editor's LSP client does the rest: completion, hover, go to
  definition, usages, rename, diagnostics, quick fixes, inlay hints.
- `debuggers`: a Debug Adapter Protocol program. `transport` is stdio (the
  default) or tcp, where `{port}` in the command becomes a free port the
  editor connects to. `prepare` runs before each session (to build);
  `launch` is the launch request's arguments. Both have `${workspaceFolder}`,
  `${file}`, `${fileDirname}`, `${fileBasename}` and
  `${fileBasenameNoExtension}` filled in, so prepare can build the very
  file launch debugs; launch also gets the project's run arguments and
  environment. `cleanup` lists files to remove when the session ends,
  what prepare built; only a file prepare made is removed, so one that was
  there before stays. Breakpoints, stepping, inline values and the Run
  and Debug view then work as they do for Go.
- `settings`: `type` is string, secret, boolean or path; `scope` is global
  (Settings, Ctrl+,), project (Project Settings) or both, the project's
  value winning. Secrets stay in Go: the backend gets them, the window
  never does.
- `projectKinds[].template`: a folder copied into a new project; `{{name}}`
  in a file becomes the project's name. `settings` are the new project's.
- Every path stays inside the extension; a manifest that points outside it
  is refused.

## Backend

A Go program built with `host`:

```go
s := host.New()
s.Handle("compile", func(ctx context.Context, p json.RawMessage) (any, error) {
	schema := s.Context().Setting("schemaPath") // the project's, else the global one
	...
	return result, nil // or &host.Error{Code: "BAD_QUERY", Message: ..., Line: 3, Col: 7}
})
host.Run(s)
```

The editor starts `backend.command` (with `-GOOS-GOARCH` first, as for
servers), sends `initialize` and then `context` whenever the folder, the
project or the settings change. A call the window cancels cancels the
handler's ctx. `s.Notify(method, params)` reaches the script's
`api.on(method, ...)`. Any language works that speaks the same framing.

## Frontend

The script is an IIFE whose default export the editor calls with its API
(`frontend/bmeditor.d.ts`). Build it with `frontend/vite.mjs`, which leaves
Vue and CodeMirror to the editor's own copies:

```js
// vite.config.mjs
import vue from '@vitejs/plugin-vue'
import { bmext } from './bmext/vite.mjs' // copied from this module
export default { plugins: [vue()], ...bmext({ name: 'my-ext' }) }
```

```ts
// frontend/index.ts
import type { ExtensionAPI } from '../bmext/bmeditor'
export default function activate(api: ExtensionAPI) {
  api.register({
    panels: [{ id: 'my-panel', label: 'Mine', component: MyPanel }],
    commands: [{ id: 'my-run', label: 'Mine: Run', run: () => api.call('run') }],
  })
}
```

Keep the editor's versions of vue and @codemirror/* in devDependencies, for
types and the Vue compiler.
