#!/usr/bin/env bash
# Builds the web bundle, copies it into the host module and links the window.
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"

cd "$here/web"
./node_modules/.bin/tsc -b --noEmit
./node_modules/.bin/vite build

rm -rf "$here/host/dist"
cp -r "$here/web/dist" "$here/host/dist"

cd "$here/host"
go build -tags "production webkit2_41" -trimpath -o "$here/build/hikari-ui" .
echo "built $here/build/hikari-ui"
