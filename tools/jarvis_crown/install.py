#!/usr/bin/env python3
"""Fail-closed J22 handoff into TECHO5's proven Crown slot-store installer."""
from __future__ import annotations

from dataclasses import dataclass
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import tarfile
from typing import Callable

from jarvis_crown.recovery import RecoveryError, verify_backup

FINAL_CONFIRM_PHRASE = "ERASE LINEAGE INSTALL JARVIS CROWN"
CROWN_BOOT_SHA256 = "cf5a492f7ee7ec16305905c58bf7f0b2e3f3e75521668ca905b9ae1ffb6d0baa"
ROOTFS_MARKER = "etc/jarvis-crown-release.json"
PROFILE_ID = "jarvis-crown-v1"
SUPPORTED_BOARD = "crown"
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


class InstallError(RuntimeError):
    """Raised before/around the destructive J22 install handoff."""


@dataclass(frozen=True)
class RootfsIdentity:
    product: str
    board: str
    version: str
    sha256: str


@dataclass(frozen=True)
class InstallPlan:
    argv: tuple[str, ...]
    rootfs: RootfsIdentity
    backup: Path


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def verify_crown_boot(path: Path) -> None:
    if not path.is_file():
        raise InstallError(f"Crown boot image missing: {path}")
    actual = _sha256(path)
    if actual != CROWN_BOOT_SHA256:
        raise InstallError(
            "Crown boot image is not the pinned known-good v0.8.0 image "
            f"(expected {CROWN_BOOT_SHA256}, got {actual})"
        )


def verify_jarvis_rootfs(path: Path, expected_sha256: str) -> RootfsIdentity:
    """Require exact caller-pinned bytes plus the Jarvis Crown product marker."""
    expected = expected_sha256.strip().lower()
    if not SHA256_RE.fullmatch(expected):
        raise InstallError("rootfs SHA-256 must be exactly 64 lowercase hex characters")
    if not path.is_file():
        raise InstallError(f"Jarvis Crown rootfs missing: {path}")
    actual = _sha256(path)
    if actual != expected:
        raise InstallError(f"rootfs SHA-256 mismatch (expected {expected}, got {actual})")

    try:
        with tarfile.open(path, mode="r:*") as archive:
            marker = None
            for member in archive.getmembers():
                normalized = member.name.lstrip("./")
                if normalized == ROOTFS_MARKER:
                    marker = member
                    break
            if marker is None or not marker.isfile() or marker.size > 4096:
                raise InstallError(f"rootfs does not contain a regular {ROOTFS_MARKER} marker")
            handle = archive.extractfile(marker)
            if handle is None:
                raise InstallError(f"cannot read {ROOTFS_MARKER} from rootfs")
            raw = handle.read(4097)
    except InstallError:
        raise
    except (OSError, tarfile.TarError) as exc:
        raise InstallError(f"rootfs is not a readable tar archive: {exc}") from exc

    try:
        payload = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise InstallError(f"invalid {ROOTFS_MARKER}: {exc}") from exc
    product = payload.get("product")
    board = payload.get("board")
    version = payload.get("version")
    if product != PROFILE_ID or board != SUPPORTED_BOARD:
        raise InstallError(
            f"rootfs marker is for product={product!r}, board={board!r}; "
            f"expected {PROFILE_ID!r}/{SUPPORTED_BOARD!r}"
        )
    if not isinstance(version, str) or not version.strip() or len(version) > 64:
        raise InstallError("rootfs marker has no valid Jarvis Crown version")
    return RootfsIdentity(product=product, board=board, version=version.strip(), sha256=actual)


def verify_install_backup(backup_root: Path, *, adb_serial: str) -> Path:
    """Require J20's complete atomic backup and bind it to this recovery serial."""
    path = backup_root / "partitions"
    try:
        verify_backup(path)
    except RecoveryError as exc:
        raise InstallError(f"J20 recovery backup is not complete/valid: {exc}") from exc
    try:
        manifest = json.loads((path / "manifest.json").read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise InstallError(f"cannot read J20 backup manifest: {exc}") from exc
    recorded = manifest.get("adb_serial")
    if recorded != adb_serial:
        raise InstallError(
            f"J20 backup belongs to adb serial {recorded!r}, not current recovery serial {adb_serial!r}"
        )
    return path


def make_install_plan(
    *,
    repo_root: Path,
    adb_serial: str,
    name: str,
    backup_root: Path,
    work_dir: Path,
    boot_image: Path,
    rootfs: Path,
    rootfs_sha256: str,
    confirmation: str,
    wifi: str | None = None,
    wifi_passphrase_file: Path | None = None,
    ssh_key: Path | None = None,
) -> InstallPlan:
    if confirmation != FINAL_CONFIRM_PHRASE:
        raise InstallError(f"final confirmation must be exactly: {FINAL_CONFIRM_PHRASE}")
    if not adb_serial.strip():
        raise InstallError("ADB recovery serial is required")
    clean_name = name.strip()
    if not clean_name or len(clean_name) > 64 or any(ord(ch) < 32 for ch in clean_name):
        raise InstallError("device name must be 1-64 printable characters")

    verify_crown_boot(boot_image)
    rootfs_identity = verify_jarvis_rootfs(rootfs, rootfs_sha256)
    backup = verify_install_backup(backup_root, adb_serial=adb_serial)

    installer = repo_root / "tools" / "install-show.py"
    if not installer.is_file():
        raise InstallError(f"upstream slot-store installer missing: {installer}")
    if backup_root.name != adb_serial:
        raise InstallError(
            f"backup root must be backups/<adb-serial>; got {backup_root} for {adb_serial}"
        )

    argv = [
        sys.executable,
        str(installer),
        "--serial", adb_serial,
        "--name", clean_name,
        "--boot", str(boot_image.resolve()),
        "--rootfs", str(rootfs.resolve()),
        "--jarvis-crown-prestaged",
        "--jarvis-crown-version", rootfs_identity.version,
        "--amazon-logo",
        "--force",
        "--backups", str(backup_root.parent.resolve()),
        "--work", str(work_dir.resolve()),
    ]
    if wifi:
        argv.extend(["--wifi", wifi])
        if wifi_passphrase_file:
            argv.extend(["--wifi-passphrase-file", str(wifi_passphrase_file.resolve())])
    elif wifi_passphrase_file:
        raise InstallError("--wifi-passphrase-file requires a Wi-Fi network name")
    if ssh_key:
        argv.extend(["--ssh-key", str(ssh_key.resolve())])

    # The Jarvis wrapper deliberately keeps the Amazon/kaeru expdb bytes unchanged and never asks the
    # generic installer to stage Lineage again.  These flags are release-blocking safety invariants.
    required = {"--jarvis-crown-prestaged", "--amazon-logo", "--force", "--boot", "--rootfs"}
    if not required.issubset(set(argv)) or "--lineage-zip" in argv:
        raise InstallError("internal error: unsafe Jarvis Crown install argument set")

    return InstallPlan(argv=tuple(argv), rootfs=rootfs_identity, backup=backup)


def execute_install(plan: InstallPlan, *, run: Callable = subprocess.run) -> None:
    """Execute the already-validated plan; no shell, no secret values in argv except file paths."""
    try:
        result = run(list(plan.argv), check=False)
    except OSError as exc:
        raise InstallError(f"cannot start TECHO5 slot-store installer: {exc}") from exc
    if result.returncode != 0:
        raise InstallError(f"TECHO5 slot-store installer failed with exit code {result.returncode}")
