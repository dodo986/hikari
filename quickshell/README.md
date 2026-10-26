# Hikari Quickshell UI

A standalone Quickshell app (`qs -c hikari`), built on Ryoku.Ui tokens and
primitives like the Hub. It is a pure client of the daemon: every fact on
screen comes over the JSON-RPC socket.

## Run

- Development (demo daemon + UI, nothing real touched):
  `scripts/dev.sh`
- Against the real daemon: `scripts/deploy.sh` once, then `qs -n -c hikari`
  (`-n` refuses a duplicate instance)
  (the packaged systemd unit should be enabled for the daemon).

## Keyboard and controller

Tab cycles rail → search → chips → grid; arrows move in the grid (or the
rail column); Enter activates; Escape backs out of a detail page. Ryoku's
shell maps controller input onto the same navigation keys (its evdev
key-press manager), so keyboard-complete means controller-complete.

Suggested keybind snippets (add to the compositor config you own, not to
Ryoku's files):

Hyprland (`~/Project2/ryoku` conventions: binds live in the user's Lua
module set):

```lua
-- hikari launcher (Super+G)
bind({"Super"}, "G", "togglespecialworkspace,hikari", function()
  hl.spawn("qs -n -c hikari")
end)
```

niri (KDL, in the user's niri config):

```kdl
// Mod+G opens Hikari
Mod+G { spawn "qs" "-c" "hikari"; }
```

## Layout

- `shell.qml` window + IPC deep links (`qs -c hikari ipc call nav open library`)
- `Hikari.qml` rail, page stack, offline plate
- `Singletons/Daemon.qml` the socket: JSON-RPC correlation, reconnect with
  retry, event dispatch, live library/auth/engines/downloads state
- `components/` shared artwork, badge, panel and large action primitives
- `pages/` Library (grid/search/chips), GameDetail (hero + detail cards),
  Downloads, Accounts (automatic browser login), Settings
- `pages/Tile.qml` one cover tile

Every colour, size and duration comes from `Tokens`; user-visible strings
are `I18n.tr` with English keys.
