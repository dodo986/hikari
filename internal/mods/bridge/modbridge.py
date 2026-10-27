#!/usr/bin/env python3
"""Hikari's bridge to the vendored Amethyst backend.

One JSON event per line on fd 1, diagnostics on fd 2. The backend tree and the
native extension arrive in HIKARI_BACKEND_SRC and HIKARI_BACKEND_EXT; nothing
about this process is Amethyst's CLI contract.
"""

from __future__ import annotations

import importlib.util
import json
import os
import pathlib
import sys
import threading
import time
import traceback

sys.dont_write_bytecode = True  # the Amethyst tree is never written to

SRC = os.environ.get("HIKARI_BACKEND_SRC", "")
EXT = os.environ.get("HIKARI_BACKEND_EXT", "")

REQUIRED_MODULES = (
    "msgpack", "py7zr", "lz4", "bsdiff4", "keyring", "requests", "zstandard",
    "xxhash", "yaml", "PIL",
)

LOGIN_TIMEOUT = 300

FLAGS = {"--game", "--profile", "--archive", "--url", "--name", "--action",
         "--query", "--sort", "--page", "--mod-id", "--file-id", "--root",
         "--after", "--until", "--path", "--staging", "--prefix"}


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
        raise BridgeError("usage", "usage: modbridge.py <doctor|games|configure|"
                                   "deploy|restore|install|nexus|search|files|"
                                   "account|installed|health|lastlog>"
                                   " [--flag value]")
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


def _setup() -> None:
    if not SRC or not os.path.isfile(os.path.join(SRC, "app_bootstrap.py")):
        raise BridgeError(
            "config",
            "the modding backend was not unpacked; run `hikari mods setup`")
    if EXT:
        os.environ["AMETHYST_FILEGRAPH_EXTENSION"] = EXT
    if SRC not in sys.path:
        sys.path.insert(0, SRC)
    missing = [name for name in REQUIRED_MODULES
               if importlib.util.find_spec(name) is None]
    if missing:
        raise BridgeError(
            "unavailable",
            "the modding backend's Python packages are incomplete (" +
            ", ".join(missing) + "); run `hikari mods setup`")
    import app_bootstrap
    app_bootstrap.setup_environment(capture_diagnostics=False)


def _bootstrap() -> dict:
    _setup()
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
        raise BridgeError("notfound", f"no handler for '{key}'; `hikari mods games` lists the keys")
    if not game.is_configured():
        raise BridgeError(
            "unconfigured",
            f"{game.name} has no game folder yet; set it in Mods before installing anything")
    return game


def _profile(game, requested: str) -> tuple:
    """The profile to act on, creating the standard layout on first use."""
    name = requested or game.get_last_deployed_profile() or "default"
    path = game.get_profile_root() / "profiles" / name
    if not path.is_dir() and not requested:
        from Utils.profiles.structure import create_profile_structure
        create_profile_structure(game)
        name = game.get_last_deployed_profile() or "default"
        path = game.get_profile_root() / "profiles" / name
    if not path.is_dir():
        raise BridgeError("notfound", f"{game.name} has no profile '{name}'")
    return name, path


def _domain(game) -> str:
    try:
        return str(getattr(game, "nexus_game_domain", "") or "")
    except Exception:
        return ""


def _nexus_api():
    """Return (api, mode): Amethyst's own OAuth session if it has one, else the stored key."""
    _setup()
    from Nexus import nexus_oauth
    from Nexus.nexus_api import NexusAPI, load_api_key

    try:
        tokens = nexus_oauth.load_oauth_tokens()
    except Exception as exc:
        log(f"ignoring unreadable Nexus login: {_one_line(exc)}")
        tokens = None
    if tokens is not None:
        try:
            return NexusAPI.from_oauth(tokens), "oauth"
        except nexus_oauth.OAuthRefreshError as exc:
            if getattr(exc, "token_revoked", False):
                log("the Nexus login expired; sign in again")
                try:
                    nexus_oauth.clear_oauth_tokens()
                except Exception:
                    pass
            else:
                log(f"could not refresh the Nexus login: {_one_line(exc)}")

    key = load_api_key()
    if not key:
        raise BridgeError("config",
                          "not signed in to Nexus; run `hikari mods login`")
    return NexusAPI(key), "key"


