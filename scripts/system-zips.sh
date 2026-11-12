#!/bin/sh
# Prints the zips bmx pack wrote beside <zip> for one system each,
# <zip without .zip>-<os>-<arch>.zip, that exist.
#
#   scripts/system-zips.sh dist/web-0.3.1.zip
set -eu

zip=${1:?usage: system-zips.sh <zip>}
base=${zip%.zip}
for f in "$base"-*.zip; do
	[ -f "$f" ] || continue
	sys=${f#"$base-"}
	sys=${sys%.zip}
	case $sys in
	*-*-*) continue ;; # another version's, such as 0.1.0-beta.1-linux-amd64
	linux-* | darwin-* | windows-* | freebsd-* | openbsd-* | netbsd-*) echo "$f" ;;
	esac
done
