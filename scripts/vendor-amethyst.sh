#!/usr/bin/env bash
# Snapshot Amethyst's headless backend into internal/mods/backend.
#
# The snapshot is byte exact on purpose: local edits belong in backend/patches
# (documented in backend/README.md), so re-running this script produces a diff
# that only contains upstream changes.
set -euo pipefail

src=${1:?usage: vendor-amethyst.sh /path/to/Amethyst-Mod-Manager}
repo=$(cd "$(dirname "$0")/.." && pwd)
dst=$repo/internal/mods/backend
trees=(Utils Nexus Games Thunderstore LOOT wrappers)
skip=(__pycache__ '*.pyc' meson.build)

[ -f "$src/src/cli.py" ] || { echo "not an Amethyst checkout: $src" >&2; exit 1; }
[ -d "$src/native/amethyst_filegraph" ] || { echo "no native/amethyst_filegraph in $src" >&2; exit 1; }

rm -rf "$dst/src" "$dst/native"
mkdir -p "$dst/src" "$dst/native"

excludes=()
for pattern in "${skip[@]}"; do excludes+=(--exclude "$pattern"); done
# A Windows proxy DLL shipped for the Steam texture wrappers, not our code path.
excludes+=(--exclude '*.dll')

for tree in "${trees[@]}"; do
    tar -C "$src/src" "${excludes[@]}" -cf - "$tree" | tar -C "$dst/src" -xf -
done
tar -C "$src/src" -cf - app_bootstrap.py version.py | tar -C "$dst/src" -xf -

tar -C "$src/native" --exclude target --exclude '*.so' -cf - amethyst_filegraph |
    tar -C "$dst/native" -xf -

install -m644 "$src/LICENSE" "$dst/LICENSE"
git -C "$src" log -1 --format='%H%n%cd' --date=iso-strict >"$dst/upstream.commit"

files=$(find "$dst/src" "$dst/native" -type f | wc -l)
size=$(du -sh "$dst" | cut -f1)
echo "vendored $files files ($size) from $src"
echo "upstream $(head -1 "$dst/upstream.commit")"

if [ -f "$src/src/requirements-vendor.txt" ]; then
    upstream_pkgs=$(sed -e 's/#.*//' -e 's/[<>=!~ ].*//' -e '/^$/d' \
        "$src/src/requirements-vendor.txt" | tr 'A-Z' 'a-z' | sort -u)
    ours=$(sed -e 's/#.*//' -e 's/[<>=!~ ].*//' -e '/^$/d' \
        "$repo/internal/mods/bridge/requirements.txt" | tr 'A-Z' 'a-z' | sort -u)
    drift=$(comm -23 <(echo "$upstream_pkgs") <(echo "$ours"))
    if [ -n "$drift" ]; then
        echo "packages Amethyst needs that our requirements.txt lacks:"
        echo "$drift" | sed 's/^/  /'
    fi
fi

git -C "$repo" status --short -- "$dst" | wc -l | xargs echo "changed paths:"
