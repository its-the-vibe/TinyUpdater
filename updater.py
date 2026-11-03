"""TinyUpdater: manifest-based OTA updater for MicroPython (Pico / Pico 2 W).

Usage from a boot sequence:

    import updater
    updater.check_for_updates("its-the-vibe/PicoPaper")

The host is read from ``secrets.TINY_UPDATER_HOST``. The manifest is fetched
over https; individual files are streamed over http in fixed-size chunks.
"""
import binascii
import json
import os

try:
    import urequests as requests
except ImportError:  # allow running on CPython / in tests
    import requests

CHUNK_SIZE = 1024
STAGING_PREFIX = "tinyupdater_"
VERSION_FILE = "tinyupdater_version.txt"


def _host():
    from secrets import TINY_UPDATER_HOST
    return TINY_UPDATER_HOST


def _bearer_token():
    try:
        from secrets import TINY_UPDATER_BEARER_TOKEN
        return TINY_UPDATER_BEARER_TOKEN
    except ImportError:
        return None


def _exists(path):
    try:
        os.stat(path)
        return True
    except OSError:
        return False


def _split(path):
    if "/" in path:
        head, _, tail = path.rpartition("/")
        return head, tail
    return "", path


def _staging_path(path):
    head, tail = _split(path)
    name = STAGING_PREFIX + tail
    return head + "/" + name if head else name


def _validate_name(path):
    # Refuse anything that could escape the project directory.
    if not path or path.startswith("/") or ".." in path.split("/"):
        raise ValueError("unsafe file name: %r" % (path,))


def _ensure_dir(path):
    head, _ = _split(path)
    current = ""
    for part in head.split("/") if head else []:
        current = current + "/" + part if current else part
        if not _exists(current):
            os.mkdir(current)


def file_crc32(path):
    """Return the CRC32 of a file as a lowercase 8-digit hex string."""
    crc = 0
    with open(path, "rb") as f:
        while True:
            chunk = f.read(CHUNK_SIZE)
            if not chunk:
                break
            crc = binascii.crc32(chunk, crc)
    return format(crc & 0xFFFFFFFF, "08x")


def get_local_version():
    """Return the stored version string, or None if not recorded."""
    try:
        with open(VERSION_FILE, "r") as f:
            return f.read().strip()
    except OSError:
        return None


def _save_version(version):
    tmp = STAGING_PREFIX + "version.tmp"
    with open(tmp, "w") as f:
        f.write(version)
    if _exists(VERSION_FILE):
        os.remove(VERSION_FILE)
    os.rename(tmp, VERSION_FILE)


def fetch_manifest(repo, host=None):
    """Fetch and parse manifest.json for ``repo`` (https)."""
    host = host or _host()
    response = None
    headers = {}
    token = _bearer_token()
    if token:
        headers["Authorization"] = "Bearer " + token
    try:
        response = requests.get(
            "https://%s?repo=%s&file=manifest.json" % (host, repo),
            headers=headers,
        )
        if response.status_code != 200:
            raise OSError("manifest request failed: %s" % response.status_code)
        return json.loads(response.text)
    finally:
        if response is not None:
            response.close()


def download_file(repo, name, dest, host=None):
    """Stream ``name`` from the server into ``dest`` in fixed-size chunks (http)."""
    host = host or _host()
    response = None
    out = None
    headers = {}
    token = _bearer_token()
    if token:
        headers["Authorization"] = "Bearer " + token
    try:
        response = requests.get(
            "http://%s?repo=%s&file=%s" % (host, repo, name),
            headers=headers,
        )
        if response.status_code != 200:
            raise OSError("download of %s failed: %s" % (name, response.status_code))
        _ensure_dir(dest)
        out = open(dest, "wb")
        raw = response.raw
        while True:
            chunk = raw.read(CHUNK_SIZE)
            if not chunk:
                break
            out.write(chunk)
    finally:
        if out is not None:
            out.close()
        if response is not None:
            response.close()


def _cleanup(paths):
    for path in paths:
        try:
            os.remove(path)
        except OSError:
            pass


def check_for_updates(repo, host=None, reboot=True, log=print):
    """Check the manifest and apply an update if the version changed.

    Returns True if an update was applied, False otherwise (up to date, or
    the update was aborted; active files are untouched on failure).
    """
    try:
        manifest = fetch_manifest(repo, host)
        remote_version = manifest["version"]
        files = manifest["files"]
    except Exception as e:
        log("TinyUpdater: manifest check failed:", e)
        return False

    if remote_version == get_local_version():
        log("TinyUpdater: up to date (%s)" % remote_version)
        return False

    staged = []  # (staging_path, production_path)
    try:
        for entry in files:
            name = entry["file"]
            expected = entry["crc32"].lower()
            _validate_name(name)
            # Skip files whose production copy already matches.
            if _exists(name) and file_crc32(name) == expected:
                continue
            staging = _staging_path(name)
            staged.append((staging, name))
            download_file(repo, name, staging, host)
            if file_crc32(staging) != expected:
                raise ValueError("CRC32 mismatch for %s" % name)
    except Exception as e:
        log("TinyUpdater: update aborted:", e)
        _cleanup([s for s, _ in staged])
        return False

    # All files verified: promote them.
    for staging, name in staged:
        if _exists(name):
            os.remove(name)
        os.rename(staging, name)
    _save_version(remote_version)
    log("TinyUpdater: updated to %s" % remote_version)

    if reboot:
        import machine
        machine.soft_reset()
    return True
