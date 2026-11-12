#!/bin/sh
# Publishes the catalog (catalog.mjs) as the asset catalog.json of the
# release "catalog", where BMEditor's Extension Manager reads it:
#
#   .../releases/download/catalog/catalog.json
#
# Run from the repository root after a publish or a rollback, with gh
# authenticated (GH_TOKEN in CI).
set -eu

here=$(dirname "$0")
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
node "$here/catalog.mjs" "$work/catalog.json"

if gh release view catalog >/dev/null 2>&1; then
	gh release upload catalog "$work/catalog.json" --clobber
else
	gh release create catalog "$work/catalog.json" --latest=false --title "Extension catalog" \
		--notes "The extensions BMEditor's Extension Manager shows, each at its latest version. Every publish and rollback updates it."
fi
