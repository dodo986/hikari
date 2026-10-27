#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
version="${1:?usage: scripts/release.sh <version> [arch]}"
version="${version#v}"
arch="${2:-x86_64}"
tag="v$version"
dist="$here/dist"
rm -rf "$dist"
mkdir -p "$dist"
(cd "$here" && go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$dist/hikarid" ./cmd/hikarid)
(cd "$here" && go build -trimpath -ldflags "-s -w" -o "$dist/hikari" ./cmd/hikari)
if [ "$("$dist/hikarid" --version)" != "hikarid $version" ]; then
	echo "release.sh: built daemon reports the wrong version" >&2
	exit 1
fi
"$here/scripts/build-backend.sh" "$dist/amethyst_filegraph.abi3.so"
install -m755 "$here/scripts/hikari-app" "$here/scripts/hikari-update" "$dist/"
install -m755 "$here/scripts/install-tarball.sh" "$dist/install.sh"
cp -r "$here/quickshell" "$dist/quickshell"
cp "$here/packaging/hikari.desktop" "$here/packaging/hikari.svg" "$here/LICENSE" "$here/NOTICE" "$dist/"
asset="hikari-$tag-$arch.tar.gz"
tar -C "$dist" --sort=name --mtime='UTC 2020-01-01' --owner=0 --group=0 --numeric-owner \
	-czf "$dist/$asset" hikarid hikari amethyst_filegraph.abi3.so hikari-app \
	hikari-update install.sh quickshell hikari.desktop hikari.svg LICENSE NOTICE
for f in hikarid hikari amethyst_filegraph.abi3.so hikari-app hikari-update install.sh quickshell/shell.qml hikari.desktop hikari.svg LICENSE NOTICE; do
	tar -tzf "$dist/$asset" "$f" >/dev/null || { echo "release.sh: $asset is missing $f" >&2; exit 1; }
done
(cd "$dist" && sha256sum "$asset" > "$asset.sha256" && rm -rf quickshell hikarid hikari amethyst_filegraph.abi3.so hikari-app hikari-update install.sh hikari.desktop hikari.svg LICENSE NOTICE)
echo "release $tag:"
echo "  $dist/$asset"
echo "  $dist/$asset.sha256"
echo "tag the commit and attach both files to the GitHub release"
