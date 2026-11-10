#!/bin/sh
# Publishes a new version of an extension as GitHub releases, so that
# BMEditor installs it from a URL without building, and no build output is
# kept in the repository itself:
#
#   .../releases/download/<name>/v<version>/<name>-<version>.zip   that version
#   .../releases/download/<name>/latest/<name>.zip                 the latest
#
# A version is published once, when the version in bmext.json is not
# released yet; a push without a version change publishes nothing. The
# version must be newer than every released one and have an entry in the
# extension's CHANGELOG.md, which becomes the release notes. A stable
# version also becomes the latest (set-latest.sh); a prerelease
# (X.Y.Z-beta.1) does not.
#
# Run from the repository root after `bmx pack` in <dir>, with gh
# authenticated (GH_TOKEN in CI):
#
#   scripts/publish.sh bmeditor-vim
set -eu

dir=${1:?usage: publish.sh <extension dir>}
remote=${REMOTE:-origin}
here=$(dirname "$0")
manifest="$dir/dist/bmext.json"
[ -f "$manifest" ] || { echo "publish: $manifest missing; run bmx pack in $dir" >&2; exit 1; }

field() { node -p "const m = require('./$manifest'); String(($1) ?? '')"; }
name=$(field m.name)
title=$(field 'm.displayName || m.name')
version=$(field m.version)
engine=$(field 'm.engines?.bmeditor')
tag="$name/v$version"
zip="$dir/$name-$version.zip"
target=${GITHUB_SHA:-$(git rev-parse HEAD)}
repo_url="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-Cn8001/BMEditorExtensions}"

released=$(git ls-remote --tags "$remote" "refs/tags/$name/v*" | sed -n "s|.*refs/tags/$name/v\([^^]*\)\$|\1|p")
for r in $released; do
	if [ "$r" = "$version" ]; then
		echo "$name: $version is already released; bump version in $dir/bmext.json to publish"
		exit 0
	fi
done

# shellcheck disable=SC2086 # one argument per released version
node "$here/version.mjs" newer "$version" $released
[ -f "$zip" ] || { echo "publish: $zip missing; run bmx pack in $dir" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
changes=$(sh "$here/changelog.sh" "$dir" "$version")

prerelease=
case $version in *-*) prerelease=--prerelease ;; esac

{
	printf '%s\n\n' "$changes"
	if [ -n "$engine" ]; then printf 'Requires BMEditor %s.\n\n' "$engine"; fi
	printf 'Install this version in BMEditor (Extensions, Ctrl+Shift+X):\n\n```\n%s\n```\n' \
		"$repo_url/releases/download/$tag/$name-$version.zip"
	if [ -z "$prerelease" ]; then
		printf '\nOr install the latest version, which Update follows:\n\n```\n%s\n```\n' \
			"$repo_url/releases/download/$name/latest/$name.zip"
	else
		printf '\nThis is a prerelease: the latest version, which Update follows, stays as it is.\n'
	fi
} >"$work/notes.md"

# shellcheck disable=SC2086 # $prerelease is empty or one flag
gh release create "$tag" "$zip" --target "$target" --latest=false $prerelease \
	--title "$title $version" --notes-file "$work/notes.md"
echo "$name: released $tag"

if [ -z "$prerelease" ]; then
	sh "$here/set-latest.sh" "$name" "$title" "$version" "$zip"
fi
