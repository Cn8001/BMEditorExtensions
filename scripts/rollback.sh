#!/bin/sh
# Makes an earlier released version the latest of an extension again, when
# the newest one is broken: users who installed from the latest URL get it
# with their next Update. The version releases themselves are left as they
# are, and the next published version becomes the latest as usual. Needs gh
# authenticated (GH_TOKEN in CI).
#
#   scripts/rollback.sh bmeditor-vim 0.1.0
set -eu

dir=${1:?usage: rollback.sh <extension dir> <version>}
version=${2:?usage: rollback.sh <extension dir> <version>}
remote=${REMOTE:-origin}
here=$(dirname "$0")
[ -f "$dir/bmext.json" ] || { echo "rollback: $dir/bmext.json missing" >&2; exit 1; }

node "$here/version.mjs" valid "$version"
case $version in *-*) echo "rollback: $version is a prerelease; only a stable version can be the latest" >&2; exit 1 ;; esac

name=$(node -p "require('./$dir/bmext.json').name")
title=$(node -p "const m = require('./$dir/bmext.json'); m.displayName || m.name")
tag="$name/v$version"

if ! git ls-remote --exit-code --tags "$remote" "refs/tags/$tag" >/dev/null 2>&1; then
	echo "rollback: $tag is not released" >&2
	exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
gh release download "$tag" --pattern "$name-$version.zip" --dir "$work"
sh "$here/set-latest.sh" "$name" "$title" "$version" "$work/$name-$version.zip"
