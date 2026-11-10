#!/bin/sh
# Publishes a new version of bmext as a GitHub release tagged
# bmext/v<version>, the tag Go resolves for the module in bmext/, so that
#
#   go install github.com/Cn8001/BMEditorExtensions/bmext/cmd/bmx@latest
#
# installs it, and with bmx built for each system, to download without Go:
#
#   .../releases/latest/download/bmx-<os>-<arch>[.exe]     the latest
#   .../releases/download/bmext/v<version>/bmx-<os>-<arch>  that version
#
# As for an extension (publish.sh), a version is released once, when the
# one in bmext/VERSION is not released yet; it must be newer than every
# released one and have an entry in bmext/CHANGELOG.md, which becomes the
# release notes. A stable version becomes the repository's latest release;
# a prerelease (X.Y.Z-beta.1) does not. Extension releases are never the
# latest, so the latest URL stays bmx's.
#
# Run from the repository root with Go, Node and gh authenticated (GH_TOKEN
# in CI):
#
#   scripts/release-bmext.sh
set -eu

here=$(dirname "$0")
remote=${REMOTE:-origin}
version=$(tr -d ' \r\n' <bmext/VERSION)
tag="bmext/v$version"
target=${GITHUB_SHA:-$(git rev-parse HEAD)}
repo_url="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-Cn8001/BMEditorExtensions}"

released=$(git ls-remote --tags "$remote" 'refs/tags/bmext/v*' | sed -n 's|.*refs/tags/bmext/v\([^^]*\)$|\1|p')
for r in $released; do
	if [ "$r" = "$version" ]; then
		echo "bmext: $version is already released; bump bmext/VERSION to publish"
		exit 0
	fi
done

# shellcheck disable=SC2086 # one argument per released version
node "$here/version.mjs" newer "$version" $released
changes=$(sh "$here/changelog.sh" bmext "$version")

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
for t in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
	os=${t%/*} arch=${t#*/} ext=
	[ "$os" = windows ] && ext=.exe
	echo "go build bmx $os/$arch"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go -C bmext build -trimpath \
		-ldflags "-s -w -X main.version=$version" -o "$work/bin/bmx-$os-$arch$ext" ./cmd/bmx
done

latest=--latest
case $version in *-*) latest="--latest=false --prerelease" ;; esac

{
	printf '%s\n\n' "$changes"
	printf 'Install bmx with Go:\n\n```\ngo install github.com/Cn8001/BMEditorExtensions/bmext/cmd/bmx@v%s\n```\n\n' "$version"
	printf 'or download it for your system below: bmx-linux-amd64, bmx-darwin-arm64, bmx-windows-amd64.exe and so on.\n'
	case $version in *-*) ;; *)
		printf '\nThe latest bmx is always at\n\n```\n%s\n```\n' "$repo_url/releases/latest/download/bmx-<os>-<arch>"
		;;
	esac
} >"$work/notes.md"

# shellcheck disable=SC2086 # $latest is one or two flags
gh release create "$tag" "$work"/bin/* --target "$target" $latest \
	--title "bmx $version" --notes-file "$work/notes.md"
echo "bmext: released $tag"
