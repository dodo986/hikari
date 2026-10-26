# Changelog

## Unreleased

- The Mods page drives the Amethyst Mod Manager's own backend as a
  supervised child process: list the games Amethyst can mod, pick a profile,
  deploy it, restore the game to vanilla, and install a mod archive (FOMOD
  installers take Amethyst's default selections) or an `nxm://` download —
  from the UI or with `hikari mods`. Amethyst owns the mod state; Hikari
  never keeps a second copy of it. `hikari mods setup` gives the backend its
  own Python environment under `$XDG_DATA_HOME/hikari/mods`.

## 0.1.2

- The daemon stages the release's own `hikari-update` into the install
  directory before running it, so the updater always knows the real install
  path even when it was executed from the extracted tree.

## 0.1.1

- Release tarballs are installable on their own: `install.sh` arrives in the
  archive and installs the binaries, UI, icon and desktop entry under the
  user prefix. `scripts/release.sh` verifies the built version, the expected
  archive contents and the checksum.
- Updates only run for a genuinely newer version, skip files that did not
  change, verify the installed binary and the restarted daemon report the
  release version, and roll back if either does not.
- The snapshot workflow skips a run when the commit is unchanged since the
  last successful snapshot.

## 0.1.3

- A single-file AppImage (`scripts/build-appimage.sh`) bundles the daemon,
  CLI and UI, updates itself in place, and is built by the release, snapshot
  and CI workflows alongside the tarball.
- The installer now delivers one app: `scripts/install-user.sh` builds the
  daemon/CLI, installs the launcher (`hikari-app`), the icon and the desktop
  entry, so Hikari appears in the app launcher and brings up the daemon and
  window on click.
- In-place updates from GitHub releases: an Update button under the library
  title appears only when a newer release exists; it verifies the sha256,
  replaces binaries and UI, restarts the daemon, reopens the window and rolls
  back if the new daemon does not start. Also `hikari update-check` and
  `hikari update-apply`, and `scripts/release.sh` to build release assets.

- Maintenance: shared atomic file writes, one canceled-install helper,
  stdlib replacements for hand-rolled helpers, and the GOG comet sidecar is
  actually attached to launches now.

- Add-game form redesigned: live cover preview with automatic image search
  (SteamGridDB when a key is configured, Steam otherwise), thumbnail picker,
  and native file browsing for the executable and cover.
- Popouts are window-level: they cover the rail, swallow presses and wheel,
  and nothing behind them can be clicked. They can be reopened.
- Buttons and switches in a popout use the same MouseArea input as the
  popout background, so Close, Browse, the launch options and the
  MangoHud/GameMode/FSR switches respond again.
- `scripts/test-ui.sh` hit-tests the popout offscreen (rail, page, padding,
  Close, body button, wheel) so the click-through regression is caught
  without a mouse.
- Local games: the executable decides the platform (PE/.exe → Windows,
  ELF/script → Linux), so a Windows program is no longer run as a Linux
  binary and killed by exec-format error.
- Launch failures explain themselves: the daemon keeps each game's last
  launch (state, exit code, error, output), the popout has a Launch log with
  Copy, and an instant crash opens it. `hikari launch` reports the reason
  and exits non-zero.
- Diagnostics users can reach: `~/.local/share/hikari/hikarid.log`,
  `hikari logs [--game STORE ID]` (works with the daemon stopped),
  `hikari report`, and a Diagnostics panel in Settings with Copy report.
- Pinned games now unpin: `library.get` returned the game without its pin,
  so the detail page always offered "Pin".
- The mouse wheel over an open popout no longer scrolls the page behind it.
- Filtering and sorting the grid are animated: games that leave a filter
  fade out and every other tile springs from its old cell to the new one
  (440 ms, overshoot). A–Z is strictly alphabetical, and "Playing first"
  now uses the last launch time (running sessions first), so it visibly
  reorders instead of doing nothing.
- Library and Store are separate sidebar pages: installed games in one,
  games the connected stores still offer in the other (same grid, search,
  chips and sort).
- The sidebar follows RyoStore's sidebar design (fold labels, rounded
  rows, Material Symbols, mono counters) and marks the current page with
  a square that animates between rows. The "Daemon connected" footer line
  is gone.
