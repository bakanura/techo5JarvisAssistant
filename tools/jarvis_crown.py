#!/usr/bin/env python3
"""Jarvis Crown v1 one-installer entry point.

Only the host-only ``preflight`` command exists at this checkpoint.  It deliberately
never talks to adb/fastboot devices; later installer jobs add the device gates and
write paths after this preflight has passed.
"""
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

MIN_PYTHON = (3, 10)
MIN_FREE_BYTES = 8 * 1024 ** 3
REQUIRED_TOOLS = {
    "adb": "Android platform tools",
    "fastboot": "Android platform tools",
    "git": "Git",
    "bash": "bash (required by the Crown Amonet package)",
    "timeout": "GNU coreutils timeout (required by Amonet fastbrick)",
}
AMONET_REQUIRED = (
    "fastbrick.sh",
    "profile.sh",
    "device.prop",
    "bin/fastbrick.img",
    "bin/twrp.img",
    "bin/crown-kaeru.bin",
    "bin/preloader.img",
    "bin/lk.bin",
)


@dataclass(frozen=True)
class Check:
    level: str
    message: str


def _existing_parent(path: Path) -> Path:
    path = path.expanduser().resolve(strict=False)
    while not path.exists() and path != path.parent:
        path = path.parent
    return path


def _default_amonet() -> Path:
    override = os.environ.get("JARVIS_CROWN_AMONET")
    if override:
        return Path(override).expanduser()
    project = Path(__file__).resolve().parents[2]
    return project / "third_party" / "amonet-crown-v2.0.1" / "amonet"


