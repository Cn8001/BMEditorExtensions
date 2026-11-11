import { reactive } from 'vue'
import type { ExtensionAPI } from '../bmext/bmeditor'

// The editor's API, kept from activate for the components.
export let api: ExtensionAPI

export function setApi(a: ExtensionAPI) {
  api = a
}

export const panelId = 'web-image'

// The image the panel shows: src is a URL for <img>; tabId is set for an SVG
// shown live from its tab. An SVG's size is what its width, height and
// viewBox say (0 when they say nothing), as a browser does not give one.
export const viewer = reactive({ src: '', name: '', bytes: -1, tabId: '', failed: false, svg: false, width: 0, height: 0, viewBox: '' })

// The Open Image command asks; the panel answers with its file picker, also
// when it is first drawn after the asking.
export const pick = reactive({ asked: 0, answered: 0 })

// What the editor's /workspace/ route serves from the open folder.
export const images = /\.(png|jpe?g|gif|webp|avif|bmp|ico|svg)$/i

let objectUrl = ''

const isSvg = (name: string) => /\.svg$/i.test(name)

function setSrc(src: string, name: string, bytes = -1, tabId = '', show = true, svgText?: string) {
  if (objectUrl && objectUrl !== src) URL.revokeObjectURL(objectUrl)
  objectUrl = src.startsWith('blob:') ? src : ''
  const size = svgText === undefined ? { width: 0, height: 0, viewBox: '' } : svgSize(svgText)
  Object.assign(viewer, { src, name, bytes, tabId, failed: false, svg: isSvg(name) || svgText !== undefined, ...size })
  if (show) api.ui.showPanel(panelId)
}

const length = (v: string | null): number => {
  // 24, 24px, 2em... ; a percentage says nothing about the size.
  const m = /^\s*([\d.]+)\s*(px|pt|em|in|cm|mm)?\s*$/.exec(v ?? '')
  if (!m) return 0
  const k: Record<string, number> = { px: 1, pt: 4 / 3, em: 16, in: 96, cm: 96 / 2.54, mm: 96 / 25.4 }
  return parseFloat(m[1]) * (k[m[2] ?? 'px'] ?? 1)
}

// svgSize reads an SVG's size: its width and height, else its viewBox's.
export function svgSize(text: string): { width: number; height: number; viewBox: string } {
  const root = new DOMParser().parseFromString(text, 'image/svg+xml').documentElement
  if (!root || root.nodeName !== 'svg') return { width: 0, height: 0, viewBox: '' }
  const viewBox = (root.getAttribute('viewBox') ?? '').trim()
  const vb = viewBox.split(/[\s,]+/).map(Number)
  let width = length(root.getAttribute('width'))
  let height = length(root.getAttribute('height'))
  if (vb.length === 4 && vb[2] > 0 && vb[3] > 0) {
    if (!width && !height) [width, height] = [vb[2], vb[3]]
    else if (!width) width = (height * vb[2]) / vb[3]
    else if (!height) height = (width * vb[3]) / vb[2]
  }
  return { width: Math.round(width * 100) / 100, height: Math.round(height * 100) / 100, viewBox }
}

// The image of the open folder the panel shows, followed on disk: its
// path, the URL without the cache buster, and the version last read.
const followed = { path: '', url: '', stamp: '' }
let timer = 0

// openPath shows an image of the open folder, read again each time: the
// URL carries a new ?v= so that the webview does not show what it cached.
export function openPath(path: string, show = true) {
  const rel = api.paths.relativeTo(api.ide.folder, path)
  const url = '/workspace/' + rel.split(/[\\/]/).map(encodeURIComponent).join('/')
  const src = `${url}?v=${Date.now()}`
  setSrc(src, rel, -1, '', show)
  Object.assign(followed, { path, url, stamp: '' })
  void stampOf(url).then((st) => {
    if (followed.url === url) followed.stamp = st ?? ''
  })
  if (isSvg(rel)) {
    fetch(src, { cache: 'no-store' })
      .then((r) => (r.ok ? r.text() : Promise.reject()))
      .then((text) => viewer.src === src && Object.assign(viewer, svgSize(text), { bytes: new Blob([text]).size }))
      .catch(() => {})
  }
  follow()
}

// reload reads the image shown again: from disk, or from its tab.
export function reload() {
  if (viewer.tabId) openTab(viewer.tabId, false)
  else if (followed.path) openPath(followed.path, false)
}

// stampOf is what says an image changed: its time and size on disk; null
// when it is gone.
async function stampOf(url: string): Promise<string | null> {
  try {
    const r = await fetch(url, { method: 'HEAD', cache: 'no-store' })
    if (!r.ok) return null
    return `${r.headers.get('Last-Modified')}|${r.headers.get('Content-Length')}`
  } catch {
    return null
  }
}

// follow checks the image shown every second and a half while the window
// is in view, and shows it again when it changed on disk.
function follow() {
  if (timer) return
  timer = window.setInterval(async () => {
    if (!followed.url || viewer.tabId || !viewer.src.startsWith(followed.url)) {
      window.clearInterval(timer)
      timer = 0
      return
    }
    if (document.hidden) return
    const url = followed.url
    const st = await stampOf(url)
    if (followed.url !== url) return
    if (st === null) {
      viewer.failed = true
      return
    }
    if (followed.stamp && st !== followed.stamp) openPath(followed.path, false)
    followed.stamp = st
  }, 1500)
}

// openFile shows an image from the file picker or the desktop.
export function openFile(f: File) {
  followed.url = ''

  if (!f.type.startsWith('image/')) {
    api.ui.toast('info', `${f.name} is not an image`)
    return
  }
  const url = URL.createObjectURL(f)
  setSrc(url, f.name, f.size)
  if (f.type === 'image/svg+xml') void f.text().then((text) => viewer.src === url && Object.assign(viewer, svgSize(text), { svg: true }))
}

// openTab shows an SVG tab as it is in the editor, saved or not; show is
// false for the refresh as it is typed, which leaves the panels alone.
export function openTab(id: string, show = true) {
  const tab = api.session.tab(id)
  if (!tab) return
  followed.url = ''
  const text = api.session.textOf(id)
  const blob = new Blob([text], { type: 'image/svg+xml' })
  setSrc(URL.createObjectURL(blob), tab.name, blob.size, id, show, text)
}