- Steam games show their own artwork again: the cache is read from both
  flat and hashed directories (portrait, capsule, header, hero), with the
  Steam CDN as the fallback.
- Downloads: the empty state is centred in the page instead of sitting at
  the top of an otherwise blank scroll area.
- Settings rounding is consistent: badges use the same corner radius as
  panels and buttons instead of being pills, and the redundant CONNECTED
  badge is gone.
- An About page in the sidebar: version, license, a GitHub link to
  @dodo986, and a short list of references (Heroic, Ryoku.Ui, OptiScaler).
- Launch the UI with `qs -n -c hikari`: a second instance used to open a
  second window, which could leave a stale UI on top.
- The UI reconnects by itself when the daemon restarts.
- Add your own games: "+ Add game" in the library (or `hikari add-local`),
  with a Linux/Windows choice; they launch directly or through the
  configured compatibility layer and can be removed again.
- Game settings and the launch command open in popout sheets, so the game
  page no longer scrolls; switches (MangoHud, GameMode, FSR, ...) and the
  compatibility picker now save the moment they change.
- Removed the blank block under the action bar on installed games.
- Wine/Proton manager: discovery (Hikari store, Steam, Lutris, system),
  one-click GE-Proton and Proton-CachyOS downloads with sha512 verification,
  removal, and a Settings panel.
- Per-game compatibility-layer picker, save-path sync (Epic), verify &
  repair, launch options (Epic/GOG, including DLC and alternative exe),
  and game pins that lead the library.
- Comet runs beside GOG games (sidecar, killed with the session) when the
  binary and the account name are available.
- Detail page sections stagger in; `nav game <store> <id>` deep-links a game
  detail over IPC; the detail page scrolls with the keyboard.

- First real install and launch verified: Fall Guys through legendary, then
  umu + Proton, then `game.stop` tearing the whole process group down.
- epic: legendary's numeric `install_size` no longer blanks the whole
  installed.json parse; sizes are rendered as "7.39 GiB" per the API.
- launch: prefixes are bootstrapped before the engine starts
  (`wineboot --init` or `umu-run createprefix`), inside the launch
  goroutine so a cold prefix cannot outlast the request.
- launch: the plan now carries the engine's own config path, so a launched
  legendary/gogdl/nile reads Hikari's state instead of the user's default.
- cli: `hikari install/update --follow` prints progress again (events read
  while waiting are forwarded to the printer).
- cli: `hikari settings <store> <id> --set ...` is honoured after the
  positionals, and `--set -` reads stdin.
- api: crash logs include the engine's last stderr lines.
- opti: OptiScaler v0.9.4 installed into a real game and confirmed loaded
  by the game process; `injection` accepts the documented short hook
  names (`dxgi`) as well as file names (`dxgi.dll`).
- launch: plain-wine launches now carry Heroic's library environment
  (`LD_LIBRARY_PATH`, `ORIG_LD_LIBRARY_PATH`,
  `GST_PLUGIN_SYSTEM_PATH_1_0`, `WINEDLLPATH`) from the wine build's own
  lib/lib64 trees; `prefer_system_libs` (default false) turns it off.
- engine: a normal engine exit no longer SIGTERMs the process group (it
  killed the game for plain-wine and proton-direct launches); cancel still
  tears the group down.
- launch: sessions survive the engine — when a stub launcher hands off and
  the engine exits, the game is found in the prefix (`WINEPREFIX` /
  `STEAM_COMPAT_DATA_PATH` in /proc) and the session stays `playing`
  until the game leaves or `game.stop` cancels it.
- launch: stopping a Proton prefix passes
  `STEAM_COMPAT_CLIENT_INSTALL_PATH`, without which
  `proton run wineboot -k` aborts; `game.stop` is idempotent when the
  game already exited.
- launch: a half-built prefix (registry present, no `syswow64`) is
  repaired before launch, which is what the first `c0000135` really was.
- api/cli: `game.launch` answers `{"state":"launching"}` (the game's pid
  only exists later, in the `game.state` event) and `hikari launch` prints
  that state instead of `{"plan": null}`.