def _nexus_game(games: dict, key: str):
    game = _require(games, key)
    domain = _domain(game)
    if not domain:
        raise BridgeError("unsupported",
                          f"{game.name} has no Nexus game domain")
    return game, domain


def _iso(stamp, text) -> str:
    """Nexus hands either an ISO string or a unix stamp; clients get one shape."""
    text = str(text or "")
    if text:
        return text
    try:
        return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(int(stamp or 0))) if stamp else ""
    except (TypeError, ValueError, OSError):
        return ""


def _entry(info) -> dict:
    return {
        "mod_id": int(getattr(info, "mod_id", 0) or 0),
        "name": info.name or "",
        "summary": info.summary or "",
        "author": info.author or getattr(info, "uploaded_by", "") or "",
        "version": info.version or "",
        "downloads": int(info.downloads_total or 0),
        "endorsements": int(info.endorsement_count or 0),
        "adult": bool(info.contains_adult_content),
        "updated": _iso(getattr(info, "updated_timestamp", 0), getattr(info, "updated_at", "")),
        "created": _iso(getattr(info, "created_timestamp", 0), getattr(info, "created_at", "")),
        "category": getattr(info, "category_name", "") or "",
        "picture_url": info.picture_url or "",
    }


SORTS = {"downloads": "downloads", "endorsements": "endorsements",
         "created": "createdAt", "updated": "updatedAt"}


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
        "nexus_domain": _domain(game),
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
    from Nexus.nexus_download import NexusDownloader
    api, _mode = _nexus_api()
    target = os.environ.get("HIKARI_MODS_DOWNLOADS", "")
    if not target:
        target = os.path.join(
            os.environ.get("XDG_CACHE_HOME") or
            os.path.join(os.path.expanduser("~"), ".cache"),
            "hikari", "mods", "downloads")
    os.makedirs(target, exist_ok=True)
    downloader = NexusDownloader(api, pathlib.Path(target))
    result = downloader.download_from_nxm(
        link, progress_cb=lambda done, total: progress(done, total))
    if not result.success or not result.file_path:
        raise BridgeError("failure", result.error or "the Nexus download failed")
    return str(result.file_path)


BROWSE_COUNT = 20


def cmd_search(opts: dict) -> dict:
    api, _ = _nexus_api()
    game, domain = _nexus_game(_bootstrap(), opts.get("game", ""))
    page = max(0, int(opts.get("page", "0") or 0))
    sort = SORTS.get(opts.get("sort", ""), "downloads")
    query = (opts.get("query", "") or "").strip()
    if query.isdigit():
        entries = api.search_mod_by_id(domain, int(query))
    elif query:
        entries = api.search_mods(domain, query, count=BROWSE_COUNT,
                                  offset=page * BROWSE_COUNT, sort_key=sort)
    else:
        entries = api.get_top_mods(domain, count=BROWSE_COUNT,
                                   offset=page * BROWSE_COUNT, sort_key=sort)
    results = [_entry(entry) for entry in entries]
    log(f"{len(results)} mod(s) for {game.name}")
    return {"game": game.name, "domain": domain, "page": page,
            "more": len(results) == BROWSE_COUNT, "results": results}


