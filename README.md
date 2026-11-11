# BMEditor Extensions

Extensions for [BMEditor](https://github.com/Cn8001/BMEditor-Releases), a
desktop code editor. BMEditor can be downloaded from its
[releases page](https://github.com/Cn8001/BMEditor-Releases/releases).

## Extensions

| Extension | Description |
|---|---|
| [bmeditor-web](bmeditor-web) | JavaScript, TypeScript, CSS, SCSS, Less, HTML, Svelte, Vue and SVG, each with its language server, and an image viewer panel |
| [bmeditor-vim](bmeditor-vim) | Vim key bindings in every editor tab |
| [bmeditor-csharp](bmeditor-csharp) | C# for Unity and UPM packages: language server, Solution Explorer, assembly graph, Inspector for .asmdef and package.json, Unity compile errors, .meta and GUID tools |
| [bmeditor-powershell](bmeditor-powershell) | PowerShell with PowerShell Editor Services (completion, PSScriptAnalyzer, debugging) and cmd batch files with a language server of their own and F5 to run them |
| [bmeditor-autocomplete](bmeditor-autocomplete) | Inline code suggestions in every language from a small model on your machine (llama.cpp with Qwen2.5-Coder, or Ollama) |

The [bmext](bmext) directory holds the extension format and SDK. See
[bmext/README.md](bmext/README.md) or
[Writing extensions](https://github.com/Cn8001/BMEditor-Releases/blob/master/docs/extensions.md).

## Installation

1. In BMEditor, open Extensions (Ctrl+Shift+X).
2. Enter the install URL of the extension: its name without `bmeditor-`
   in

   ```
   https://github.com/Cn8001/BMEditorExtensions/releases/download/<name>/latest/<name>.zip
   ```

   For bmeditor-web, for example:
   `https://github.com/Cn8001/BMEditorExtensions/releases/download/web/latest/web.zip`.

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
