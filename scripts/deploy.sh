#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
dest="${1:-$HOME/.config/quickshell/hikari}"
mkdir -p "$dest"
cp -r "$here"/quickshell/. "$dest/"
echo "deployed -> $dest"