def cmd_files(opts: dict) -> dict:
    api, _ = _nexus_api()
    game, domain = _nexus_game(_bootstrap(), opts.get("game", ""))
    try:
        mod_id = int(opts.get("mod-id", "0") or 0)
    except ValueError:
        raise BridgeError("usage", "files needs a numeric --mod-id")
    if mod_id <= 0:
        raise BridgeError("usage", "files needs --mod-id")
    info = api.get_mod(domain, mod_id)
    files = api.get_mod_files(domain, mod_id).files
    # Nexus lists files oldest first; anyone installing wants the newest build on top.
    files = sorted(files, key=lambda f: (int(f.uploaded_timestamp or 0), int(f.file_id or 0)),
                   reverse=True)
    return {"mod": _entry(info), "files": [{
        "file_id": int(file.file_id),
        "name": file.name or "",
        "version": file.version or file.mod_version or "",
        "category": file.category_name or "",
        "size": _size(file.size_kb or 0),
        "uploaded": _iso(file.uploaded_timestamp, ""),
        "primary": bool(file.is_primary),
    } for file in files]}


def _size(kb: int) -> str:
    if kb >= 1024 * 1024:
        return f"{kb / (1024.0 * 1024.0):.1f} GB"
    if kb >= 1024:
        return f"{kb / 1024.0:.1f} MB"
    return f"{kb} KB"


def cmd_downloads(opts: dict) -> dict:
    """Is one of the mod's files already sitting in the browser's folder?

    A free account installs by downloading the archive by hand; this tells the
    UI when that download has landed so it can install it."""
    api, _ = _nexus_api()
    game, domain = _nexus_game(_bootstrap(), opts.get("game", ""))
    mod_id = int(opts.get("mod-id", "0") or 0)
    file_id = int(opts.get("file-id", "0") or 0)
    if mod_id <= 0:
        raise BridgeError("usage", "downloads needs --mod-id")
    from Nexus.manual_download_watch import find_existing_archive
    files = api.get_mod_files(domain, mod_id).files
    if file_id > 0:
        wanted = [f for f in files if int(f.file_id or 0) == file_id]
        files = wanted or files
    hit = find_existing_archive(mod_id, files)
    return {"found": str(hit[0]) if hit else ""}


def cmd_account(_opts: dict) -> dict:
    try:
        api, mode = _nexus_api()
    except BridgeError as exc:
        if exc.kind == "config":
            return {"stored": False}
        raise
    try:
        user = api.validate()
    except Exception as exc:
        raise BridgeError("config", f"Nexus rejected the stored credentials: {_one_line(exc)}")
    return {"stored": True, "mode": mode, "user": getattr(user, "name", "") or "",
            "premium": bool(getattr(user, "is_premium", False))}


def cmd_login(_opts: dict) -> dict:
    """Amethyst's own browser login: PKCE, loopback callback, its token store."""
    _setup()
    from Nexus import nexus_oauth
    from Nexus.nexus_api import NexusAPI

    done = threading.Event()
    box = {}

    def on_url(url):
        emit({"t": "url", "u": url})

    def on_token(tokens):
        box["tokens"] = tokens
        done.set()

    def on_error(message):
        box["error"] = message
        done.set()

    client = nexus_oauth.NexusOAuthClient(
        on_token, on_error, lambda m: log(m), on_authorization_url=on_url)
    log("waiting for you to authorise Hikari on the Nexus page")
    client.start()
    if not done.wait(LOGIN_TIMEOUT):
        client.cancel()
        raise BridgeError("auth", "Nexus did not answer in five minutes; start the login again")
    if "error" in box:
        raise BridgeError("auth", str(box["error"]))
    tokens = box.get("tokens") or nexus_oauth.load_oauth_tokens()
    if tokens is None:
        raise BridgeError("auth", "the login finished without tokens")
    try:
        user = NexusAPI.from_oauth(tokens).validate(bypass_cache=True)
    except Exception as exc:
        raise BridgeError("auth", f"signed in, but Nexus refused the new session: {_one_line(exc)}")
    return {"stored": True, "mode": "oauth", "user": getattr(user, "name", "") or "",
            "premium": bool(getattr(user, "is_premium", False))}