- launch: plain-wine games can install DXVK, VKD3D-Proton and DXVK-NVAPI
  into their prefix (`auto_install_dxvk`, `auto_install_vkd3d`,
  `auto_install_dxvk_nvapi`): newest GitHub release, one extracted tree
  per version under `$XDG_DATA_HOME/hikari/tools`, DLLs in
  system32/syswow64 with `native,builtin` overrides, version marker in
  the prefix so later launches skip the download.
- launch: `eac_runtime`/`battleye_runtime` install the Proton
  anti-cheat runtimes from Lutris' list and point
  `PROTON_EAC_RUNTIME`/`PROTON_BATTLEYE_RUNTIME` at them; `enable_eos_overlay`
  installs and enables Epic's EOS overlay in the prefix (opt-in, Epic
  only).
- launch: an installed DXVK/VKD3D/NVAPI build is no longer reinstalled on
  every launch (the version marker is enough), and the release list is
  re-checked at most once a day, so launches stay offline.
- launch: stopping a game that never ran no longer spawns proton for a
  prefix that does not exist.
- cli: `hikari status` prints a human summary (identity, socket, engines,
  store sign-in) instead of raw JSON; `--json` keeps the raw reply.
- launch: each game gets its own wine prefix
  (`~/Games/Hikari/Prefixes/<slug>`) unless a prefix is set per game or as
  the global default; a shared default no longer puts every title in one
  wineserver.
- api: `game.settings.set` stores a `wine_prefix` equal to the game's
  computed default as unset, so saving a game never pins that default.
- ui: the game detail page scrolls under a fixed header (keyboard focus
  follows), loads once per game instead of on every library refresh, and
  its OptiScaler Remove button refreshes the status again.
- ui: a game with no artwork shows the placeholder instead of requesting
  `file:///`.
- ui: the library is an Epic-style cover grid with real gutters, store
  filters, counts and per-tile update/running/progress states; covers are
  clipped so a wide crop can never paint into the next cell.
- ui: the game detail page is a landscape key-art hero with a decisive
  action bar, launch-command plate and detail/runner/OptiScaler cards.
- auth: `auth.login.auto` opens a private Chromium window and completes the
  login from the authorization code in its final URL, for Epic, GOG and
  Amazon; without a Chromium-family browser the UI falls back to the
  default browser plus code entry.
- ui: the runner card exposes the six launch extras with applicability
  hints, and saves them from a draft object.
- ui: LAUNCHING / PLAYING / CRASHED are surfaced from `game.state`; Play is
  disabled while a launch is in flight and a crashed session stays visible.
- ui: Downloads and Settings use the same card language, with artwork,
  progress, engine readiness and a daemon summary.
- settings: the per-game entry can turn off a boolean inherited from factory
  defaults or a Heroic import (entries written through `game.settings.set`
  copy booleans exactly; legacy partial entries keep the OR merge).
  `hikari settings --set` merges its patch over the effective settings
  first, so a one-field CLI write stays complete.
- ui: a motion layer for the launcher: page rise/fade on navigation,
  staggered library covers, art zoom on hover, hero zoom on the detail
  page, press/hover scale on actions, and a turning refresh glyph.
- ui: Downloads and Settings cards stagger in through Ryoku's Entrance,
  and the rail's active marker animates instead of snapping.
- ui: a library Refresh button re-syncs every store; `library.refresh`
  without a store now continues past a logged-out store and returns an
  `errors` map, so one unavailable source no longer blocks the refresh.

## 0.1.0 - phase 1

- Audit of Heroic, OptiTux and Ryoku (read-only references; OptiTux has
  no license file, so no code reuse).
- hikarid daemon: JSON-RPC 2.0 over $XDG_RUNTIME_DIR/hikari.sock with
  install-progress, game-state, library and auth events.
- Read-only Heroic importer: discovers installed games from Heroic's own
  files and their runner settings; Heroic's directories are never written.
- legendary/gogdl/nile providers with isolated engine state, install and
  uninstall, and headless browser-based login flows.
- Launch pipeline: umu, Proton and plain Wine with the audited env
  assembly, wrapper chain and prefix handling; --dry-run prints the exact
  command and environment.
- OptiScaler integration contract with a fake implementation; no OptiTux
  code is used.
- hikari CLI covering the whole API, PKGBUILD and systemd user unit.
