#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
version="${1:?usage: scripts/build-appimage.sh <version> [arch]}"
version="${version#v}"
arch="${2:-x86_64}"
tool="${APPIMAGETOOL:-$(command -v appimagetool || true)}"
if [ -z "$tool" ]; then
	echo "build-appimage.sh: appimagetool not found; install it or set APPIMAGETOOL" >&2
	exit 1
fi
dist="$here/dist"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
appdir="$work/Hikari.AppDir"
mkdir -p "$appdir/usr/bin" "$appdir/usr/share/hikari" "$appdir/usr/share/applications" \
	"$appdir/usr/share/icons/hicolor/scalable/apps" "$appdir/usr/share/licenses/hikari"

(cd "$here" && go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$appdir/usr/bin/hikarid" ./cmd/hikarid)
(cd "$here" && go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$appdir/usr/bin/hikari" ./cmd/hikari)
"$here/scripts/build-backend.sh" "$appdir/usr/share/hikari/amethyst_filegraph.abi3.so"
if [ "$("$appdir/usr/bin/hikarid" --version)" != "hikarid $version" ]; then
	echo "build-appimage.sh: the daemon reports the wrong version" >&2
	exit 1
fi

cp -r "$here/quickshell" "$appdir/usr/share/hikari/quickshell"
rm -rf "$appdir/usr/share/hikari/quickshell/tests" "$appdir/usr/share/hikari/quickshell/README.md"
install -m755 "$here/packaging/appimage/AppRun" "$appdir/AppRun"
install -m644 "$here/packaging/hikari-appimage.desktop" "$appdir/hikari.desktop"
install -m644 "$here/packaging/hikari-appimage.desktop" "$appdir/usr/share/applications/hikari.desktop"
install -m644 "$here/packaging/hikari.svg" "$appdir/hikari.svg"
install -m644 "$here/packaging/hikari.svg" "$appdir/usr/share/icons/hicolor/scalable/apps/hikari.svg"
install -m644 "$here/LICENSE" "$here/NOTICE" "$appdir/usr/share/licenses/hikari/"

mkdir -p "$dist"
out="$dist/hikari-v$version-$arch.AppImage"
runner=()
case "$tool" in
	*.AppImage) runner=(--appimage-extract-and-run) ;;
esac
ARCH="$arch" "$tool" "${runner[@]}" --no-appstream "$appdir" "$out"
chmod 755 "$out"
(cd "$dist" && sha256sum "$(basename "$out")" > "$(basename "$out").sha256")
echo "appimage $version:"
echo "  $out"
echo "  $out.sha256"
