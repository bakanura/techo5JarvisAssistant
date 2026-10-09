#!/usr/bin/env python3
"""Pinned download sources for the large per-board install assets.

The LineageOS zip (staged only for its vendor drivers) and the board boot image are hundreds of MB and
never live in the repository. They are fetched over HTTPS into a cache outside the checkout and kept only
when their size and SHA-256 match what is listed here. Boot hashes come from install.py, so the installer
and the fetcher cannot disagree about which image is trusted.
"""
from __future__ import annotations

from dataclasses import dataclass
import os
from pathlib import Path

from jarvis_crown.install import BOOT_SHA256_BY_BOARD


@dataclass(frozen=True)
class Asset:
    kind: str
    name: str
    url: str
    sha256: str
    size: int
    source: str


LINEAGE_RELEASES = "https://github.com/amazon-oss/releases/releases/download"
TECHO5_RELEASES = "https://github.com/HuskerMinion/techo5/releases/download"

# LineageOS 18.1 builds from amazon-oss/releases. Each sha256 is the one the release notes publish and
# the one GitHub reports as the asset digest. 20260904 is the first build whose kernel is
# 4.9.337-g8d928c5176cc, the ABI the Jarvis boot images and the vendor-module gate require.
ASSETS: dict[str, tuple[Asset, ...]] = {
    "crown": (
        Asset(
            "lineage", "lineage-18.1-20260904-UNOFFICIAL-crown.zip",
            f"{LINEAGE_RELEASES}/lineage-18.1-crown-v0.5/lineage-18.1-20260904-UNOFFICIAL-crown.zip",
            "a01359a5e13dad8ac24c5507e8a747163b992012410b14d2ca9a8711ec4a1491", 489922458,
            "amazon-oss/releases lineage-18.1-crown-v0.5",
        ),
        Asset(
            "boot", "techo5-boot-crown-v0.8.0.img",
            f"{TECHO5_RELEASES}/v0.8.0/techo5-boot-crown-v0.8.0.img",
            BOOT_SHA256_BY_BOARD["crown"] or "", 13154304,
            "HuskerMinion/techo5 v0.8.0",
        ),
    ),
    "checkers": (
        Asset(
            "lineage", "lineage-18.1-20260904-UNOFFICIAL-checkers.zip",
            f"{LINEAGE_RELEASES}/lineage-18.1-checkers-v0.7/lineage-18.1-20260904-UNOFFICIAL-checkers.zip",
            "785fa643fd68b2e6f6f02d96a2da58373c6a577b92a27cf6cec69603bb94068e", 485795474,
            "amazon-oss/releases lineage-18.1-checkers-v0.7",
        ),
        Asset(
            "boot", "techo5-boot-checkers-v1.0.1.img",
            f"{TECHO5_RELEASES}/v1.0.1/techo5-boot-checkers-v1.0.1.img",
            BOOT_SHA256_BY_BOARD["checkers"] or "", 13172736,
            "HuskerMinion/techo5 v1.0.1 (signed manifest)",
        ),
    ),
}


def default_cache_dir() -> Path:
    """$JARVIS_SHOW_ASSETS, else $XDG_CACHE_HOME/jarvis-show, else ~/.cache/jarvis-show."""
    explicit = os.environ.get("JARVIS_SHOW_ASSETS")
    if explicit:
        return Path(explicit).expanduser()
    base = os.environ.get("XDG_CACHE_HOME") or os.path.join(os.path.expanduser("~"), ".cache")
    return Path(base) / "jarvis-show"


def assets_for(board: str) -> tuple[Asset, ...]:
    try:
        return ASSETS[board]
    except KeyError:
        raise ValueError(f"no pinned assets for board {board!r}") from None


def asset_path(cache: Path, board: str, asset: Asset) -> Path:
    return cache / board / asset.name
