#!/bin/sh
# Makes a version's zip the latest of an extension: the asset <name>.zip of
# the release <name>/latest, whose URL, and an Update from it, therefore
# reach that version. Used by publish.sh for each stable version and by
# rollback.sh to go back to an earlier one. The tag <name>/latest is moved
# to the commit the version was built from, so the release shows that
# commit. Needs gh authenticated and push access to the repository.
#
#   scripts/set-latest.sh <name> <title> <version> <zip> [<commit>]
set -eu

name=${1:?usage: set-latest.sh <name> <title> <version> <zip> [<commit>]}
title=${2:?}
version=${3:?}
zip=${4:?}
target=${5:-${GITHUB_SHA:-$(git rev-parse HEAD)}}
remote=${REMOTE:-origin}
tag="$name/latest"
repo_url="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-Cn8001/BMEditorExtensions}"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp "$zip" "$work/$name.zip"

if gh release view "$tag" >/dev/null 2>&1; then
	git push -q -f "$remote" "$target:refs/tags/$tag"
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
