# Tool surface for the agent

Every tool below is an existing Hikari operation. The agent gets a thin server
in front of `hikarid`'s socket: one tool per RPC, `--json` semantics, and a
`read_only` flag. The list is deliberately the launcher's own API rather than a
new one, so anything the agent does, the user could have done in the UI, and
anything the UI can do, the agent can too.

The tool server is the authority on paths and strings; the model only quotes what
a call returned.

## Observe: safe, changes nothing

| Tool | RPC | What it answers |
|---|---|---|
| `machine` | `daemon.status`, `engine.versions`, `mods.status` | daemon version and uptime, store engines, umu and wine versions, and whether the mod backend is ready. An unavailable backend now installs its own Python packages on first use, so the reason is either "setting up" or a sentence worth quoting |
| `library` | `library.list`, `library.get` | installed games, store, platform, install path, running state, play count, last played |
| `game_settings` | `game.settings.get` | every per-game knob: `wine_version`, `wine_prefix`, `target_exe`, `enable_fsr`, `max_sharpness`, `show_fps`, `show_mangohud`, `use_gamemode`, `gamescope`, `enable_hdr`, `enable_wine_wayland`, `enable_esync`, `enable_fsync`, `auto_install_dxvk*`, `nvidia_prime`, `environment`, `wrappers` |
| `launch_plan` | `game.launch` with `{dry_run: true}` | the exact argv and env that would run, including the merged `WINEDLLOVERRIDES`. The cheapest way to prove an environment point without launching |
| `game_log` | `game.log` | the last launch's output, including Proton and Wine messages |
| `logs` | `logs.tail` | the daemon's own log |
| `mods_status` | `mods.status` | backend ready or not, its Python, and where its source lives |
| `mods_games` | `mods.games` | every game the backend has a handler for, with `configured`, its profile count and its current profile. `configured` false means it has no game folder yet and will refuse mods |
| `mods_installed` | `mods.installed` `{game, profile?}` | the profile's mod list in load order, with enabled flags and separators. This is staging, not the game folder |
| `mods_health` | `mods.health` `{game, profile?, network?}` | the important one. Returns findings with an id, a severity, and a `fixable` flag. See the finding table below |
| `mods_overrides` | `mods.overrides` `{game}` | the Wine DLL overrides this game's mods need, as `name=value` pairs |
| `mods_missing` | `mods.missing` `{game}` | Nexus-listed requirements the profile does not satisfy |
| `mods_updates` | `mods.updates` `{game, profile?}` | enabled mods whose Nexus file is newer than the installed one: mod, installed version, latest version, file id, file name, url. Network call, needs a login |
| `mods_browse` | `mods.browse`, `mods.files` | search a game's mods on the source; a mod's files with versions and dates |
| `mods_downloads` | `mods.downloads` `{game, mod_id, file_id?}` | whether the browser's download has landed yet. On a free account this is how an install actually completes |
| `mods_account` | `mods.account` | who is signed in and whether they are premium |
| `opti_status` | `opti.detect`, `opti.status`, `opti.versions` | GPU, whether OptiScaler is installed for this game and which build |
| `report` | `logs.report` | the diagnostic bundle, for when the checks above come up empty |

## Health findings, exactly as the backend words them

The agent must not invent finding ids. These are the only ones that exist.

| Id | Severity | Fixable | Meaning |
|---|---|---|---|
| `not-deployed` | info | yes | the game folder holds none of this profile yet; Deploy writes it |
| `nothing-deployed` | error | yes | a deploy was recorded but not one loader file is in the game folder |
| `loader-not-deployed` | error | yes | part of a specific mod is missing from the game folder, so the game runs as if it were not installed |
| `mod-folder-missing` | error | no | a mod list row has no folder in staging; the row has to go or the mod be reinstalled. Ask first |
| `staging-other-filesystem` | warning | no | staging and the game folder are on different devices, so deploys copy instead of linking. A choice, not a fault |
| `overrides-missing` | warning | yes | the Wine prefix has no override for these DLLs; the launcher still sets them at launch |
| `loader-log` | info | no | the newest log the game wrote, with its timestamp. Use it to compare against the last launch |
| `loader-never-ran` | warning | no | this game has written no log at all, so nothing here can say whether its mods load |
| `requirement-missing` | warning | maybe | a Nexus requirement the profile does not satisfy |
| `requirements-unknown` | info | no | requirements could not be checked, usually no login |

