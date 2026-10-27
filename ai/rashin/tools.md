# Tool surface for the agent

Every tool below is an existing Hikari operation. The agent gets a thin server
in front of `hikarid`'s socket: one tool per RPC, `--json` semantics, and a
`read_only` flag. The list is deliberately the launcher's own API rather than a
new one, so anything the agent does, the user could have done in the UI, and
anything the UI can do, the agent can too.

Columns: **tool** (what the prompt calls it) · **RPC** · **side effect**.

## Observe — always safe, never changes anything

| Tool | RPC | What it answers |
|---|---|---|
| `machine` | `daemon.status`, `engine.versions`, `mods.status` | daemon version and uptime, store engines, umu/wine versions, whether the mod backend is ready |
| `library` | `library.list`, `library.get` | installed games, store, platform, install path, running state, play count, last played |
| `game_settings` | `game.settings.get` | every per-game knob: `wine_version`, `wine_prefix`, `target_exe`, `enable_fsr`, `max_sharpness`, `show_fps`, `show_mangohud`, `use_gamemode`, `gamescope`, `enable_hdr`, `enable_wine_wayland`, `enable_esync`, `enable_fsync`, `auto_install_dxvk*`, `nvidia_prime`, `environment`, `wrappers` |
| `launch_plan` | `game.launch` with `{dry_run: true}` | the exact argv and env that *would* run, including the merged `WINEDLLOVERRIDES` — the cheapest way to prove an environment fix without launching |
| `game_log` | `game.log` | the last launch's output, including Proton and Wine messages |
| `logs` | `logs.tail` | the daemon's own log |
| `mods_list` | `mods.games`, `mods.installed` | which games the mod backend handles, their profiles, the modlist in load order with enabled flags |
| `mods_health` | `mods.health` | the important one: loader files missing from the game folder, loader logs and their timestamps, dangling modlist rows, Wine overrides absent from the prefix. `{network: true}` adds Nexus requirement gaps |
| `mods_overrides` | `mods.overrides` | the Wine DLL overrides this game's mods need |
| `mods_missing` | `mods.missing` | Nexus-listed requirements the profile does not satisfy |
| `mods_files` | `mods.files`, `mods.browse` | a mod's files, versions and dates on the mod source |
| `opti_status` | `opti.detect`, `opti.status`, `opti.versions` | GPU, whether OptiScaler is installed for this game and which build |
| `report` | `logs.report` | the diagnostic bundle, for when the checks above come up empty |

## Act — reversible, still verify afterwards

| Tool | RPC | Reversal |
|---|---|---|
| `set_setting` | `game.settings.set` (send the whole settings object) | set the previous value back; record it first |
| `deploy` | `mods.deploy` | `mods.restore` |
| `restore` | `mods.restore` | `mods.deploy` |
| `repair` | `mods.repair` | none needed; it only redeploys, and only when the health check says a redeploy is the fix |
| `toggle_mod` | `mods.toggle` | toggle again |
| `install_mod` | `mods.install` (archive or `nxm://` link) | `mods.remove` |
| `install_optiscaler` | `opti.install` | `opti.uninstall` |
| `launch` | `game.launch` | `game.stop` |
| `stop` | `game.stop` | relaunch |

## Ask first — never autonomous

| Tool | RPC | Why it needs a human |
|---|---|---|
| `remove_mod` | `mods.remove` | deletes the staging folder and the modlist row |
| `uninstall_game` | `game.uninstall` | deletes files the user paid for or spent hours downloading |
| `update_game` | `game.update` | can take an hour and can break a modded setup that was working |
| `wine_prefix_work` | anything rewriting `wine_prefix` | a new prefix loses every mod, save path and controller mapping inside it |
| `mods_setup` | `mods.setup` | installs Python packages into the launcher's venv |

## Rules for the tool server

- One tool call returns JSON that the model can quote. No prose summaries from
  the server; the model does the summarising so its claims stay checkable.
- Long-running jobs (`deploy`, `install`, `repair`, `game.update`) publish
  progress and a `done` event. The server must wait for `done` and return
  `{ok, error}` — never return when the job *starts*, or the agent will verify
  against a state that has not changed yet.
- Errors keep the launcher's wording: they are written as sentences a person can
  act on, and the model should pass them through rather than paraphrasing them
  into something vague.
- The server refuses writes when a job of the same kind is already running
  (`CodeBusy`); the model is told to wait rather than retry in a loop.
- Every write is logged with the previous value, so "undo what the agent did" is
  a lookup rather than an investigation.

## What the agent should be told about paths

Facts the prompt intentionally leaves out, because they belong to the tool
server and differ per machine:

- the socket path (`$XDG_RUNTIME_DIR/hikari.sock`, or `--socket`),
- where the daemon's log lives,
- the game install path,
- the prefix path,
- the mod staging root.

The server resolves paths; the model only ever quotes what a tool returned.
