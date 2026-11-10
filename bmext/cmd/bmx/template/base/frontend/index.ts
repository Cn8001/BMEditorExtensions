import type { ExtensionAPI } from '../bmext/bmeditor'

// The editor calls this once, when it starts, with its API (bmext/bmeditor.d.ts).
export default function activate(api: ExtensionAPI) {
  api.register({
    commands: [
      {
        id: '{{.Name}}-hello',
        label: '{{.Title}}: Hello',
{{- if .Backend}}
        run: () => api.call<string>('hello').then((text) => api.ui.toast('info', text)),
{{- else}}
        run: () => api.ui.toast('info', 'Hello from {{.Title}}'),
{{- end}}
      },
    ],
  })
}