`fixable: true` is the launcher saying a Repair (redeploy) is the fix. Do not
reach for Repair on anything else.

## Act: reversible, verify afterwards

| Tool | RPC | Reversal |
|---|---|---|
| `mods_configure` | `mods.configure` `{game, path, staging?, prefix?}` | point it elsewhere; there is nothing to undo for a first setup |
| `set_setting` | `game.settings.set` (whole settings object) | set the previous value back; record it first |
| `deploy` | `mods.deploy` | `restore` |
| `restore` | `mods.restore` | `deploy` |
| `repair` | `mods.repair` | none needed; it only redeploys, and only when health says that is the fix |
| `toggle_mod` | `mods.toggle` | toggle the same mod again |
| `install_mod` | `mods.install`: `{archive}`, `{url}`, or `{mod_id, file_id}` | `remove_mod` |
| `update_mod` | `mods.update` `{game, name}` | reinstall the previous file from `mods.files`. It keeps the load order slot and the enabled flag, and redeploys if the game was deployed |
| `install_optiscaler` | `opti.install` | `opti.uninstall` |
| `launch` | `game.launch` | `game.stop` |
| `stop` | `game.stop` | relaunch |

Installing does not deploy. A sequence of installs ends with one deploy, not one
per mod.

## Ask first: never autonomous

| Tool | RPC | Why a human has to say yes |
|---|---|---|
| `remove_mod` | `mods.remove` | deletes the mod's staging folder and its mod list row, and undeploys its files |
| `uninstall_game` | `game.uninstall` | deletes files the user paid for or spent hours downloading |
| `update_game` | `game.update` | can take an hour and can break a modded setup that was working |
| `wine_prefix_work` | anything rewriting `wine_prefix` | a new prefix loses every mod, save path and controller mapping inside it |
| `mods_setup` | `mods.setup` | installs Python packages into the launcher's own environment. It also runs by itself on first use, so only call it when it failed |
| `login`, `logout` | `mods.login`, `mods.logout` | account changes |

## Installing on a free Nexus account

A premium login can download a file directly. A free login cannot, so the flow is
different and the agent must not pretend otherwise:

1. `install_mod` with `{mod_id, file_id}` opens the mod page in the browser.
2. The user clicks the site's own download button.
3. `mods_downloads` tells you whether the file landed. Poll it a few times with a
   wait between, never in a tight loop, and say what you are waiting for.

An `nxm://` link the site hands back is the same flow with the link as input.

## Rules for the tool server

- One call returns JSON the model can quote. No prose summaries from the server;
  the model summarises, so its claims stay checkable.
- Long-running jobs (deploy, restore, install, update, repair) publish progress
  and a `done` event. The server waits for `done` and returns `{ok, error}`. It
  must never return when the job starts, or the agent verifies against a state
  that has not changed yet.
- Errors keep the launcher's wording: they are sentences a person can act on, and
  the model passes them through instead of paraphrasing them into something
  vague.
- The server refuses writes while a job is already running (`CodeBusy`). The
  model waits rather than retrying in a loop.
- Every write is logged with the previous value, so "undo what the agent did" is
  a lookup rather than an investigation.
- FOMOD installers currently take their own default selections. When a mod needs
  a choice, say so rather than silently installing defaults and calling it done.

## What the agent is not told about paths

Facts the prompt leaves out on purpose, because they belong to the tool server and
differ per machine:

- the socket path (`$XDG_RUNTIME_DIR/hikari.sock`, or `--socket`),
- where the daemon's log lives,
- the game install path,
- the prefix path,
- the mod staging root and the profile directory.

The server resolves paths; the model only ever quotes what a tool returned.