def cmd_logout(_opts: dict) -> dict:
    _setup()
    from Nexus import nexus_oauth
    nexus_oauth.clear_oauth_tokens()
    return {"stored": False}


def _modlist(profile_dir):
    from Utils.mods.modlist import read_modlist, write_modlist
    path = profile_dir / "modlist.txt"
    return path, read_modlist(path), write_modlist


def _entry_named(entries, name: str):
    for entry in entries:
        if entry.name == name:
            return entry
    return None


def cmd_toggle(opts: dict) -> dict:
    """Flip one modlist row on or off; deploying is what applies it."""
    game = _require(_bootstrap(), opts.get("game", ""))
    _name, profile_dir = _profile(game, opts.get("profile", ""))
    wanted = opts.get("name", "") or ""
    path, entries, write_modlist = _modlist(profile_dir)
    entry = _entry_named(entries, wanted)
    if entry is None or entry.is_separator:
        raise BridgeError("notfound", f"the profile has no mod called '{wanted}'")
    if entry.locked:
        raise BridgeError("unsupported",
                          f"'{wanted}' is locked by its own manager and cannot be toggled")
    entry.enabled = not entry.enabled
    write_modlist(path, entries)
    state = "enabled" if entry.enabled else "disabled"
    log(f"{wanted} is {state}; deploy the profile to apply it")
    return {"name": wanted, "enabled": bool(entry.enabled)}


def cmd_remove(opts: dict) -> dict:
    """Delete a mod for real: deployed files, plugins, staging folder, catalog."""
    game = _require(_bootstrap(), opts.get("game", ""))
    _name, profile_dir = _profile(game, opts.get("profile", ""))
    wanted = opts.get("name", "") or ""
    game.set_active_profile_dir(profile_dir)
    game.load_paths()
    path, entries, write_modlist = _modlist(profile_dir)
    entry = _entry_named(entries, wanted)
    if entry is None:
        raise BridgeError("notfound", f"the profile has no mod called '{wanted}'")
    if entry.is_separator:
        raise BridgeError("unsupported",
                          "'%s' is a separator, not a mod" % wanted)
    from Utils.mods.remove import remove_mods
    log(f"removing {wanted}")
    remove_mods(game, profile_dir, [wanted], log_fn=log)
    write_modlist(path, [e for e in entries if e.name != wanted])
    log(f"{wanted} is gone; deploy the profile to update the game")
    return {"removed": wanted}


def cmd_overrides(opts: dict) -> dict:
    """Wine DLL overrides this game's mods need, straight from its handler.

    MelonLoader, script extenders and anything else shipping a proxy DLL in the
    game folder only loads under Wine when its name is forced to native."""
    games = _bootstrap()
    key = opts.get("game", "")
    game = _require(games, key)
    table = getattr(game, "wine_dll_overrides", None) or {}
    try:
        table = dict(table)
    except Exception:
        table = {}
    return {"overrides": {str(k): str(v) for k, v in table.items()}}


_LOADER_SUFFIXES = {".dll", ".asi", ".exe", ".so", ".dylib", ".esp", ".esm",
                    ".esl", ".bsa", ".ba2", ".pak", ".vpk", ".jar", ".lua"}


def _deployed_anywhere(root, data_root, rel):
    """Whether a staged file reached the game folder, wherever its handler puts it."""
    for base in (data_root, root):
        try:
            if (base / rel).is_file():
                return True
        except OSError:
            continue
    return False


def _loader_files(mod_dir, staging):
    """A mod's loaders: the files that decide whether it loads at all."""
    out = []
    for path in mod_dir.rglob("*"):
        if not path.is_file() or len(path.relative_to(mod_dir).parts) > 8:
            continue
        if {suffix.lower() for suffix in path.suffixes} & _LOADER_SUFFIXES:
            out.append(path.relative_to(mod_dir).as_posix())
    return out


_LOG_SKIP = {"content", "archive", "deriveddatacache", "packs", "steamapps",
             "node_modules", ".git", "__pycache__", "shadercache"}
