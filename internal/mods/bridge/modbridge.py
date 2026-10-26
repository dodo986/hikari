#!/usr/bin/env python3
"""Hikari's bridge to the Amethyst modding backend.

One JSON event per line on fd 1, diagnostics on fd 2. Paths to Amethyst's
source tree and native extension arrive in HIKARI_AMETHYST_SRC and
HIKARI_AMETHYST_EXT; nothing about this process is Amethyst's CLI contract.
"""

from __future__ import annotations

import importlib.util
import json
import os
import sys
import threading
import traceback

sys.dont_write_bytecode = True  # the Amethyst tree is never written to

SRC = os.environ.get("HIKARI_AMETHYST_SRC", "")
EXT = os.environ.get("HIKARI_AMETHYST_EXT", "")

REQUIRED_MODULES = (
    "msgpack", "py7zr", "lz4", "bsdiff4", "keyring", "requests", "zstandard",
    "xxhash", "yaml", "PIL",
)

FLAGS = {"--game", "--profile", "--archive", "--url", "--name"}


class BridgeError(Exception):
    def __init__(self, kind: str, message: str) -> None:
        super().__init__(message)
        self.kind = kind


_out = None
_out_lock = threading.Lock()


def emit(event: dict) -> None:
    with _out_lock:
        try:
            _out.write(json.dumps(event, ensure_ascii=False) + "\n")
            _out.flush()
        except (BrokenPipeError, ValueError):
            os._exit(0)


def log(message) -> None:
    emit({"t": "log", "m": " ".join(str(message).split())})


def progress(current, total, message=None) -> None:
    emit({"t": "progress", "cur": int(current or 0),
          "total": int(total or 0), "m": str(message or "")})


def _one_line(exc: BaseException) -> str:
    text = " ".join(str(exc).split())
    return text or exc.__class__.__name__


def _parse(argv: list) -> tuple:
    if not argv:
        raise BridgeError("usage", "usage: modbridge.py <doctor|games|deploy|"
                                   "restore|install> [--flag value]")
    command, rest = argv[0], argv[1:]
    opts: dict = {}
    index = 0
    while index < len(rest):
        flag = rest[index]
        if flag not in FLAGS:
            raise BridgeError("usage", f"unknown option {flag}")
        if index + 1 >= len(rest):
            raise BridgeError("usage", f"{flag} needs a value")
        opts[flag[2:]] = rest[index + 1]
        index += 2
    return command, opts


def _bootstrap() -> dict:
    if not SRC or not os.path.isfile(os.path.join(SRC, "cli.py")):
        raise BridgeError(
            "config",
            "Amethyst's source tree was not found; set mods.source_dir in "
            "settings.toml to an Amethyst install")
    if EXT:
        os.environ["AMETHYST_FILEGRAPH_EXTENSION"] = EXT
    if SRC not in sys.path:
        sys.path.insert(0, SRC)
    missing = [name for name in REQUIRED_MODULES
               if importlib.util.find_spec(name) is None]
    if missing:
        raise BridgeError(
            "unavailable",
            "Amethyst's Python dependencies are incomplete (" +
            ", ".join(missing) + "); run `hikari mods setup`")
    import app_bootstrap
    app_bootstrap.setup_environment(capture_diagnostics=False)
    from Utils.games.discovery import discover_games
    return discover_games()


def _steam_ids(game) -> set:
    ids = {str(getattr(game, "steam_id", "") or "")}
    ids.update(str(value) for value in getattr(game, "alt_steam_ids", []) or ())
    ids.discard("")
    return ids


def _find(games: dict, key: str):
    wanted = key.lower()
    by_game_id = None
    by_steam_id = None
    for name, game in games.items():
        if name.lower() == wanted:
            return game
        if by_game_id is None and getattr(game, "game_id", "").lower() == wanted:
            by_game_id = game
        if by_steam_id is None and wanted in _steam_ids(game):
            by_steam_id = game
    return by_game_id if by_game_id is not None else by_steam_id


def _require(games: dict, key: str):
    game = _find(games, key or "")
    if game is None:
        raise BridgeError("notfound", f"Amethyst has no handler for '{key}'")
    if not game.is_configured():
        raise BridgeError(
            "unconfigured",
            f"{game.name} has no game path in Amethyst yet; set it in "
            "Amethyst ▸ Game ▸ Settings")
    return game


def _profile(game, requested: str) -> tuple:
    name = requested or game.get_last_deployed_profile() or "default"
    path = game.get_profile_root() / "profiles" / name
    if not path.is_dir():
        raise BridgeError("notfound", f"{game.name} has no profile '{name}'")
    return name, path


def _describe(game) -> dict:
    root = game.get_profile_root() / "profiles"
    try:
        path = str(game.get_game_path() or "")
    except Exception:
        path = ""
    return {
        "id": getattr(game, "game_id", "") or game.name,
        "name": game.name,
        "steam_app_id": str(getattr(game, "steam_id", "") or ""),
        "configured": bool(game.is_configured()),
        "path": path,
        "profiles": sorted(entry.name for entry in root.iterdir()
                           if entry.is_dir()) if root.is_dir() else [],
        "deployed_profile": game.get_last_deployed_profile() or "",
        "deploy_active": bool(game.get_deploy_active()),
    }


