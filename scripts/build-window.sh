#!/usr/bin/env bash
# Typechecks the web app, bundles it, and links the Wails window.
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:-$here/frontend/build/hikari-ui}"

cd "$here/frontend/web"
if [ ! -d node_modules ]; then
	npm ci --prefer-offline --no-audit --no-fund
fi
./node_modules/.bin/tsc -b --noEmit
./node_modules/.bin/vite build

rm -rf "$here/frontend/host/dist"
cp -r "$here/frontend/web/dist" "$here/frontend/host/dist"

cd "$here/frontend/host"
CGO_ENABLED=1 go build -tags "production webkit2_41" -trimpath -ldflags "-s -w" -o "$out" .
echo "built $out"