_LOG_TAIL_BYTES = 1 << 18

# Display names for the loaders that are common enough to recognise; anything
# else is named after the folder that holds its log.
_LOG_LOADERS = (("archivexl", "ArchiveXL"), ("cyber_engine_tweaks", "CET"),
                ("redscript", "redscript"), ("red4ext", "RED4ext"),
                ("skse", "SKSE"), ("f4se", "F4SE"), ("nvse", "NVSE"),
                ("scriptextender", "the script extender"))


def _loader_name(path: pathlib.Path, root: pathlib.Path) -> str:
    """Which loader wrote a log, judging by the folder it sits in."""
    text = path.as_posix().lower()
    for needle, name in _LOG_LOADERS:
        if needle in text:
            return name
    try:
        return path.parent.relative_to(root).as_posix()
    except Exception:
        return path.parent.name


def _tail(path: pathlib.Path, count: int) -> list:
    """The last lines of a log, without reading a whole gigabyte of it."""
    try:
        with path.open("rb") as handle:
            handle.seek(0, os.SEEK_END)
            size = handle.tell()
            handle.seek(max(0, size - _LOG_TAIL_BYTES))
            raw = handle.read()
    except OSError:
        return []
    lines = raw.decode("utf-8", "replace").splitlines()
    if size > _LOG_TAIL_BYTES and lines:
        lines = lines[1:]
    return [line.rstrip() for line in lines[-count:]]


def _walk_logs(root, after=0.0, until=0.0):
    """Logs under a game folder, newest first, without crawling the whole game."""
    found, seen = [], 0
    for here, dirs, names in os.walk(root):
        seen += 1
        if seen > 60000:
            break
        dirs[:] = [d for d in dirs if d.lower() not in _LOG_SKIP]
        if len(pathlib.Path(here).relative_to(root).parts) >= 5:
            dirs[:] = []
        for name in names:
            if not name.endswith(".log"):
                continue
            path = pathlib.Path(here) / name
            try:
                stamp = path.stat().st_mtime
            except OSError:
                continue
            if stamp >= after and (until <= 0 or stamp <= until):
                found.append((stamp, path))
    found.sort(key=lambda item: item[0], reverse=True)
    return found


def _newest_log(root):
    """The newest log under a folder, so "did it even run" has an answer."""
    if not root.is_dir():
        return "", 0.0
    found = _walk_logs(root)
    if not found:
        return "", 0.0
    return found[0][1].as_posix(), found[0][0]


def cmd_lastlog(opts: dict) -> dict:
    """Every log one launch wrote, newest first, so a game that dies has a last word."""
    root = pathlib.Path(opts.get("root", "")).expanduser()
    if not root.is_dir():
        raise BridgeError("notfound", f"there is no folder at {root}")
    found = _walk_logs(root, float(opts.get("after") or 0),
                       float(opts.get("until") or 0))
    return {"logs": [{"path": path.as_posix(), "loader": _loader_name(path, root),
                      "when": stamp, "lines": _tail(path, 40)}
                     for stamp, path in found[:8]]}


