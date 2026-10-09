#!/usr/bin/env python3
"""Download the pinned LineageOS zip and boot image for a Jarvis Show board into a cache outside the
repository, and print the install command that uses them.

    python3 tools/fetch-show-assets.py --board checkers
    python3 tools/fetch-show-assets.py --board crown --cache /srv/jarvis-show-assets
    python3 tools/fetch-show-assets.py --board checkers --check     # verify only, no download

The cache defaults to $JARVIS_SHOW_ASSETS, else ~/.cache/jarvis-show. These files are hundreds of MB, so a
folder inside the checkout is refused. A file is kept only when its size and SHA-256 match the pin in
tools/jarvis_crown/assets.py; one already there and right is not downloaded again.
"""
import argparse
import os
from pathlib import Path
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from jarvis_crown.assets import asset_path, assets_for, default_cache_dir  # noqa: E402
from techo5lib import download_checked, fail, hash_file, note, repo_root, run_main, step  # noqa: E402


def inside_repo(path: Path) -> bool:
    root = Path(repo_root()).resolve()
    resolved = path.resolve()
    return resolved == root or root in resolved.parents


def verified(path: Path, asset) -> bool:
    return path.is_file() and path.stat().st_size == asset.size and hash_file(str(path)) == asset.sha256


def fetch_board(board: str, cache: Path, *, check_only: bool = False) -> dict:
    if inside_repo(cache):
        fail('%s is inside the repository; keep these large files outside it (use --cache or '
             'JARVIS_SHOW_ASSETS)' % cache)
    paths = {}
    for asset in assets_for(board):
        if len(asset.sha256) != 64:
            fail('%s has no pinned sha256 in this checkout' % asset.name)
        path = asset_path(cache, board, asset)
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
    p.add_argument('--check', action='store_true', help='verify what is cached; download nothing')
    a = p.parse_args()
    cache = (a.cache or default_cache_dir()).expanduser()
    paths = fetch_board(a.board, cache, check_only=a.check)
    print()
    print('lineage zip: %s' % paths['lineage'])
    print('boot image:  %s' % paths['boot'])
    print('use with: python3 tools/jarvis-show.py install --board %s --lineage-zip %s --boot-image %s ...'
          % (a.board, paths['lineage'], paths['boot']))


if __name__ == '__main__':
    run_main(main)
