import { reactive } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'

// The editor's API, kept from activate for the components.
export let api: ExtensionAPI

export function setApi(a: ExtensionAPI) {
  api = a
}

export const panelId = 'web-image'

// The image the panel shows: src is a URL for <img>; tabId is set for an SVG
// shown live from its tab.
export const viewer = reactive({ src: '', name: '', bytes: -1, tabId: '', failed: false })

// The Open Image command asks; the panel answers with its file picker, also
// when it is first drawn after the asking.
export const pick = reactive({ asked: 0, answered: 0 })

// What the editor's /workspace/ route serves from the open folder.
export const images = /\.(png|jpe?g|gif|webp|avif|bmp|ico|svg)$/i

let objectUrl = ''

function setSrc(src: string, name: string, bytes = -1, tabId = '', show = true) {
  if (objectUrl && objectUrl !== src) URL.revokeObjectURL(objectUrl)
  objectUrl = src.startsWith('blob:') ? src : ''
  Object.assign(viewer, { src, name, bytes, tabId, failed: false })
  if (show) api.ui.showPanel(panelId)
}

// openPath shows an image of the open folder.
export function openPath(path: string) {
  const rel = api.paths.relativeTo(api.ide.folder, path)
  setSrc('/workspace/' + rel.split(/[\\/]/).map(encodeURIComponent).join('/'), rel)
}

// openFile shows an image from the file picker or the desktop.
export function openFile(f: File) {
  if (!f.type.startsWith('image/')) {
    api.ui.toast('info', `${f.name} is not an image`)
    return
  }
  setSrc(URL.createObjectURL(f), f.name, f.size)
}

// openTab shows an SVG tab as it is in the editor, saved or not; show is
// false for the refresh as it is typed, which leaves the panels alone.
export function openTab(id: string, show = true) {
  const tab = api.session.tab(id)
  if (!tab) return
  const blob = new Blob([api.session.textOf(id)], { type: 'image/svg+xml' })
  setSrc(URL.createObjectURL(blob), tab.name, blob.size, id, show)
}
