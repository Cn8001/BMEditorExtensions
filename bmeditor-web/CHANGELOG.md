# Changelog

Each version of the Web extension, newest first. The section of a version
becomes the notes of its release, and a version is published only when it
has a section here.

## 0.3.1

- SVG in the Image panel: an SVG tab shows there, live, as it becomes the
  active tab; it fills the panel when fitted, and its size comes from its
  width, height or viewBox.
- Zoom in and out (buttons or Ctrl+wheel), 1:1, and a light background for
  dark images.
- An image changed on disk shows again by itself, and clicking another
  image, or the same one again, always reads it anew: the webview's cache
  kept the old one. Refresh reads it again by hand.

## 0.3.0

First published version.

- JavaScript and TypeScript with TypeScript 7's language server (`tsc --lsp`).
- CSS, SCSS and Less with vscode-css-language-server.
- HTML with vscode-html-language-server.
- Svelte with svelte-language-server.
- Vue with @vue/language-server 2, through the bundled `vue-ls` wrapper.
- SVG highlighted as XML, with a live preview.
- Tags close automatically in Svelte, Vue and SVG.
- An Image panel for PNG, JPG, GIF, WebP, AVIF, BMP and ICO files.
- File icons for every language in the Explorer and the tabs.
