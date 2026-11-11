import { api, refresh, setOn, state } from './state'

// The commands' and the status bar's actions.

export async function getReady() {
  api.ui.showOutput?.()
  try {
    await api.call('prepare')
    await refresh()
  } catch (err) {
    const f = api.ui.failureOf(err)
    api.ui.output?.(f.message, 'stderr')
    api.ui.toastFailure('Cannot get the model ready', f)
  }
}

export function toggle() {
  setOn(!state.on)
  api.ui.toast('info', state.on ? 'Autocomplete suggestions on' : 'Autocomplete suggestions paused')
}

export async function removeDownloads() {
  try {
    await api.call('remove')
    await refresh()
    api.ui.toast('info', 'Downloaded models removed', 'Autocomplete: Get Model Ready downloads one again.')
  } catch (err) {
    api.ui.toastFailure('Cannot remove the downloads', api.ui.failureOf(err))
  }
}