def _device_prop(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError:
        return values
    for raw in lines:
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def check_amonet_bundle(path: Path) -> list[Check]:
    out: list[Check] = []
    if not path.is_dir():
        return [Check("FAIL", f"Crown Amonet directory not found: {path}")]
    missing = [name for name in AMONET_REQUIRED if not (path / name).is_file()]
    if missing:
        out.append(Check("FAIL", "Amonet bundle incomplete; missing: " + ", ".join(missing)))
        return out
    props = _device_prop(path / "device.prop")
    if props.get("DEVICE", "").lower() != "crown":
        out.append(Check("FAIL", f"Amonet bundle is for {props.get('DEVICE', 'unknown')!r}, not crown"))
        return out
    out.append(Check("PASS", "Crown Amonet bundle structure and DEVICE=crown marker verified"))
    return out


def lineage_board(path: Path) -> str:
    try:
        with zipfile.ZipFile(path) as archive:
            metadata = archive.read("META-INF/com/android/metadata").decode("utf-8", "replace")
    except (OSError, KeyError, zipfile.BadZipFile) as exc:
        raise ValueError(f"not a usable LineageOS zip: {exc}") from exc
    for line in metadata.splitlines():
        if line.startswith("pre-device="):
            return line.split("=", 1)[1].strip()
    raise ValueError("LineageOS metadata has no pre-device entry")


def check_lineage_zip(path: Path | None) -> list[Check]:
    if path is None:
        return [Check("WARN", "no LineageOS ZIP supplied yet; J21 will require the Crown build before install")]
    if not path.is_file():
        return [Check("FAIL", f"LineageOS ZIP not found: {path}")]
    try:
        board = lineage_board(path)
    except ValueError as exc:
        return [Check("FAIL", f"{path}: {exc}")]
    if board.lower() != "crown":
        return [Check("FAIL", f"LineageOS ZIP declares pre-device={board}, not crown")]
    return [Check("PASS", "LineageOS ZIP metadata declares pre-device=crown")]


def check_serial_host() -> list[Check]:
    if os.geteuid() == 0:
        return [Check("WARN", "running as root; supported for recovery, but normal installs should use an unprivileged dialout/uucp user")]
    import grp
    group_names = set()
    for gid in os.getgroups():
        try:
            group_names.add(grp.getgrgid(gid).gr_name)
        except KeyError:
            pass
    out: list[Check] = []
    if group_names & {"dialout", "uucp"}:
        out.append(Check("PASS", "user is in dialout/uucp for the USB serial console"))
    else:
        out.append(Check("FAIL", "user is not in dialout/uucp; fix USB serial access before installing"))

    systemctl = shutil.which("systemctl")
    if systemctl:
        active = subprocess.run(
            [systemctl, "is-active", "--quiet", "ModemManager"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        ).returncode == 0
        if active:
            out.append(Check("FAIL", "ModemManager is running and may seize the Crown USB serial port"))
        else:
            out.append(Check("PASS", "ModemManager is not active"))
    else:
        out.append(Check("PASS", "systemctl unavailable; no ModemManager service detected through systemd"))
    return out


def check_tools() -> list[Check]:
    out: list[Check] = []
    for tool, hint in REQUIRED_TOOLS.items():
        found = shutil.which(tool)
        if not found:
            out.append(Check("FAIL", f"{tool} not found ({hint})"))
        else:
            out.append(Check("PASS", f"{tool}: {found}"))
    return out


def check_disk(paths: Iterable[Path]) -> list[Check]:
    out: list[Check] = []
    seen: set[int] = set()
    for requested in paths:
        base = _existing_parent(requested)
        if not os.access(base, os.W_OK):
            out.append(Check("FAIL", f"filesystem path is not writable for installer data: {base}"))
            continue
        try:
            stat = base.stat()
            usage = shutil.disk_usage(base)
        except OSError as exc:
            out.append(Check("FAIL", f"cannot inspect free space for {requested}: {exc}"))
            continue
        if stat.st_dev in seen:
            continue
        seen.add(stat.st_dev)
        gib = usage.free / 1024 ** 3
        if usage.free < MIN_FREE_BYTES:
            out.append(Check("FAIL", f"only {gib:.1f} GiB free on filesystem containing {requested}; require at least 8 GiB"))
        else:
            out.append(Check("PASS", f"{gib:.1f} GiB free on filesystem containing {requested}"))
    return out


def run_preflight(args: argparse.Namespace) -> int:
    checks: list[Check] = []
    if not sys.platform.startswith("linux"):
        checks.append(Check("FAIL", f"Jarvis Crown v1 one-installer currently supports Linux hosts only (found {sys.platform})"))
    else:
        checks.append(Check("PASS", "Linux host"))

    if sys.version_info < MIN_PYTHON:
        checks.append(Check("FAIL", f"Python {MIN_PYTHON[0]}.{MIN_PYTHON[1]}+ required"))
    else:
        checks.append(Check("PASS", f"Python {sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}"))

    checks.extend(check_tools())
    checks.extend(check_serial_host())
    checks.extend(check_disk((args.work, args.backups)))
    checks.extend(check_amonet_bundle(args.amonet))
    checks.extend(check_lineage_zip(args.lineage_zip))

    for item in checks:
        print(f"{item.level}: {item.message}")

    failed = any(item.level == "FAIL" for item in checks)
    if failed:
        print("FAIL: host preflight did not pass; no device command was executed")
        return 1
    print("PASS: host preflight passed; no device command was executed")
    return 0


def parser() -> argparse.ArgumentParser:
    ap = argparse.ArgumentParser(description="Jarvis Crown v1 one-installer")
    sub = ap.add_subparsers(dest="command", required=True)
    p = sub.add_parser("preflight", help="host-only checks; never contacts or modifies a device")
    p.add_argument("--amonet", type=Path, default=_default_amonet(), help="local Crown Amonet 'amonet' directory")
    p.add_argument("--lineage-zip", type=Path, help="optional Crown LineageOS 18.1 ZIP to validate host-side")
    root = Path(__file__).resolve().parents[1]
    p.add_argument("--work", type=Path, default=root / "build" / "jarvis-crown", help="installer work directory")
    p.add_argument("--backups", type=Path, default=root / "backups", help="device backup directory")
    p.set_defaults(func=run_preflight)
    return ap


def main() -> int:
    args = parser().parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
