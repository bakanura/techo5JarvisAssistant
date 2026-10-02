#!/usr/bin/env python3
"""Jarvis Show v1 installer front-end for Crown and Checkers."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
from jarvis_crown.boards import profile_for_board, profile_for_product  # noqa: E402
from jarvis_crown.device_gate import DeviceGateError, identify_show  # noqa: E402
from jarvis_crown.flow import FlowError, InstallInputs, run_install_flow  # noqa: E402
from jarvis_crown.preflight import preflight_ok, print_checks, run_preflight  # noqa: E402
from jarvis_crown.unlock import UnlockError, unlock_show  # noqa: E402

SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="jarvis-show")
    parser.add_argument("command", choices=["preflight", "identify", "unlock", "install", "wifi"], nargs="?", default="preflight")
    parser.add_argument("--board", choices=("crown", "checkers"), help="optional first-gen board cross-check; install/identify auto-detect by default")
    parser.add_argument("--amonet-dir", type=Path)
    parser.add_argument("--amonet-hashes", type=Path, help="trusted JSON SHA-256 map for a board whose Amonet bytes are not built-in")
    parser.add_argument("--twrp-sha256", help="trusted board-specific TWRP SHA-256 when not built-in")
    parser.add_argument("--lineage-zip", type=Path)
    parser.add_argument("--work-dir", type=Path)
    parser.add_argument("--backup-dir", type=Path)
    parser.add_argument("--name")
    parser.add_argument("--boot-image", type=Path)
    parser.add_argument("--boot-sha256", help="trusted board-specific boot image SHA-256 when not built-in")
    parser.add_argument("--rootfs", type=Path)
    parser.add_argument("--rootfs-sha256")
    parser.add_argument("--wifi")
    parser.add_argument("--wifi-passphrase-file", type=Path)
    parser.add_argument("--serial", help="TECHO5 USB serial for offline Wi-Fi recovery; auto-selected when exactly one Show is attached")
    parser.add_argument("--ssh-key", type=Path)
    return parser.parse_args()


def load_hash_manifest(path: Path | None) -> dict[str, str] | None:
    if path is None:
        return None
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read Amonet hash manifest: {exc}") from exc
    if not isinstance(payload, dict) or not payload:
        raise ValueError("Amonet hash manifest must be a non-empty JSON object")
    result: dict[str, str] = {}
    for name, digest in payload.items():
        if not isinstance(name, str) or not name or not isinstance(digest, str) or not SHA256_RE.fullmatch(digest.lower()):
            raise ValueError(f"invalid Amonet hash manifest entry for {name!r}")
        result[name] = digest.lower()
    return result


def _validate_optional_sha(label: str, value: str | None) -> str | None:
    if value is None:
        return None
    normalized = value.strip().lower()
    if not SHA256_RE.fullmatch(normalized):
        raise ValueError(f"{label} must be exactly 64 hexadecimal characters")
    return normalized


def _print_identity(identity, profile) -> None:
    print(f"PASS: detected {profile.model}")
    print(f"PASS: product={identity.product}")
    print(f"PASS: fastboot serial={identity.serial}")
    print(f"PASS: unlock_status={'true' if identity.unlocked else 'false'}")


def _detect_profile(board: str | None):
    identity = identify_show(expected_board=board)
    profile = profile_for_product(identity.product)
    _print_identity(identity, profile)
    return identity, profile


def main() -> int:
    args = parse_args()
    root = Path(__file__).resolve().parents[1]
    project = root.parent
    work = (args.work_dir or (project / "work")).resolve()
    backups = (args.backup_dir or (project / "backups")).resolve()

    # Wi-Fi recovery runs against a booted Jarvis/TECHO5 USB serial console, not fastboot. Keep it
    # inside this one front-end so an offline device never depends on HA, mDNS, or a second workflow.
    if args.command == "wifi":
        if not args.wifi:
            print("FAIL: wifi recovery requires --wifi NETWORK", file=sys.stderr)
            return 1
        cmd = [sys.executable, str(root / "tools" / "show-wifi.py"), args.wifi]
        if args.serial:
            cmd += ["--serial", args.serial]
        if args.wifi_passphrase_file:
            cmd += ["--passphrase-file", str(args.wifi_passphrase_file.resolve())]
        return subprocess.call(cmd)

    # Identity is always established with read-only fastboot queries before a live-device command
    # chooses board-specific assets. --board, when supplied, is only a cross-check and can never make
    # the installer treat a different product as that board.
    if args.command == "identify":
        try:
            _detect_profile(args.board)
        except DeviceGateError as exc:
            print(f"FAIL: device identity gate: {exc}", file=sys.stderr)
            return 2
        return 0

    # Offline preflight may still be requested for a known board. Without --board it auto-detects the
    # attached first-generation Show, preserving the one-command installer experience.
    identity = None
    if args.board is not None and args.command == "preflight":
        profile = profile_for_board(args.board)
    else:
        try:
            identity, profile = _detect_profile(args.board)
        except DeviceGateError as exc:
            print(f"FAIL: device identity gate: {exc}", file=sys.stderr)
            return 2

    amonet = (args.amonet_dir or (project / "third_party" / profile.amonet_dir_name)).resolve()

    try:
        amonet_hashes = load_hash_manifest(args.amonet_hashes)
        twrp_sha256 = _validate_optional_sha("TWRP SHA-256", args.twrp_sha256)
        boot_sha256 = _validate_optional_sha("boot SHA-256", args.boot_sha256)
    except ValueError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1

    if args.command == "install":
        missing = []
        for name, value in (
            ("--lineage-zip", args.lineage_zip),
            ("--name", args.name),
            ("--boot-image", args.boot_image),
            ("--rootfs", args.rootfs),
            ("--rootfs-sha256", args.rootfs_sha256),
        ):
            if not value:
                missing.append(name)
        if missing:
            print("FAIL: install requires " + ", ".join(missing), file=sys.stderr)
            return 1
        try:
            rootfs_sha256 = _validate_optional_sha("rootfs SHA-256", args.rootfs_sha256)
            assert rootfs_sha256 is not None
        except (ValueError, AssertionError) as exc:
            print(f"FAIL: {exc}", file=sys.stderr)
            return 1

        def confirm_unlock(p):
            build = identity.lk_build_desc if identity is not None else None
            print(f"WARN: unlocking {p.model} runs the board-specific Amonet exploit.")
            print(f"WARN: detected LK build: {build or 'UNREADABLE'}")
            print("WARN: bootloader exploits inherently carry brick risk; keep mains power connected and do not interrupt writes.")
            return input(f"Type {p.unlock_confirmation} to continue: ").strip()

        def confirm_install(p):
            print("WARN: the next stage formats userdata for Lineage vendor staging, then converts system into the A/B slot store.")
            print("WARN: the verified recovery backup already exists; do not disconnect power/USB during these writes.")
            return input(f"Type {p.install_confirmation} to continue: ").strip()

        inputs = InstallInputs(
            board=profile.board,
            repo_root=root,
            amonet_dir=amonet,
            lineage_zip=args.lineage_zip.resolve(),
            work_dir=work,
            backups_dir=backups,
            name=args.name,
            boot_image=args.boot_image.resolve(),
            boot_sha256=boot_sha256,
            rootfs=args.rootfs.resolve(),
            rootfs_sha256=rootfs_sha256,
            twrp_sha256=twrp_sha256,
            amonet_hashes=amonet_hashes,
            wifi=args.wifi,
            wifi_passphrase_file=args.wifi_passphrase_file.resolve() if args.wifi_passphrase_file else None,
            ssh_key=args.ssh_key.resolve() if args.ssh_key else None,
            expected_fastboot_serial=identity.serial if identity is not None else None,
        )
        try:
            result = run_install_flow(
                inputs,
                confirm_unlock=confirm_unlock,
                confirm_install=confirm_install,
                progress=lambda stage: print(f"INFO: stage={stage}"),
            )
        except (FlowError, DeviceGateError, UnlockError, RuntimeError) as exc:
            print(f"FAIL: install stopped: {exc}", file=sys.stderr)
            return 4
        print(f"PASS: {result.profile.product_id} installation flow completed")
        return 0

    checks = run_preflight(
        repo_root=root,
        amonet_dir=amonet,
        lineage_zip=args.lineage_zip.resolve() if args.lineage_zip else None,
        work_dir=work,
        backup_dir=backups,
        board=profile.board,
    )
    print_checks(checks)
    if not preflight_ok(checks):
        print("FAIL: host/input preflight failed; no write was attempted", file=sys.stderr)
        return 1
    if args.command == "preflight":
        if identity is None:
            print(f"PASS: offline {profile.model} host/input preflight complete; no device was queried or modified")
        else:
            print(f"PASS: auto-detected {profile.model} host/input preflight complete; no device was modified")
        return 0

    # unlock reaches here only after the live target was auto-detected above.
    assert identity is not None
    if identity.unlocked:
        print(f"PASS: {profile.model} already unlocked; Amonet will not run")
        return 0

    print(f"WARN: the next stage intentionally runs Amonet for {profile.board}.")
    print(f"WARN: detected LK build: {identity.lk_build_desc or 'UNREADABLE'}")
    print("WARN: bootloader exploits inherently carry brick risk; keep mains power connected and do not interrupt writes.")
    confirmation = input(f"Type {profile.unlock_confirmation} to continue: ").strip()
    try:
        result = unlock_show(
            identity,
            amonet,
            confirmation=confirmation,
            expected_hashes=amonet_hashes,
        )
    except UnlockError as exc:
        print(f"FAIL: {profile.board} unlock: {exc}", file=sys.stderr)
        return 3
    if not result.identity.unlocked:
        print("FAIL: unlock result was not proven", file=sys.stderr)
        return 3
    print("PASS: unlock_status=true re-confirmed read-only after Amonet")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
