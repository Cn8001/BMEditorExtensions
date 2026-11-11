# Web for BMEditor

Web languages in BMEditor, each with its language server, and an image
viewer.

| Language | Files | Language server |
|---|---|---|
| JavaScript | `.js` `.mjs` `.cjs` `.jsx` | TypeScript 7's own (`tsc --lsp`) |
| TypeScript | `.ts` `.mts` `.cts` `.tsx` | TypeScript 7's own (`tsc --lsp`) |
| CSS, SCSS, Less | `.css` `.scss` `.less` | vscode-css-language-server |
| HTML | `.html` `.htm` (highlighting and preview are BMEditor's own) | vscode-html-language-server |
| Svelte | `.svelte`: markup, `<script>`, `<style>` and `{...}` blocks | svelte-language-server |
| Vue | `.vue`: template and `{{ }}`, `<script>` (TypeScript with `lang="ts"`), `<style>` | @vue/language-server 2 (for Vue 3 code), through the bundled `vue-ls` |
| SVG | `.svg`, highlighted as XML, with a live preview | none |

Each file type has its own icon in the Explorer and the tabs. In Svelte, Vue
and SVG, tags close as in VS Code: typing the `>` of `<div>` adds `</div>`,
and `</` finishes the tag still open.

The language servers are optional: without one you still get highlighting,
and BMEditor shows how to install it the first time it is missing. To have
them all:

```
npm install -g typescript vscode-langservers-extracted svelte-language-server @vue/language-server@2
```

Vue's server needs a TypeScript with its JavaScript API, which TypeScript 7
no longer has. `vue-ls` takes the project's own (its `node_modules`), else
TypeScript 5 that another global package carries, so a global TypeScript 7
does not get in the way.

## Images

Click a PNG, JPG, GIF, WebP, AVIF, BMP or ICO file in the Explorer and it
opens in the **Image** panel at the bottom. You can also run **Image: Open
Image…** from the command palette, or drag an image into the panel from the
Explorer or your desktop. Click the image to switch between fit and actual
size; zoom with the buttons or Ctrl+wheel, and switch to a light background
for a dark image. An image that changes on disk shows again by itself;
Refresh reads it again by hand.

An SVG opens as code, and the Image panel shows it beside the code as the
tab becomes active, following your typing. When fitted it fills the panel,
and its size comes from its `width`, `height` or `viewBox`. **Image: Preview
SVG** shows it again after you open another image.

## Install

In BMEditor, open Extensions (Ctrl+Shift+X), enter

```
https://github.com/Cn8001/BMEditorExtensions/releases/download/web/latest/web.zip
```

and reload the window when asked. Earlier versions are on the
[releases page](https://github.com/Cn8001/BMEditorExtensions/releases).

To build it yourself, run `make dist` (Node and Go needed) and pick `dist` with
the folder button in Extensions.

## License

MIT. See [LICENSE](LICENSE).
