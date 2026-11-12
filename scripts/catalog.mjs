// Writes the catalog BMEditor's Extension Manager shows: each extension of
// this repository with a latest release, as that release has it (name,
// description, version, icon) and the URL Update follows.
//
//   node scripts/catalog.mjs <out.json>
import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { extname, join } from 'node:path'
import { compare, parse } from './version.mjs'

const out = process.argv[2]
if (!out) {
  console.error('usage: catalog.mjs <out.json>')
  process.exit(1)
}
const remote = process.env.REMOTE || 'origin'
const repo = `${process.env.GITHUB_SERVER_URL || 'https://github.com'}/${process.env.GITHUB_REPOSITORY || 'Cn8001/BMEditorExtensions'}`
const git = (...args) => execFileSync('git', args, { maxBuffer: 64 << 20 })
const types = { '.svg': 'image/svg+xml', '.png': 'image/png' }

const entries = []
for (const dir of readdirSync('.').filter((d) => existsSync(join(d, 'bmext.json'))).sort()) {
  const { name } = JSON.parse(readFileSync(join(dir, 'bmext.json'), 'utf8'))
  // The commit the latest release was built from: a rollback may have made
  // an older version the latest, so the newest tag is not always it.
  try {
    git('fetch', '-q', '--depth', '1', remote, `refs/tags/${name}/latest`)
  } catch {
    continue // not released yet
  }
  const m = JSON.parse(git('show', `FETCH_HEAD:${dir}/bmext.json`))
  if (!parse(m.version)) continue
  let icon = ''
  if (m.icon && types[extname(m.icon)]) {
    try {
      icon = `data:${types[extname(m.icon)]};base64,${git('show', `FETCH_HEAD:${dir}/${m.icon}`).toString('base64')}`
    } catch {
      // no icon then
    }
  }
  entries.push({
    name,
    displayName: m.displayName || name,
    description: m.description || '',
    version: m.version,
    icon,
    install: `${repo}/releases/download/${name}/latest/${name}.zip`,
    homepage: `${repo}/tree/master/${dir}`,
  })
}
entries.sort((a, b) => a.displayName.localeCompare(b.displayName) || compare(parse(b.version), parse(a.version)))
writeFileSync(out, JSON.stringify({ extensions: entries }, null, 2) + '\n')
console.log(`catalog: ${entries.map((e) => `${e.name} ${e.version}`).join(', ')}`)