def cmd_configure(opts: dict) -> dict:
    """Point a game at its folder and staging root, and lay out its profile."""
    games = _bootstrap()
    game = _find(games, opts.get("game", ""))
    if game is None:
        raise BridgeError("notfound", f"no handler for '{opts.get('game', '')}'")
    path = pathlib.Path(os.path.expanduser(opts.get("path", "")))
    if not opts.get("path") or not path.is_dir():
        raise BridgeError("usage", f"{path or 'that'} is not a folder; pass --path <game folder>")
    staging = opts.get("staging") or ""
    if not staging:
        base = os.environ.get("MOD_MANAGER_PROFILES_DIR") or str(pathlib.Path.home() / "Games" / "Amethyst")
        staging = str(pathlib.Path(base) / game.name)
    paths_file = game._paths_file
    data = {}
    if paths_file.is_file():
        try:
            data = json.loads(paths_file.read_text(encoding="utf-8"))
        except Exception:
            data = {}
    data.update({
        "game_path": str(path),
        "staging_path": str(pathlib.Path(os.path.expanduser(staging))),
        "deploy_mode": data.get("deploy_mode") or "hardlink",
    })
    for key, value in (("prefix_path_cleared", False), ("save_path_override", ""),
                       ("shortcut_appid", "")):
        data.setdefault(key, value)
    if opts.get("prefix"):
        data["prefix_path"] = str(pathlib.Path(os.path.expanduser(opts["prefix"])))
        data["prefix_path_cleared"] = False
    data.setdefault("prefix_path", "")
    paths_file.parent.mkdir(parents=True, exist_ok=True)
    paths_file.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
    game.load_paths()
    if not game.is_configured():
        from Utils.profiles.structure import create_profile_structure
        create_profile_structure(game)
    name, profile_dir = _profile(game, "")
    return {
        "game": game.name, "profile": name,
        "game_path": str(game.get_game_path() or ""),
        "staging": str(game.get_effective_mod_staging_path()),
        "profile_dir": str(profile_dir),
        "configured": bool(game.is_configured()),
        "problems": game.validate_install(),
    }


def cmd_health(opts: dict) -> dict:
    """What silently stops mods from loading, and whether a redeploy heals it."""
    game = _require(_bootstrap(), opts.get("game", ""))
    _name, profile_dir = _profile(game, opts.get("profile", ""))
    staging = pathlib.Path(game.get_effective_mod_staging_path())
    root = pathlib.Path(game.get_game_path() or "")
    data_root = pathlib.Path(game.get_mod_data_path() or root)
    path, entries, _write = _modlist(profile_dir)
    findings = []
    enabled = [e for e in entries if e.enabled and not e.is_separator]
    deploy_active = bool(game.get_deploy_active())
    if not deploy_active:
        findings.append({
            "id": "not-deployed", "severity": "info", "mod": "", "file": "",
            "fixable": True,
            "detail": "the game folder holds none of this profile yet; Deploy "
                      "writes the enabled mods into it",
        })
    try:
        if root.is_dir() and staging.is_dir() and \
                root.stat().st_dev != staging.stat().st_dev:
            findings.append({
                "id": "staging-other-filesystem", "severity": "warning",
                "mod": "", "file": str(staging), "fixable": False,
                "detail": "the staging folder and the game folder are on "
                          "different filesystems, so every deploy copies files "
                          "instead of linking them; move staging beside the game",
            })
    except OSError:
        pass

    for entry in entries:
        if entry.is_separator:
            continue
        if not (staging / entry.name).is_dir():
            findings.append({
                "id": "mod-folder-missing", "severity": "error",
                "mod": entry.name, "file": "", "fixable": False,
                "detail": "the modlist row has no folder in the staging root, "
                          "so nothing can be deployed for it",
            })

    deployed = 0
    for entry in enabled:
        mod_dir = staging / entry.name
        if not mod_dir.is_dir():
            continue
        for rel in _loader_files(mod_dir, staging):
            if not deploy_active:
                break
            deployed += 1
            if not _deployed_anywhere(root, data_root, rel):
                findings.append({
                    "id": "loader-not-deployed", "severity": "error",
                    "mod": entry.name, "file": rel, "fixable": True,
                    "detail": "part of this mod is missing from the game folder, "
                              "so the game runs as if it were not installed",
                })

    # Did the game get as far as writing anything? A game folder that has never
    # produced a log cannot say whether its mods load, whatever engine it runs.
    log_path, log_when = _newest_log(root)
    findings.append({
        "id": "loader-log" if log_path else "loader-never-ran",
        "severity": "info" if log_path else "warning", "mod": "",
        "file": log_path, "fixable": False,
        "detail": ("newest log the game wrote: " + _iso(log_when, "")) if log_path else
                  "this game has written no log at all, so nothing here can tell "
                  "whether its mods load — launch it once",
    })

    overrides = getattr(game, "wine_dll_overrides", None) or {}
    if overrides:
        try:
            from Utils.wine.dll_config import read_prefix_wine_dll_overrides
            prefix = pathlib.Path(game.get_prefix_path()) if game.get_prefix_path() else None
            present = {k.lower() for k in read_prefix_wine_dll_overrides(prefix)}
        except Exception as err:  # a missing prefix is not a mod problem
            log(f"wine prefix not readable: {err}")
            present = set()
        missing = [dll for dll in overrides if dll.lower() not in present]
        if missing:
            findings.append({
                "id": "overrides-missing", "severity": "warning", "mod": "",
                "file": "", "fixable": True,
                "detail": "the Wine prefix has no override for "
                          + ", ".join(sorted(missing))
                          + "; Hikari still sets them at launch",
            })

    if deploy_active and deployed == 0 and enabled:
        findings.append({
            "id": "nothing-deployed", "severity": "error", "mod": "",
            "file": "", "fixable": True,
            "detail": "not one loader file from this profile is in the game "
                      "folder, so the profile was never really deployed",
        })

    if opts.get("network"):
        try:
            api, _mode = _nexus_api()
            from Nexus.nexus_requirements import check_missing_requirements
            _key, domain = _nexus_game(_bootstrap(), opts.get("game", ""))
            for item in check_missing_requirements(api, staging, domain, progress_cb=log):
                findings.append({
                    "id": "requirement-missing", "severity": "warning",
                    "mod": item.mod_name, "file": "", "fixable": False,
                    "detail": "needs " + ", ".join(
                        f"{r.mod_name} [mod {r.mod_id}]" for r in item.missing),
                })
        except Exception as err:
            findings.append({
                "id": "requirements-unknown", "severity": "info", "mod": "",
                "file": "", "fixable": False,
                "detail": f"could not ask Nexus about requirements: {err}",
            })

    return {"findings": findings}


