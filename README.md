# BMEditor Extensions

Extensions for [BMEditor](https://github.com/Cn8001/BMEditor-Releases), a
desktop code editor. BMEditor can be downloaded from its
[releases page](https://github.com/Cn8001/BMEditor-Releases/releases).

## Extensions

| Extension | Description |
|---|---|
| [bmeditor-web](bmeditor-web) | JavaScript, TypeScript, CSS, SCSS, Less, HTML, Svelte, Vue and SVG, each with its language server, and an image viewer panel |
| [bmeditor-vim](bmeditor-vim) | Vim key bindings in every editor tab |

The [bmext](bmext) directory holds the extension format and SDK. See
[bmext/README.md](bmext/README.md) or
[Writing extensions](https://github.com/Cn8001/BMEditor-Releases/blob/master/docs/extensions.md).

## Installation

1. In BMEditor, open Extensions (Ctrl+Shift+X).
2. Enter the install URL of the extension:

   | Extension | Install URL |
   |---|---|
   | bmeditor-web | `https://github.com/Cn8001/BMEditorExtensions/releases/download/web/latest/web.zip` |
   | bmeditor-vim | `https://github.com/Cn8001/BMEditorExtensions/releases/download/vim/latest/vim.zip` |

3. Reload the window when prompted.

These URLs always point to the latest version, and Update in BMEditor keeps
the extension up to date. Earlier versions are available on the
[releases page](https://github.com/Cn8001/BMEditorExtensions/releases), each
with its own install URL and release notes.

## Building

Building from source is needed only for development:

```
cd bmeditor-vim
make dist
```

Building requires Node.js; bmeditor-web also requires Go. The result in
`dist/` can be installed with the folder button in the Extensions view.

## License

MIT. See [LICENSE](LICENSE).
