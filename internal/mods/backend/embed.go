// Package backend holds Amethyst's headless modding backend as embedded data.
//
// The Python trees are shipped inside hikarid so a modding install needs
// nothing from Amethyst itself; see README.md for provenance and sync policy.
package backend

import (
	"embed"
	"strings"
)

// The crate is listed file by file on purpose: `all:native` would also swallow
// native/amethyst_filegraph/target, the cargo build directory. That tree is
// hundreds of megabytes, it is embedded into every hikarid build, and
// materializeBackend then writes it into the user's data dir.
//
//go:embed all:src
//go:embed upstream.commit
//go:embed native/amethyst_filegraph/src
//go:embed native/amethyst_filegraph/Cargo.toml
//go:embed native/amethyst_filegraph/Cargo.lock
//go:embed native/amethyst_filegraph/build.rs
//go:embed native/amethyst_filegraph/build.sh
//go:embed native/amethyst_filegraph/build_extension.py
//go:embed native/amethyst_filegraph/README.md
var files embed.FS

// Src is the directory holding Utils, Nexus, Games and friends.
const Src = "src"

// FilegraphCrate is the Rust crate that builds the native filegraph module.
const FilegraphCrate = "native/amethyst_filegraph"

// Files is the backend tree, rooted at the package directory.
func Files() embed.FS { return files }

// Upstream is the Amethyst revision this snapshot was taken from.
func Upstream() string {
	raw, err := files.ReadFile("upstream.commit")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