def cmd_doctor(_opts: dict) -> dict:
    report = {"python": "%d.%d.%d" % sys.version_info[:3], "src": SRC,
              "games": 0, "available": False, "reason": ""}
    try:
        report["games"] = len(_bootstrap())
        report["available"] = True
    except BridgeError as exc:
        report["reason"] = str(exc)
    except Exception as exc:
        report["reason"] = _one_line(exc)
    return report


def cmd_games(_opts: dict) -> dict:
    games = _bootstrap()
    return {"games": [_describe(game) for _, game in sorted(games.items())]}


def cmd_deploy(opts: dict) -> dict:
    game = _require(_bootstrap(), opts.get("game", ""))
    name, _ = _profile(game, opts.get("profile", ""))
    from Utils.deployment.pipeline import run_deploy_pipeline
    if not run_deploy_pipeline(game, name, log_fn=log, progress_fn=progress):
        raise BridgeError("failure",
                          f"deploying {game.name} / {name} failed; see the log")
    return {"game": game.name, "profile": name, "deployed": True}


def cmd_restore(opts: dict) -> dict:
    game = _require(_bootstrap(), opts.get("game", ""))
    from Utils.deployment import restore_root_folder_for_game
    from Utils.deployment.pipeline import (
        finalize_filegraph_recovery, prepare_restore_profile,
    )
    recovery = prepare_restore_profile(game, log_fn=log)
    if hasattr(game, "restore"):
        game.restore(log_fn=log)
    root_folder = game.get_effective_root_folder_path()
    game_root = game.get_game_path()
    if root_folder.is_dir() and game_root:
        restore_root_folder_for_game(game, root_folder_dir=root_folder,
                                     game_root=game_root, log_fn=log)
    finalize_filegraph_recovery(game, recovery, log_fn=log)
    game.clear_deploy_active()
    return {"game": game.name, "deployed": False}


def _download(url: str) -> str:
    from Nexus.nxm_handler import parse_nxm_url
    link, _collection = parse_nxm_url(url)
    if link is None:
        raise BridgeError("unsupported",
                          "only mod downloads are supported; install "
                          "collections from Amethyst itself")
    from Nexus.nexus_api import NexusAPI, load_api_key
    from Nexus.nexus_download import NexusDownloader
    key = load_api_key()
    if not key:
        raise BridgeError("config",
                          "no Nexus API key is stored; add one in Amethyst ▸ "
                          "Settings ▸ Nexus")
    target = os.environ.get("HIKARI_MODS_DOWNLOADS", "")
    if not target:
        target = os.path.join(
            os.environ.get("XDG_CACHE_HOME") or
            os.path.join(os.path.expanduser("~"), ".cache"),
            "hikari", "mods", "downloads")
    os.makedirs(target, exist_ok=True)
    downloader = NexusDownloader(NexusAPI(key), target)
    result = downloader.download_from_nxm(
        link, progress_cb=lambda done, total: progress(done, total))
    if not result.success or not result.file_path:
        raise BridgeError("failure", result.error or "the Nexus download failed")
    return str(result.file_path)


def cmd_install(opts: dict) -> dict:
    game = _require(_bootstrap(), opts.get("game", ""))
    name, profile_dir = _profile(game, opts.get("profile", ""))
    game.set_active_profile_dir(profile_dir)
    game.load_paths()
    archive = opts.get("archive", "")
    if opts.get("url"):
        archive = _download(opts["url"])
        log(f"Downloaded {os.path.basename(archive)}")
    if not archive or not os.path.isfile(archive):
        raise BridgeError("usage", "install needs an existing archive path or "
                                   "an nxm:// url")
    from Utils.mods.install import install_archive
    mod = install_archive(archive, game, profile_dir, log_fn=log,
                          progress_fn=progress,
                          preferred_name=opts.get("name", ""))
    if not mod:
        raise BridgeError("failure",
                          f"Amethyst could not install "
                          f"{os.path.basename(archive)}")
    return {"game": game.name, "profile": name, "mod": mod, "archive": archive}


COMMANDS = {
    "doctor": cmd_doctor,
    "games": cmd_games,
    "deploy": cmd_deploy,
    "restore": cmd_restore,
    "install": cmd_install,
}


def main(argv: list) -> int:
    global _out
    # Amethyst prints to stdout; the protocol keeps a private handle on fd 1 and
    # every other writer ends up on stderr.
    _out = os.fdopen(os.dup(1), "w", encoding="utf-8", buffering=1)
    os.dup2(2, 1)
    try:
        command, opts = _parse(argv)
        handler = COMMANDS.get(command)
        if handler is None:
            raise BridgeError("usage", f"unknown command {command}")
        data = handler(opts)
    except BridgeError as exc:
        emit({"t": "error", "m": str(exc), "kind": exc.kind})
        return 2 if exc.kind == "usage" else 1
    except BaseException as exc:
        traceback.print_exc()
        emit({"t": "error", "m": _one_line(exc), "kind": "failure"})
        return 1
    emit({"t": "result", "data": data})
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
