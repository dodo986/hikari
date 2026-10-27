# Playbook — making one game run better on this machine

Performance work on Linux gaming is a search, not a list of flags. The machine
decides which knobs matter and the game decides which ones it even respects. The
playbook is therefore: measure, change one thing, measure the same way, keep or
revert, and keep a record of what this machine actually liked.

## 0. Establish the baseline before touching anything

Facts first, in this order:

1. `machine` — GPU, driver, Proton/Wine builds available, umu present.
2. `library` — the game, its store, its install path, whether it is native or
   Windows, and when it was last played.
3. `game_settings` — every knob and its current value. Write nothing down as
   "default" that you have not read: the user may already have tuned it, and
   their choice wins over your preference.
4. A measurement method, agreed with the user: `show_fps` (DXVK's own counter),
   `show_mangohud` (average, percentiles, frame time graph, GPU and CPU load),
   or nothing visible at all if they just want it to feel better. Without a
   number, "better" is an opinion and the next change cannot be judged.

Never propose a change in the same session where you have no baseline. "It
feels smoother" is not a measurement.

## 1. Choose the lever that fits the symptom

| Symptom | First lever | Why |
|---|---|---|
| Low average frame rate, GPU pegged | upscaling: `enable_fsr` (`max_sharpness`), or OptiScaler (`opti_status` then `install_optiscaler`) for DLSS/XeSS/FSR3 with frame generation | the cheapest large win, and the game's own scaler may be worse than a modern one |
| Stutter that fades after a few minutes | shader cache: let the first minutes run, check `DXVK_HUD=compiler`-style info via mangohud, and do not wipe the cache | it is compilation, not a bug; wiping makes it worse |
| Frame pacing bad, average fine | `use_gamemode`, `gamescope` (with its own fps limit), VRR status | limiting and pacing beats raw fps for feel |
| CPU-bound, GPU idle | check that the GPU is not in a power-saving state; `nvidia_prime` off for a single-GPU machine; `auto_install_dxvk_nvapi` only on NVIDIA | a wrong knob here costs frames for nothing |
| Crashes or driver errors under load | try another Proton build (`wine_version`), then `auto_install_dxvk`/`auto_install_vkd3d` | the build is the variable, not every setting at once |
| Tearing | VRR or a framerate limit rather than a compositor change | Wayland compositors handle this differently; ask before touching the session |
| HDR looks wrong | `enable_hdr` requires Wine Wayland: `enable_wine_wayland` first, then `enable_hdr` | the two are ordered; setting HDR alone does nothing |
| Input latency | frame limit above the refresh rate, then `enable_esync`/`enable_fsync` (ntsync on newer kernels) | measure with the frame time graph, not by feel |

## 2. One change, then the same measurement

The loop, and nothing else:

1. State the hypothesis in one sentence: "The frame drops after ten minutes are
   shader compilation; a frame-time graph will show spikes with flat GPU load."
2. Make the single change (`set_setting`, whole settings object, previous values
   recorded).
3. Launch and measure the same way as the baseline, in the same area of the game
   if possible. Same settings in-game too — a game's own preset changing
   mid-experiment invalidates everything.
4. Keep it or set the value back. Say which, and why, with the numbers.
5. If it helped, note it for this machine and this game (the vault/journal), so
   the next session does not rediscover it.

Two changes in one session is the limit for a *diagnosis*; more than three total
and the user stops being able to attribute the result, which is worse than no
change at all.

## 3. What not to do

- Do not install a new Proton build to "see if it helps" without asking: the
  download is large and a new prefix can lose a working modded setup.
- Do not enable everything that sounds fast (FSR + frame gen + gamemode + a
  frame limit) in one pass.
- Do not touch `wine_prefix` on a game with mods. It moves the game to a fresh
  prefix and every deployed mod, save path and controller layout inside it goes
  with it. If a user asks for it, that is a "here is what you will lose" answer.
- Do not recommend `PROTON_USE_WINED3D`, `WINE_FULLSCREEN_FSR` on a game that
  does not support it, or `DXVK_HUD` left on permanently.
- Do not claim a driver or kernel change as a fix unless the user themselves
  said they are willing to reboot.

## 4. When the honest answer is "this machine"

Some targets do not exist on some hardware: a game whose frame generation needs
NVIDIA's DLSS on an AMD card, HDR on a compositor that does not cooperate, a
particular anti-cheat under Proton. Say so once, with the reason, and offer the
best alternative instead of a chain of experiments that cannot succeed.

## 5. Report

Numbers with units and provenance, the single change that produced them, and the
undo. If nothing helped, say that too: "three settings tested, none moved the
average above noise" is a real result and it saves the user from trying the same
three.
