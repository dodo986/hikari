# Playbook — mods installed, but nothing happens

The symptom set: the mods list looks right, the game starts, no error anywhere,
and none of the mods are visibly active (no overlay, no menu entry, no changed
behaviour). Second variant: the game crashes or exits before a window appears
after mods were installed. Third: a mod worked last week and does not now.

The order below is the whole point. Each step is cheap, and each one rules out a
different layer. Never skip to "your mods are incompatible" — that conclusion is
almost never right and it costs the user a night.

## 1. Is anything actually in the game folder?

`mods_health`. It answers, per installed mod, whether the files that make the
mod load (the proxy DLLs and script-extender DLLs) are present in the game
folder.

- `loader-not-deployed` — the mod's loader file is missing from the game folder.
  This is the single most common cause. A deploy that was interrupted, or a
  deploy that trusted a stale index, leaves exactly this. **A redeploy is the
  fix**: `repair` (`mods.repair`), which only redeploys when the health check
  says that is the fix.
- `nothing-deployed` — not one loader file is there; the profile was never
  deployed for this game.
- `mod-folder-missing` — the modlist row points at a staging folder that no
  longer exists. Not fixable by deploying; the row has to go or the mod has to
  be reinstalled. Ask.

## 2. Does the loader itself get loaded?

`mods_health` also reports the newest log per loader (the script extender's log,
the framework's plugin logs).

- `loader-ok` with a timestamp **older than the last launch** — the files are
  there but the loader never ran. Go to step 3.
- `loader-not-run` — no log at all. Either the game has not been launched since
  the mod was installed, or the loader is not loading at all. Say which one you
  know: if `library` says the game was played after the deploy, then it is not
  loading.

## 3. Does the environment let the loader load?

On Linux the loader is a DLL beside the executable. Wine loads its *own* copy
unless the name is forced native, so the game starts with the mod silently
absent. The required table comes from the mod backend, per game:
`mods_overrides` (for example `winmm=native,builtin` and
`version=native,builtin` for Cyberpunk 2077).

Prove it without launching: `launch_plan` with `dry_run` returns the environment
the game will get. `WINEDLLOVERRIDES` must contain the entries from
`mods_overrides`. If it does not, the launcher is failing to merge them — that is
a launcher bug, report it rather than working around it by editing the prefix by
hand (a hand-edit is lost on the next deploy and confuses the next person).

Also in the plan, worth a glance: `target_exe` (launching a game's *pre-launcher*
instead of the executable often works but adds a window and a failure mode), and
the winetype (plain Wine versus Proton/Umu) against what the game needs.

## 4. Launch once and read the loader's own log

The logs are the ground truth; they name their own version and the game version
they hooked. Two lines matter: the version pair, and whether the hooks completed.
A loader log that stops halfway is a different problem from a loader that never
started, and both look identical from outside the game.

Read them through the paths `mods_health` gives. Quote the version line when you
report, so the user can see the pairing.

## 5. Only now, bisect

If the loaders are in place, loading, hooked, and the symptom is a crash or a
wrong game state, then a mod is the cause. Bisect by toggling, not by removing:

1. Settings/UI libraries first (they are the ones other mods depend on) stay
   enabled; the *content* mods get disabled.
2. Disable half the remaining mods, deploy, launch, look. `toggle_mod` keeps the
   modlist order intact and stays reversible at every step.
3. Narrow to one mod, then to a version: `mods_files` shows the other files the
   mod offers. An older file of the same mod is a legitimate fix and a much
   better answer than "remove the mod".

Two rules while bisecting: change the enabled set and nothing else, and tell the
user the plan and the number of launches before you start. A bisect is minutes
of their time and they should agree to spend it.

## 6. Dependencies and version pairing

- `mods_missing` (or `mods_health` with `network`) lists requirements Nexus
  knows about that the profile does not satisfy, including which mod needs what.
- A framework typically has a supported game-version range; the loader log
  states both versions, so a mismatch is a fact, not a guess. When they do not
  match, the fix is the right *file* of the framework from `mods_files`, not
  "disable the mod".

## 7. Report, and leave the setup better

Close the loop with: what was wrong (quoted evidence), what you changed, whether
a launch proved it, and the exact undo. If the cause was a launcher bug, say that
too — that is information the user cannot get anywhere else.

## Anti-patterns

- Concluding incompatibility before step 3. A missing `WINEDLLOVERRIDES` entry
  makes every mod in a game look broken and starts no error.
- Recommending a "clean reinstall" of the game. It destroys a working setup and
  rarely fixes a missing DLL.
- Editing files in the game folder by hand. The next deploy silently reverts it
  and the user learns to distrust deploys.
- Disabling the one mod that was working to make room for a new one, without
  saying that is what you are doing.
- Reporting "mods work" because the mod list is populated. Installed is not
  loaded; loaded is the loader writing a log; working is the user seeing it.
