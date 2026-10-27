# Amethyst backend (vendored)

Hikari's modding backend is Amethyst Mod Manager's headless code, copied here and
compiled into `hikarid`. Nothing is fetched at runtime: an install of Hikari can
deploy, restore, install and repair mods on a machine that has never seen
Amethyst.

| | |
|---|---|
| Upstream | https://github.com/Amethyst-Mod-Manager/Amethyst-Mod-Manager |
| Snapshot | `upstream.commit` (hash and date) |
| License | GPL-3.0-or-later, same as Hikari, see `LICENSE` |
| Files | `src/` (Python), `native/amethyst_filegraph/` (Rust) |

## What is in the snapshot

`src/` is a byte exact copy of upstream `src/` for the headless trees:
`Utils/`, `Nexus/`, `Games/`, `Thunderstore/`, `LOOT/`, `wrappers/`,
`app_bootstrap.py` and `version.py`, including their data files.

Left out on purpose, because none of it is reachable from a headless caller:

- `gui_qt/`, `wizards_qt/`, `themes/`, `icons/`, `translations/`, `appimage/`
- `cli.py`, `run_qt.py`
- `meson.build` files and `__pycache__`
- `wrappers/ucrtbase.dll`, a Windows proxy DLL shipped for the Steam texture
  wrappers

## How it is used

`internal/mods/backend/embed.go` embeds both trees. On start the daemon writes
them to `<data>/mods/backend/` and the bridge process imports `Utils` and
`Nexus` from there. `HIKARI_MODS_SOURCE` points the bridge at a checkout
instead, which is how a new upstream revision is tried before it is vendored.

State does not live in Amethyst's directories: the bridge hands the backend
Hikari's own paths (see `internal/mods/state.go`).

## Syncing with upstream

```sh
scripts/vendor-amethyst.sh ~/path/to/Amethyst-Mod-Manager
git diff --stat -- internal/mods/backend
```

The script replaces `src/` and `native/` with a fresh snapshot, so the diff is
exactly what upstream changed, and it prints any package Amethyst needs that
`internal/mods/bridge/requirements.txt` is missing.

Do not edit the snapshot by hand. A fix that cannot wait for upstream goes in
`patches/` as a `.patch` applied by the vendor script and listed below, so the
next sync does not silently drop it.

## Local patches

None.
