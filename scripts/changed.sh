#!/bin/sh
# Prints the extensions changed since <base> as a GitHub Actions output, a
# JSON list of directories, so that the workflow builds only those:
#
#   dirs=["bmeditor-vim"]
#
# Every extension is listed when <base> is unknown (a manual run, a new
# branch, a force push) or when files they share changed (scripts/, bmext/
# with bmx and the SDK they copy, the workflow). An extension is a
# directory with a bmext.json at its root.
#
#   scripts/changed.sh <base commit>
set -eu

base=${1:-}
all=$(for m in */bmext.json; do dirname "$m"; done)

# Whether base is a commit HEAD descends from. GitHub sends all zeros as
# the base of a new branch.
known() {
	case $base in *[!0]*) ;; *) return 1 ;; esac
	git merge-base --is-ancestor "$base" HEAD 2>/dev/null
}

if known; then
	files=$(git diff --name-only "$base" HEAD)
	# bmext's tests and docs do not change how an extension is built or checked.
	shared=$(printf '%s\n' "$files" | grep -E '^(scripts/|bmext/|\.github/workflows/publish\.yml$)' |
		grep -vE '^bmext/.*(_test\.go|\.md)$' || true)
	if [ -n "$shared" ]; then
		changed=$all
	else
		changed=
		for d in $all; do
			if printf '%s\n' "$files" | grep -q "^$d/"; then changed="$changed $d"; fi
		done
	fi
else
	changed=$all
fi

printf 'dirs=['
sep=
for d in $changed; do
	printf '%s"%s"' "$sep" "$d"
	sep=,
done
printf ']\n'
