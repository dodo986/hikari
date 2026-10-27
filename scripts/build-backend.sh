#!/usr/bin/env bash
# Build the native filegraph extension the vendored modding backend needs.
set -euo pipefail
here="$(cd "$(dirname "$0")/.." && pwd)"
crate="$here/internal/mods/backend/native/amethyst_filegraph"
out="${1:-$here/amethyst_filegraph.abi3.so}"

if ! command -v cargo >/dev/null 2>&1; then
	echo "build-backend.sh: cargo is needed to build the filegraph extension" >&2
	exit 1
fi
if [ ! -f "$crate/Cargo.toml" ]; then
	echo "build-backend.sh: $crate is missing; run scripts/vendor-amethyst.sh first" >&2
	exit 1
fi

(cd "$crate" && cargo build --release --locked)
built=""
for candidate in "$crate/target/release/amethyst_filegraph.so" \
	"$crate/target/release/libamethyst_filegraph.so"; do
	if [ -f "$candidate" ]; then
		built="$candidate"
		break
	fi
done
if [ -z "$built" ]; then
	echo "build-backend.sh: cargo produced no amethyst_filegraph shared object" >&2
	exit 1
fi
install -Dm755 "$built" "$out"
echo "filegraph extension: $out"
