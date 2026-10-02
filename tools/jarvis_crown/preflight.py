#!/usr/bin/env python3
"""Host-only preflight for the Jarvis Crown installer.

This module deliberately performs no adb/fastboot/device I/O.  It only validates
that the host is safe and ready before later installer phases are allowed to
look at a Crown.
"""
from __future__ import annotations

from dataclasses import dataclass
import grp
import os
from pathlib import Path
import shutil
import subprocess
import sys
from typing import Callable

MIN_FREE_BYTES = 8 * 1024 * 1024 * 1024
REQUIRED_TOOLS = {
    "adb": "install Android platform-tools (adb)",
    "fastboot": "install Android platform-tools (fastboot)",
}
SERIAL_GROUPS = {"dialout", "uucp"}


@dataclass(frozen=True)
class Check:
    level: str
    name: str
    detail: str

    @property
    def failed(self) -> bool:
        return self.level == "FAIL"


def _group_names() -> set[str]:
    names: set[str] = set()
    if not hasattr(os, "getgroups"):
        return names
    for gid in os.getgroups():
        try:
            names.add(grp.getgrgid(gid).gr_name)
        except KeyError:
            pass
    return names


def _modemmanager_active(run: Callable = subprocess.run) -> bool:
    if shutil.which("systemctl") is None:
        return False
    result = run(
        ["systemctl", "is-active", "--quiet", "ModemManager"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    return result.returncode == 0


def _writable_dir(path: Path) -> tuple[bool, str]:
    try:
        path.mkdir(parents=True, exist_ok=True)
    except OSError as exc:
        return False, str(exc)
    if not path.is_dir():
        return False, "not a directory"
    if not os.access(path, os.W_OK | os.X_OK):
        return False, "not writable by this user"
    return True, "writable"


def run_preflight(
    *,
    repo_root: Path,
    amonet_dir: Path,
    work_dir: Path,
    backup_dir: Path,
    min_free_bytes: int = MIN_FREE_BYTES,
) -> list[Check]:
    """Return all host readiness checks without opening or querying a device."""
    checks: list[Check] = []

    if not sys.platform.startswith("linux"):
        checks.append(Check("FAIL", "host-os", "Jarvis Crown v1 installer supports Linux hosts only"))
    else:
        checks.append(Check("PASS", "host-os", "Linux host"))

    if sys.version_info < (3, 10):
        checks.append(Check("FAIL", "python", "Python 3.10 or newer is required"))
    else:
        checks.append(Check("PASS", "python", f"Python {sys.version_info[0]}.{sys.version_info[1]}"))

    for exe, hint in REQUIRED_TOOLS.items():
        found = shutil.which(exe)
        if found:
            checks.append(Check("PASS", f"tool-{exe}", found))
        else:
            checks.append(Check("FAIL", f"tool-{exe}", hint))

    if os.geteuid() == 0:
        checks.append(Check("WARN", "host-root", "running installer as root is unnecessary; prefer an unprivileged user with serial access"))
        checks.append(Check("PASS", "serial-permission", "root can open the USB serial device"))
    else:
        groups = _group_names()
        usable = sorted(groups & SERIAL_GROUPS)
        if usable:
            checks.append(Check("PASS", "serial-permission", "member of " + ", ".join(usable)))
        else:
            checks.append(Check(
                "FAIL",
                "serial-permission",
                "join dialout (or uucp) and log out/in before install; rescue console access is mandatory",
            ))

    if _modemmanager_active():
        checks.append(Check("FAIL", "modemmanager", "ModemManager is active and may seize /dev/ttyACM*; stop it for installation"))
    else:
        checks.append(Check("PASS", "modemmanager", "not active"))

    if not repo_root.is_dir():
        checks.append(Check("FAIL", "repo", f"repository missing: {repo_root}"))
    else:
        checks.append(Check("PASS", "repo", str(repo_root)))

    required_amonet = (
        amonet_dir / "amonet",
        amonet_dir / "amonet" / "bootrom-step.sh",
    )
    missing = [str(p) for p in required_amonet if not p.exists()]
    if missing:
        checks.append(Check("FAIL", "amonet", "local Crown Amonet bundle incomplete: " + ", ".join(missing)))
    else:
        checks.append(Check("PASS", "amonet", str(amonet_dir)))

    for label, path in (("work-dir", work_dir), ("backup-dir", backup_dir)):
        ok, detail = _writable_dir(path)
        checks.append(Check("PASS" if ok else "FAIL", label, f"{path}: {detail}"))

    try:
        free = shutil.disk_usage(work_dir).free
    except OSError as exc:
        checks.append(Check("FAIL", "disk-space", f"cannot inspect free space: {exc}"))
    else:
        gib = free / (1024 ** 3)
        if free < min_free_bytes:
            checks.append(Check("FAIL", "disk-space", f"{gib:.1f} GiB free; at least {min_free_bytes / (1024 ** 3):.0f} GiB required"))
        else:
            checks.append(Check("PASS", "disk-space", f"{gib:.1f} GiB free"))

    return checks


def print_checks(checks: list[Check]) -> None:
    for check in checks:
        print(f"{check.level}: {check.name}: {check.detail}")


def preflight_ok(checks: list[Check]) -> bool:
    return not any(check.failed for check in checks)
