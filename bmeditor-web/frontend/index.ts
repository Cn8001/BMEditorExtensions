import { watch } from 'vue'
import { LanguageSupport, StreamLanguage } from '@codemirror/language'
import { css, less, sCSS } from '@codemirror/legacy-modes/mode/css'
import { xml } from '@codemirror/legacy-modes/mode/xml'
import type { ExtensionAPI } from '../bmext/bmeditor'
import { images, openPath, openTab, pick, setApi, viewer, panelId } from './api'
import { svelte, vue } from './sfc'
import { autoCloseTags } from './autoclose'
import ImagePanel from './ImagePanel.vue'

type Parser = Parameters<typeof StreamLanguage.define>[0]
const support = (parser: Parser, markup = false) => new LanguageSupport(StreamLanguage.define(parser), markup ? [autoCloseTags] : [])
const grammars = {
  css: support(css),
  scss: support(sCSS),
  less: support(less),
  svelte: support(svelte, true),
  vue: support(vue, true),
  svg: support(xml, true),
}

// HTML is the editor's own, and JavaScript and TypeScript use the grammars
// it carries (bmext.json); this script adds the rest and the image panel.
export default function activate(api: ExtensionAPI) {
  setApi(api)
  api.register({
    languages: Object.entries(grammars).map(([id, lang]) => ({ id, support: () => lang })),
    panels: [{ id: panelId, label: 'Image', component: ImagePanel }],
    commands: [
      {
        id: 'web-open-image',
        label: 'Image: Open Image…',
        icon: 'file-media',
        run: () => {
          api.ui.showPanel(panelId)
          pick.asked++
        },
      },
      {
        id: 'web-preview-svg',
        label: 'Image: Preview SVG',
        icon: 'eye',
        when: () => api.ui.activeTab()?.language === 'svg',
        run: () => openTab(api.ide.activeId),
      },
    ],
  })

  // BMEditor opens text files only, so a click on an image in the Explorer
  // ends in its "Cannot open <path>" notice. That notice is taken as the
  // click: it is put away and the image shown here instead.
  const toasts = api.ide.toasts as { id: number; title: string }[]
  watch(
    () => toasts.length,
    () => {
      for (let i = toasts.length - 1; i >= 0; i--) {
        const path = /^Cannot open (.+)$/.exec(toasts[i].title)?.[1]
        if (!path || !images.test(path)) continue
        toasts.splice(i, 1)
        openPath(path)
      }
    },
  )

  // An SVG opens as code; the panel shows it beside, live, as it becomes
  // the active tab.
  watch(
    () => api.ui.activeTab(),
    (tab) => {
      if (tab?.language === 'svg' && viewer.tabId !== tab.id) openTab(tab.id)
    },
    { immediate: true },
  )

  // An SVG in the panel follows its tab as it is edited.
  watch(
    () => viewer.tabId && api.session.tab(viewer.tabId)?.version,
    () => viewer.tabId && openTab(viewer.tabId, false),
  )
}
