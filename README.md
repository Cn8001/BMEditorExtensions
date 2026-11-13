# BMEditor Extensions

Extensions for [BMEditor](https://github.com/Cn8001/BMEditor-Releases), a
desktop code editor. BMEditor can be downloaded from its
[releases page](https://github.com/Cn8001/BMEditor-Releases/releases).

## Extensions

| Extension | Description | Install URL |
|---|---|---|
| [bmeditor-web](bmeditor-web) | JavaScript, TypeScript, CSS, SCSS, Less, HTML, Svelte, Vue and SVG, each with its language server, and an image viewer panel | `https://github.com/Cn8001/BMEditorExtensions/releases/download/web/latest/web.zip` |
| [bmeditor-vim](bmeditor-vim) | Vim key bindings in every editor tab | `https://github.com/Cn8001/BMEditorExtensions/releases/download/vim/latest/vim.zip` |
| [bmeditor-csharp](bmeditor-csharp) | C# for Unity and UPM packages: language server, Solution Explorer, assembly graph, Inspector for .asmdef and package.json, Unity compile errors, .meta and GUID tools | `https://github.com/Cn8001/BMEditorExtensions/releases/download/csharp/latest/csharp.zip` |
| [bmeditor-powershell](bmeditor-powershell) | PowerShell with PowerShell Editor Services (completion, PSScriptAnalyzer, debugging) and cmd batch files with a language server of their own and F5 to run them | `https://github.com/Cn8001/BMEditorExtensions/releases/download/powershell/latest/powershell.zip` |
| [bmeditor-autocomplete](bmeditor-autocomplete) | Inline code suggestions in every language from a small model on your machine (llama.cpp with Qwen2.5-Coder, or Ollama) | `https://github.com/Cn8001/BMEditorExtensions/releases/download/autocomplete/latest/autocomplete.zip` |
| [bmeditor-hex](bmeditor-hex) | Files that are not text, such as executables, in hex: offsets, sixteen bytes a row, the bytes as characters, what a selection reads as, and editing in place | `https://github.com/Cn8001/BMEditorExtensions/releases/download/hex/latest/hex.zip` |
| [bmeditor-ascdii](bmeditor-ascdii) | Games made with the [ASCDII Engine](https://github.com/Cn8001/ASCDII-Engine): a Game view of the project's assets, Lua with the engine's API and Code Vision, Project Settings, a Console, and Play in the game's window or the engine's editor | `https://github.com/Cn8001/BMEditorExtensions/releases/download/ascdii/latest/ascdii.zip` |

The [bmext](bmext) directory holds the extension format and SDK. See
[bmext/README.md](bmext/README.md) or
[Writing extensions](https://github.com/Cn8001/BMEditor-Releases/blob/master/docs/extensions.md).

## Installation

1. In BMEditor, open Extensions (Ctrl+Shift+X).
2. Enter the extension's install URL from the table above.
3. Reload the window when prompted.

These URLs always point to the latest version, and Update in BMEditor keeps
the extension up to date. Earlier versions are available on the
[releases page](https://github.com/Cn8001/BMEditorExtensions/releases), each
with its own install URL and release notes.

## Building

Building from source is needed only for development. Install `bmx`, the
extension tool in [bmext](bmext) (or download it from the
[latest release](https://github.com/Cn8001/BMEditorExtensions/releases/latest)),
then work in an extension's folder:

```
go -C bmext install ./cmd/bmx
cd bmeditor-vim
bmx dev     # build for this system and install it in BMEditor; reload the window
bmx check   # install it from a clean clone of its own files, as BMEditor does from git
bmx pack    # vim-<version>.zip for every system, as published
```

Building requires Node.js and Go. Each extension builds on its own, so it
can also live in a repository of its own; `bmx init` starts a new one.

## License

MIT. See [LICENSE](LICENSE).
