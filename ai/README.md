# Hikari + an agent: what it is for

Hikari already talks to everything an agent needs: a typed JSON-RPC daemon
(`hikarid`) that owns the library, the launch path, the Wine/Proton settings and
Amethyst's mod backend, plus every log those produce. That is the whole reason
an agent is worth wiring in — it is not there to chat, it is there to *close
loops a person would otherwise close by hand*.

This directory is the design and the prompt, not a finished integration. Today
it holds:

- [`rashin/SYSTEM_PROMPT.md`](rashin/SYSTEM_PROMPT.md) — the system prompt for
  the agent persona (Rashin, from Ryoku) when it works on a game launcher.
- [`rashin/tools.md`](rashin/tools.md) — the tool surface the prompt refers to,
  mapped to real daemon RPCs and CLI verbs, with the side effects spelled out.
- [`playbooks/mod-compatibility.md`](playbooks/mod-compatibility.md) — the loop
  that finds out why installed mods do nothing, and fixes the fixable half.
- [`playbooks/performance.md`](playbooks/performance.md) — the loop that
  measures and tunes a game for one specific machine.

## The two jobs worth automating

### 1. "I installed mods and nothing happened"

This is the single most common broken state in modded gaming on Linux, and it is
almost never the user's fault. In one real session on this machine the causes
were: a loader DLL that was never deployed, and `WINEDLLOVERRIDES` being unset so
Wine loaded its own `version.dll` instead of CET's. The game started, looked
perfectly normal, and zero mods loaded. Nothing crashed, nothing logged an
error, and no amount of staring at the mods list would reveal it.

An agent is good at exactly this shape of problem: many cheap, ordered checks
against ground truth (does the file exist? does the loader's log have a fresh
timestamp? does the prefix registry agree?) where each check is boring but the
*order* and the *interpretation* matter. `hikari mods health` is the machine
version of those checks; the agent is the part that decides what to do with a
finding, and what to ask.

What it must never do here is guess. A confident wrong answer ("your game
version is incompatible, downgrade CET") costs a user an evening. The playbook
is written to force evidence before conclusions.

### 2. "Make this game run better on *this* machine"

Optimization is a search problem with a per-machine objective and a very small
budget of user patience. Hikari exposes the knobs (`enable_fsr`, `show_fps`,
`show_mangohud`, `use_gamemode`, `enable_hdr`, `enable_wine_wayland`,
`enable_esync`/`enable_fsync`, `auto_install_dxvk`, `auto_install_dxvk_nvapi`,
`auto_install_vkd3d`, `nvidia_prime`, `gamescope`, `wrappers`, the Proton build
per game, OptiScaler through `opti.*`). What it does *not* expose is the
judgement: which knob matters for a 1080p RDNA2 card on Wayland, whether the
frame-time problem is shader compilation or a bad DXVK config, whether the user
even wants upscaling.

An agent turns that into: read the machine, propose one change, measure, keep or
revert, and remember the result. One variable at a time, always with a way back,
never more than a couple of changes in a session before reporting.

## Why this belongs in the launcher, not in a chat window

- **It has hands.** The agent's proposals are executable through the same
  daemon the UI uses, so "set FSR sharpness to 3 and turn on mangohud" is one
  RPC, not a paragraph of instructions for the user to retype.
- **It has eyes.** `game.log`, `logs.report`, `hikari mods health`, the loader
  logs under the game folder, the Proton prefix, `hikari engines`/`wines` — the
  evidence is already collected. An agent that can read it does not have to ask
  the user twenty questions.
- **It has memory.** Per-machine facts (GPU, compositor, Proton build, which
  game tolerates `PROTON_ENABLE_WAYLAND`) are worth keeping. Rashin's vault is
  the natural place: the launcher writes a `hikari.md` map, the agent appends a
  journal per game.
- **It can be bounded.** A launcher knows exactly what "safe" means per action:
  toggling a mod is reversible, deleting a mod is not; changing a settings key
  is reversible, uninstalling a game is not.

## Three ways to wire it, cheapest first

1. **Assistant over the existing socket (recommended first step).** The agent
   runs outside the shell, gets a small tool server that forwards to
   `hikarid` RPCs, and answers in the JobPanel or a dedicated page. No new
   process model in the launcher; the daemon stays the only writer.
2. **In-shell agent.** The Quickshell UI grows a "Fix this" button next to a
   problem, which runs one *playbook* with the agent choosing only from that
   playbook's tools. Smaller model, tighter scope, much easier to make safe.
3. **Full agent later.** A long-lived session with the whole tool surface and
   the vault, watching for drift (game updated, mod updated, GPU driver
   updated) and proposing maintenance. This is where Rashin's vault and journal
   pay off, and where the guard rails in the system prompt matter most.

## The contract the launcher has to keep

Whatever the level, the daemon must stay the authority:

- Every agent action is an RPC that a human could have clicked. No shell
  escapes, no hand-edited config, no `wine reg add` behind the launcher's back.
- Read-only calls first; writes only after the agent has stated what it will
  change and why, and only for things the playbook allows.
- Every write is verified by reading the state back, and every write has a
  documented way back (`mods.restore`, `game.settings.set` to the previous
  value, `game.stop`, re-deploy).
- Destructive actions (uninstall, remove a mod, delete a prefix, wipe a
  profile) are never autonomous.
- The agent's claims must quote the evidence it saw, so a human can check it.

## What to build next (concrete)

- `hikarid` gets an `agent.*` namespace that is *not* part of the normal UI
  surface: `agent.context` (one call returning machine facts, the game, its
  settings, deploy state, recent logs), `agent.propose` (validate a change
  without applying it), `agent.apply` (apply with the previous value recorded
  for one-click undo).
- A `hikari doctor` (whole-machine) that runs the mod playbook's health check
  plus the performance playbook's baseline read, and prints what an agent would
  be told.
- Evidence bundles: `hikari report` already collects diagnostics; an agent wants
  the same bundle as a JSON blob with stable field names.
- Undo journal: every agent write recorded with its inverse, so
  "undo everything the agent did today" is one call.
