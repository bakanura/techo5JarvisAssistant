#!/usr/bin/env python3
"""Download the pinned LineageOS zip and boot image for a Jarvis Show board into a cache outside the
repository, pick up the board's Amonet package from ~/Downloads, and print the install command.

    python3 tools/fetch-show-assets.py --board checkers
    python3 tools/fetch-show-assets.py --board crown --cache /srv/jarvis-show-assets
    python3 tools/fetch-show-assets.py --board checkers --check     # verify only, no download
    python3 tools/fetch-show-assets.py --board checkers --from ~/tmp # where a manual download went

The cache defaults to $JARVIS_SHOW_ASSETS, else ~/.cache/jarvis-show. These files are hundreds of MB, so a
folder inside the checkout is refused. A file is kept only when its size and SHA-256 match the pin in
tools/jarvis_crown/assets.py; one already there and right is not downloaded again.

Amonet zips are XDA attachments behind a login, so this never downloads them: log in, download the link
it prints, and run it again. A zip nobody has pinned yet is compared file by file with the public Amonet
source and then refused, so the report can be reviewed before its hash goes into assets.py.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sys
import urllib.error
import urllib.request
import zipfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from jarvis_crown.assets import AMONET_SOURCE, asset_path, assets_for, default_cache_dir  # noqa: E402
from jarvis_crown.recovery import TWRP_SHA256_BY_BOARD  # noqa: E402
from jarvis_crown.unlock import AMONET_UNLOCK_SHA256  # noqa: E402
from techo5lib import download_checked, fail, hash_file, note, repo_root, run_main, step  # noqa: E402


def inside_repo(path: Path) -> bool:
    root = Path(repo_root()).resolve()
    resolved = path.resolve()
    return resolved == root or root in resolved.parents


def verified(path: Path, asset) -> bool:
    return path.is_file() and path.stat().st_size == asset.size and hash_file(str(path)) == asset.sha256


def git_blob_sha(data: bytes) -> str:
    return hashlib.sha1(b'blob %d\0' % len(data) + data).hexdigest()


def github_blobs(repo: str, branches) -> dict:
    """{blob sha1: [(branch, path), ...]} for every file on the given branches of a public repo."""
    blobs = {}
    for branch in branches:
        url = 'https://api.github.com/repos/%s/git/trees/%s?recursive=1' % (repo, branch)
        req = urllib.request.Request(url, headers={'Accept': 'application/vnd.github+json'})
        with urllib.request.urlopen(req, timeout=30) as r:
            tree = json.load(r)
        for item in tree.get('tree', ()):
            if item.get('type') == 'blob':
                blobs.setdefault(item['sha'], []).append((branch, item['path']))
    return blobs


def github_history(repo: str, branches, path: str, blob: str):
    """(branch, commit) where an earlier revision of path had exactly this blob, else None."""
    for branch in branches:
        url = 'https://api.github.com/repos/%s/commits?sha=%s&path=%s&per_page=50' % (repo, branch, path)
        req = urllib.request.Request(url, headers={'Accept': 'application/vnd.github+json'})
        with urllib.request.urlopen(req, timeout=30) as r:
            commits = [c['sha'] for c in json.load(r)]
        for commit in commits:
            raw = 'https://raw.githubusercontent.com/%s/%s/%s' % (repo, commit, path)
            try:
                with urllib.request.urlopen(raw, timeout=30) as r:
                    data = r.read()
            except urllib.error.HTTPError:
                continue
            if git_blob_sha(data) == blob:
                return branch, commit
    return None


def installer_pins() -> dict:
    """{sha256: what the installer already pins it as}, so a package can be matched against them."""
    pins = {digest: 'unlock %s' % name for name, digest in AMONET_UNLOCK_SHA256.items()}
    pins.update({digest: '%s TWRP' % board for board, digest in TWRP_SHA256_BY_BOARD.items() if digest})
    return pins


def upstream_path(member: str) -> str:
    """amonet/modules/main.py -> modules/main.py; META-INF/... stays as it is."""
    parts = member.split('/')
    while parts and parts[0] in ('unlock', 'full', 'amonet'):
        parts = parts[1:]
    return '/'.join(parts)


def inspect_package(path: Path, asset, *, blobs_for=github_blobs, history_for=github_history) -> dict:
    """Report what in an unpinned zip is the public source (now or at an earlier commit), what differs from
    it, and what is a binary only the zip carries."""
    repo, branches = AMONET_SOURCE
    blobs = blobs_for(repo, branches)
    report = {'same': [], 'older': [], 'differs': [], 'binary': [], 'pinned': []}
    pins = installer_pins()
    with zipfile.ZipFile(path) as z:
        for info in z.infolist():
            if info.is_dir():
                continue
            data = z.read(info)
            if hashlib.sha256(data).hexdigest() in pins:
                report['pinned'].append((info.filename, pins[hashlib.sha256(data).hexdigest()]))
            hits = blobs.get(git_blob_sha(data), [])
            want = upstream_path(info.filename)
            known = any(p == want for paths in blobs.values() for _, p in paths)
            if hits:
                report['same'].append((info.filename, sorted({b for b, _ in hits})))
            elif known:
                old = history_for(repo, branches, want, git_blob_sha(data))
                if old:
                    report['older'].append((info.filename, old))
                else:
                    report['differs'].append(info.filename)
            else:
                report['binary'].append((info.filename, len(data), hashlib.sha256(data).hexdigest()))
    step('%s is not pinned yet; review before trusting it' % asset.name)
    note('sha256 %s  size %d' % (hash_file(str(path)), path.stat().st_size))
    note('%d files match %s exactly' % (len(report['same']), repo))
    for name, (branch, commit) in report['older']:
        note('older upstream copy: %s (as on %s at %s)' % (name, branch, commit[:10]))
    for name in report['differs']:
        note('DIFFERS from every upstream copy: %s' % name)
    for name, size, digest in report['binary']:
        note('only in the zip: %s (%d bytes, sha256 %s)' % (name, size, digest))
    for name, label in report['pinned']:
        note('same bytes the installer already pins (%s): %s' % (label, name))
    return report


def take_manual(asset, path: Path, downloads: Path, *, check_only: bool, blobs_for=github_blobs,
                history_for=github_history):
    """A login-only asset: cached and right, or copied in from the downloads folder when it matches its pin."""
    if asset.sha256 and verified(path, asset):
        note('%s: present and verified' % asset.name)
        return path
    found = downloads / asset.name
    if not found.is_file():
        note('%s: not here. Log in to XDA, download %s and run this again (looked in %s)'
             % (asset.name, asset.url, downloads))
        return None
    if not asset.sha256:
        inspect_package(found, asset, blobs_for=blobs_for, history_for=history_for)
        fail('%s has no pinned sha256; pin its sha256 and size in tools/jarvis_crown/assets.py only after '
             'reviewing the report above' % asset.name)
    if check_only:
        fail('%s is in %s but not in the cache; run without --check to take it' % (asset.name, downloads))
    if found.stat().st_size != asset.size:
        fail('%s in %s is not the pinned size; refusing it' % (asset.name, downloads))
    step('taking %s from %s' % (asset.name, downloads))
    path.parent.mkdir(parents=True, exist_ok=True)
    download_checked(found.resolve().as_uri(), str(path), asset.sha256)
    note('%s: verified' % asset.name)
    return path


def fetch_board(board: str, cache: Path, *, check_only: bool = False, downloads: Path | None = None,
                blobs_for=github_blobs, history_for=github_history) -> dict:
    if inside_repo(cache):
        fail('%s is inside the repository; keep these large files outside it (use --cache or '
             'JARVIS_SHOW_ASSETS)' % cache)
    downloads = downloads or Path.home() / 'Downloads'
    paths = {}
    for asset in assets_for(board):
        path = asset_path(cache, board, asset)
        if asset.manual:
            got = take_manual(asset, path, downloads, check_only=check_only, blobs_for=blobs_for,
                              history_for=history_for)
            if got is not None:
                paths[asset.kind] = got
            continue
        if len(asset.sha256) != 64:
            fail('%s has no pinned sha256 in this checkout' % asset.name)
        if verified(path, asset):
            note('%s: present and verified' % asset.name)
        elif check_only:
            fail('%s is missing or does not match its pin: %s' % (asset.name, path))
        else:
            step('downloading %s (%d MB) from %s' % (asset.name, asset.size >> 20, asset.source))
            path.parent.mkdir(parents=True, exist_ok=True)
            download_checked(asset.url, str(path), asset.sha256)
            if path.stat().st_size != asset.size:
                path.unlink()
                fail('%s has the right hash but not the pinned size; refusing it' % asset.name)
            note('%s: verified' % asset.name)
        paths[asset.kind] = path
    return paths


def main():
    p = argparse.ArgumentParser(prog='fetch-show-assets')
    p.add_argument('--board', required=True, choices=('crown', 'checkers'))
    p.add_argument('--cache', type=Path, default=None, help='default: $JARVIS_SHOW_ASSETS or ~/.cache/jarvis-show')
    p.add_argument('--from', dest='downloads', type=Path, default=None,
                   help='where login-only downloads (Amonet) went; default ~/Downloads')
    p.add_argument('--check', action='store_true', help='verify what is cached; download nothing')
    a = p.parse_args()
    cache = (a.cache or default_cache_dir()).expanduser()
    downloads = a.downloads.expanduser() if a.downloads else None
    paths = fetch_board(a.board, cache, check_only=a.check, downloads=downloads)
    print()
    print('lineage zip: %s' % paths['lineage'])
    print('boot image:  %s' % paths['boot'])
    if 'amonet' in paths:
        print('amonet zip:  %s' % paths['amonet'])
    print('use with: python3 tools/jarvis-show.py install --board %s --lineage-zip %s --boot-image %s ...'
          % (a.board, paths['lineage'], paths['boot']))


if __name__ == '__main__':
    run_main(main)
