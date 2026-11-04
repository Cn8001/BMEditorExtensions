#!/bin/sh
# Prints the notes of one version from an extension's CHANGELOG.md: the lines
# between its "## <version>" heading and the next "## " heading. The heading
# may carry more after the version, such as a date: "## 0.2.0 (2026-11-04)".
# Exits 1 when the version has no entry or the entry is empty.
#
#   scripts/changelog.sh bmeditor-vim 0.1.0
set -eu

dir=${1:?usage: changelog.sh <extension dir> <version>}
version=${2:?usage: changelog.sh <extension dir> <version>}
file="$dir/CHANGELOG.md"
[ -f "$file" ] || { echo "changelog: $file missing" >&2; exit 1; }

notes=$(awk -v v="$version" '
	/^## / {
		if (on) exit
		h = $2
		gsub(/^\[|\]$/, "", h)
		if (h == v) on = 1
		next
	}
	on { line[++n] = $0 }
	END {
		s = 1; while (s <= n && line[s] ~ /^[[:space:]]*$/) s++
		e = n; while (e >= s && line[e] ~ /^[[:space:]]*$/) e--
		for (i = s; i <= e; i++) print line[i]
	}
' "$file")

[ -n "$notes" ] || { echo "changelog: no entry for $version in $file" >&2; exit 1; }
printf '%s\n' "$notes"