def cmd_missing(opts: dict) -> dict:
    """Nexus-listed requirements that the installed mods do not satisfy."""
    game = _require(_bootstrap(), opts.get("game", ""))
    _name, profile_dir = _profile(game, opts.get("profile", ""))
    api, _mode = _nexus_api()
    _key, domain = _nexus_game(_bootstrap(), opts.get("game", ""))
    from Nexus.nexus_requirements import check_missing_requirements
    staging = game.get_effective_mod_staging_path()
    found = check_missing_requirements(api, staging, domain, progress_cb=log)
    return {"missing": [{
        "mod": item.mod_name,
        "mod_id": int(item.mod_id or 0),
        "needs": [{
            "mod_id": int(req.mod_id or 0),
            "name": req.mod_name or "",
            "external": bool(req.is_external),
            "notes": req.notes or "",
        } for req in item.missing],
    } for item in found]}


def cmd_installed(opts: dict) -> dict:
    game = _require(_bootstrap(), opts.get("game", ""))
    name, profile_dir = _profile(game, opts.get("profile", ""))
    from Utils.mods.modlist import read_modlist
    entries = read_modlist(profile_dir / "modlist.txt")
    return {"game": game.name, "profile": name, "mods": [
        {"name": entry.name, "enabled": bool(entry.enabled)}
        for entry in entries if not entry.is_separator]}


def _file_page(domain: str, mod_id: int, file_id: int) -> str:
    """The page whose buttons mint a download key; free accounts need it."""
    return (f"https://www.nexusmods.com/{domain}/mods/{mod_id}"
            f"?tab=files&file_id={file_id}")


