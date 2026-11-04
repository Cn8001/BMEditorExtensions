#!/bin/sh
# Makes a version's zip the latest of an extension: the asset <name>.zip of
# the release <name>/latest, whose URL, and an Update from it, therefore
# reach that version. Used by publish.sh for each stable version and by
# rollback.sh to go back to an earlier one. Needs gh authenticated.
#
#   scripts/set-latest.sh <name> <title> <version> <zip>
set -eu

name=${1:?usage: set-latest.sh <name> <title> <version> <zip>}
title=${2:?}
version=${3:?}
zip=${4:?}
tag="$name/latest"
target=${GITHUB_SHA:-$(git rev-parse HEAD)}
repo_url="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-Cn8001/BMEditorExtensions}"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp "$zip" "$work/$name.zip"

if gh release view "$tag" >/dev/null 2>&1; then
	gh release upload "$tag" "$work/$name.zip" --clobber
	gh release edit "$tag" --title "$title (latest: $version)"
else
	cat >"$work/notes.md" <<EOF
The latest version of $title. Install it in BMEditor (Extensions, Ctrl+Shift+X) from this URL; Update fetches the newest version from the same URL.

\`\`\`
$repo_url/releases/download/$tag/$name.zip
\`\`\`

Every version is also kept as a release of its own, tagged \`$name/v<version>\`.
EOF
	gh release create "$tag" "$work/$name.zip" --target "$target" --latest=false \
		--title "$title (latest: $version)" --notes-file "$work/notes.md"
fi
echo "$name: latest is now $version"
