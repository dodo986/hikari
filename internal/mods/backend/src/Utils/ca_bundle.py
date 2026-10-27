"""
ca_bundle.py
Resolve a usable TLS CA bundle for HTTPS requests.

Why this exists
---------------
The AppImage/Flatpak bundles its own ``certifi`` CA file, but a user's
environment can override or break it:
  * a stray ``SSL_CERT_FILE`` / ``REQUESTS_CA_BUNDLE`` pointing at a missing
    or stale file (the classic "curl works, app doesn't" SSLError);
  * the bundled certifi failing to extract / being unreadable on their mount.

``resolve_ca_bundle()`` returns the first CA file that actually exists, in
priority order: an explicitly-set (and valid) env override → bundled certifi →
the system trust store. Pass the result as ``verify=`` to ``requests``.
"""

from __future__ import annotations

import os

from Utils.app_log import app_log

# Common system trust stores across distros (Arch/SteamOS, Debian, Fedora, …)
_SYSTEM_CA_PATHS = (
    "/etc/ssl/certs/ca-certificates.crt",
    "/etc/pki/tls/certs/ca-bundle.crt",
    "/etc/ssl/ca-bundle.pem",
    "/etc/ssl/cert.pem",
)

_cached: str | None = None
_resolved: bool = False


def resolve_ca_bundle() -> str | None:
    """Return a path to a readable CA bundle, or None to use requests' default."""
    global _cached, _resolved
    if _resolved:
        return _cached

    _resolved = True
    chosen: str | None = None
    reason = ""

    # 1. Honour an explicit env override only if it actually points at a real file.
    for var in ("SSL_CERT_FILE", "REQUESTS_CA_BUNDLE"):
        val = os.environ.get(var)
        if val:
            if os.path.isfile(val) and os.access(val, os.R_OK):
                chosen, reason = val, f"env {var}"
                break
            app_log(f"ca_bundle: ignoring {var}={val!r} (file missing/unreadable)")

    # 2. Bundled certifi, if its cacert.pem is present and readable.
    if chosen is None:
        try:
            import certifi
            where = certifi.where()
            if os.path.isfile(where) and os.access(where, os.R_OK):
                chosen, reason = where, "certifi"
            else:
                app_log(f"ca_bundle: certifi bundle missing/unreadable at {where!r}")
        except Exception as exc:
            app_log(f"ca_bundle: certifi unavailable: {exc!r}")

    # 3. Fall back to the system trust store.
    if chosen is None:
        for p in _SYSTEM_CA_PATHS:
            if os.path.isfile(p) and os.access(p, os.R_OK):
                chosen, reason = p, "system store"
                break

    if chosen is None:
        app_log("ca_bundle: no usable CA bundle found; using requests' default")
    else:
        app_log(f"ca_bundle: using {reason} -> {chosen}")
    _cached = chosen
    return chosen


_ssl_ctx = None


def get_ssl_context():
    """Return a cached ``ssl.SSLContext`` built from the resolved CA bundle.

    Pass this as ``context=`` to ``urllib.request.urlopen`` so raw urllib
    downloads honour the same certifi → system-store fallback as ``requests``
    calls do via ``verify=resolve_ca_bundle()``. Without it, urllib uses
    Python's default trust store, which fails with
    ``CERTIFICATE_VERIFY_FAILED`` on machines with a broken/missing CA store
    or a stale ``SSL_CERT_FILE``.
    """
    global _ssl_ctx
    if _ssl_ctx is None:
        import ssl
        bundle = resolve_ca_bundle()
        _ssl_ctx = ssl.create_default_context(cafile=bundle) if bundle \
            else ssl.create_default_context()
    return _ssl_ctx


def download_file(url, dest, timeout=60, reporthook=None):
    """Download ``url`` to ``dest`` over HTTPS using the resolved CA bundle.

    Drop-in replacement for ``urllib.request.urlretrieve(url, dest,
    reporthook=...)`` that routes through :func:`get_ssl_context` so the
    download honours the same certifi → system-store fallback as ``requests``
    calls. Plain ``urlretrieve`` uses Python's default trust store and fails
    with ``CERTIFICATE_VERIFY_FAILED`` on machines with a broken/missing CA
    store or a stale ``SSL_CERT_FILE``.

    ``reporthook`` is called as ``reporthook(block_num, block_size,
    total_size)`` exactly like ``urlretrieve``. Writes to a ``.part`` temp
    file first so a partial download never leaves a corrupt file at ``dest``.
    """
    import urllib.request
    from pathlib import Path

    dest = Path(dest)
    dest.parent.mkdir(parents=True, exist_ok=True)
    tmp = dest.with_suffix(dest.suffix + ".part")
    block_size = 1024 * 64
    try:
        with urllib.request.urlopen(url, timeout=timeout,
                                    context=get_ssl_context()) as resp, \
                open(tmp, "wb") as out:
            total = int(resp.headers.get("Content-Length", 0) or 0)
            if reporthook is not None:
                reporthook(0, block_size, total)
            block_num = 0
            while True:
                chunk = resp.read(block_size)
                if not chunk:
                    break
                out.write(chunk)
                block_num += 1
                if reporthook is not None:
                    reporthook(block_num, block_size, total)
        tmp.replace(dest)
    finally:
        try:
            tmp.unlink()
        except OSError:
            pass