def _download_file(domain: str, mod_id: int, file_id: int) -> str:
    """Fetch one browsed file; Nexus hands direct links to premium accounts."""
    import requests
    api, _ = _nexus_api()
    info = api.get_file_info(domain, mod_id, file_id)
    try:
        links = api.get_download_links(domain, mod_id, file_id)
    except Exception as exc:
        text = _one_line(exc)
        if "403" in text or "premium" in text.lower():
            raise BridgeError(
                "unsupported",
                "Nexus hands direct download links to premium accounts only; "
                "press Mod manager download on "
                f"{_file_page(domain, mod_id, file_id)} and Hikari takes the "
                "link from there")
        raise
    if not links:
        raise BridgeError("failure", "Nexus returned no download mirror")
    target = os.environ.get("HIKARI_MODS_DOWNLOADS", "") or os.path.join(
        os.environ.get("XDG_CACHE_HOME") or
        os.path.join(os.path.expanduser("~"), ".cache"),
        "hikari", "mods", "downloads")
    os.makedirs(target, exist_ok=True)
    name = info.file_name or f"nexus-{mod_id}-{file_id}"
    path = os.path.join(target, name.replace("/", "_"))
    with requests.get(links[0].URI, stream=True, timeout=60) as resp:
        resp.raise_for_status()
        total = int(resp.headers.get("Content-Length") or 0)
        done = 0
        with open(path + ".part", "wb") as fh:
            for chunk in resp.iter_content(chunk_size=1 << 20):
                if not chunk:
                    continue
                fh.write(chunk)
                done += len(chunk)
                if total > 0:
                    progress(done, total)
    os.replace(path + ".part", path)
    return path


def cmd_install(opts: dict) -> dict:
    game = _require(_bootstrap(), opts.get("game", ""))
    name, profile_dir = _profile(game, opts.get("profile", ""))
    game.set_active_profile_dir(profile_dir)
    game.load_paths()
    archive = opts.get("archive", "")
    if opts.get("url"):
        archive = _download(opts["url"])
        log(f"Downloaded {os.path.basename(archive)}")
    elif opts.get("mod-id"):
        domain = _domain(game)
        if not domain:
            raise BridgeError("unsupported", f"{game.name} has no Nexus game domain")
        mod_id = int(opts.get("mod-id", "0") or 0)
        file_id = int(opts.get("file-id", "0") or 0)
        if mod_id <= 0 or file_id <= 0:
            raise BridgeError("usage", "install needs --mod-id and --file-id")
        archive = _download_file(domain, mod_id, file_id)
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
                          f"the backend could not install "
                          f"{os.path.basename(archive)}")
    return {"game": game.name, "profile": name, "mod": mod, "archive": archive}


def cmd_nexus(opts: dict) -> dict:
    _setup()
    from Nexus.nexus_api import clear_api_key, load_api_key, save_api_key
    action = opts.get("action", "status")
    if action == "set":
        # The key arrives in the environment so it never shows up in ps.
        key = os.environ.get("HIKARI_NEXUS_KEY", "").strip()
        if not key:
            raise BridgeError("usage", "nexus set needs an API key")
        save_api_key(key)
        log("Nexus API key stored.")
    elif action == "clear":
        clear_api_key()
        log("Nexus API key removed.")
    elif action != "status":
        raise BridgeError("usage", f"unknown nexus action '{action}'")
    return {"action": action, "stored": bool(load_api_key())}


COMMANDS = {
    "doctor": cmd_doctor,
    "configure": cmd_configure,
    "games": cmd_games,
    "deploy": cmd_deploy,
    "restore": cmd_restore,
    "install": cmd_install,
    "nexus": cmd_nexus,
    "search": cmd_search,
    "files": cmd_files,
    "downloads": cmd_downloads,
    "account": cmd_account,
    "login": cmd_login,
    "logout": cmd_logout,
    "installed": cmd_installed,
    "toggle": cmd_toggle,
    "remove": cmd_remove,
    "missing": cmd_missing,
    "health": cmd_health,
    "lastlog": cmd_lastlog,
    "overrides": cmd_overrides,
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
