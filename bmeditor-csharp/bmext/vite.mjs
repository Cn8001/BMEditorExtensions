// Vite settings for an extension's frontend script. The editor runs the
// script as a classic <script>, so it is built as an IIFE, and the
// libraries the editor already has are not bundled: the script reads them
// from window.bmeditor.shared, which makes a CodeMirror state or a Vue
// component of the extension the editor's own.
//
//   // vite.config.mjs of an extension
//   import vue from '@vitejs/plugin-vue'
//   import { bmext } from './bmext/vite.mjs' // a copy of this file
//   export default { plugins: [vue()], ...bmext({ name: 'my-ext' }) }

export const shared = {
  vue: 'bmeditor.shared.vue',
  '@codemirror/state': 'bmeditor.shared.cmState',
  '@codemirror/view': 'bmeditor.shared.cmView',
  '@codemirror/language': 'bmeditor.shared.cmLanguage',
  '@codemirror/lint': 'bmeditor.shared.cmLint',
  '@codemirror/autocomplete': 'bmeditor.shared.cmAutocomplete',
  '@codemirror/commands': 'bmeditor.shared.cmCommands',
  '@lezer/highlight': 'bmeditor.shared.lezerHighlight',
}

// bmext returns the build settings: the entry's default export becomes
// window.bmext_<name>.default, which the editor calls with its API.
export function bmext({ name, entry = 'frontend/index.ts', outDir = 'dist/frontend' }) {
  return {
    define: { 'process.env.NODE_ENV': '"production"' },
    build: {
      outDir,
      emptyOutDir: true,
      cssCodeSplit: false,
      lib: { entry, formats: ['iife'], name: 'bmext_' + name.replace(/-/g, '_'), fileName: () => 'index.js' },
      rollupOptions: {
        external: Object.keys(shared),
        output: { globals: shared, exports: 'named', assetFileNames: 'style[extname]' },
      },
    },
  }
}
