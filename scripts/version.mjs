// Semantic versions for the release scripts.
//
//   node scripts/version.mjs valid <version>
//       exits 1 unless version is X.Y.Z or X.Y.Z-<prerelease>
//   node scripts/version.mjs newer <version> [<released>...]
//       exits 1 unless version is valid and newer than every released one
//
// Precedence follows semver.org: 0.2.0-beta.1 < 0.2.0 < 0.2.1.

import { pathToFileURL } from 'node:url'

const pattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/

export function parse(v) {
  const m = pattern.exec(v)
  return m && { core: m.slice(1, 4).map(Number), pre: m[4] ? m[4].split('.') : [] }
}

export function compare(a, b) {
  for (let i = 0; i < 3; i++) if (a.core[i] !== b.core[i]) return Math.sign(a.core[i] - b.core[i])
  if (!a.pre.length || !b.pre.length) return Number(!a.pre.length) - Number(!b.pre.length)
  for (let i = 0; i < Math.min(a.pre.length, b.pre.length); i++) {
    const x = a.pre[i], y = b.pre[i]
    if (x === y) continue
    const xn = /^\d+$/.test(x), yn = /^\d+$/.test(y)
    if (xn && yn) return Math.sign(Number(x) - Number(y))
    if (xn !== yn) return xn ? -1 : 1
    return x < y ? -1 : 1
  }
  return Math.sign(a.pre.length - b.pre.length)
}

// The command line, when run rather than imported (catalog.mjs imports it).
if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) main()

function fail(message) {
  console.error(message)
  process.exit(1)
}

function main() {
  const [command, version, ...released] = process.argv.slice(2)
  const v = parse(version ?? '')
  if (!v) fail(`version "${version ?? ''}" is not a semantic version (X.Y.Z or X.Y.Z-pre)`)

  if (command === 'newer') {
    for (const r of released) {
      const p = parse(r)
      if (p && compare(v, p) <= 0) fail(`version ${version} is not newer than the released ${r}`)
    }
  } else if (command !== 'valid') {
    fail(`usage: version.mjs valid|newer <version> [<released>...]`)
  }
}
