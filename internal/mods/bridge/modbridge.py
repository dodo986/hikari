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

LOGIN_TIMEOUT = 300

FLAGS = {"--game", "--profile", "--archive", "--url", "--name", "--action",
         "--query", "--sort", "--page", "--mod-id", "--file-id"}


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
                                   "restore|install|nexus|search|files|"
                                   "account|installed> [--flag value]")
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
        "updated": getattr(info, "updated_at", "") or "",
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
    return {"mod": _entry(info), "files": [{
        "file_id": int(file.file_id),
        "name": file.name or "",
        "version": file.version or file.mod_version or "",
        "category": file.category_name or "",
        "size": _size(file.size_kb or 0),
        "primary": bool(file.is_primary),
    } for file in files]}


def _size(kb: int) -> str:
    if kb >= 1024 * 1024:
        return f"{kb / (1024.0 * 1024.0):.1f} GB"
    if kb >= 1024:
        return f"{kb / 1024.0:.1f} MB"
    return f"{kb} KB"


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


def cmd_installed(opts: dict) -> dict:
    game = _require(_bootstrap(), opts.get("game", ""))
    name, profile_dir = _profile(game, opts.get("profile", ""))
    from Utils.mods.modlist import read_modlist
    entries = read_modlist(profile_dir / "modlist.txt")
    return {"game": game.name, "profile": name, "mods": [
        {"name": entry.name, "enabled": bool(entry.enabled)}
        for entry in entries if not entry.is_separator]}


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
                "Nexus only hands direct download links to premium accounts; "
                "press Mod manager download on the mod page to send it to "
                "Hikari instead")
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
                          f"Amethyst could not install "
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
        log("Nexus API key stored where Amethyst keeps it.")
    elif action == "clear":
        clear_api_key()
        log("Nexus API key removed.")
    elif action != "status":
        raise BridgeError("usage", f"unknown nexus action '{action}'")
    return {"action": action, "stored": bool(load_api_key())}


COMMANDS = {
    "doctor": cmd_doctor,
    "games": cmd_games,
    "deploy": cmd_deploy,
    "restore": cmd_restore,
    "install": cmd_install,
    "nexus": cmd_nexus,
    "search": cmd_search,
    "files": cmd_files,
    "account": cmd_account,
    "login": cmd_login,
    "logout": cmd_logout,
    "installed": cmd_installed,
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
