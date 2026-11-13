import type { ExtensionAPI } from '../bmext/bmeditor'
import HexEditor from './HexEditor.vue'
import { save, setApi } from './state'

// A hex editor for the files that are not text, as Bless is one: the
// editor shows images itself, and hands every other such file here.
export default function activate(api: ExtensionAPI) {
  setApi(api)
  api.register({ binaryEditor: { component: HexEditor, save } })
}
