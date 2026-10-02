#!/usr/bin/env python3
"""Jarvis Show v1 installer front-end for Crown and Checkers.

At this checkpoint the entry point exposes the safe host preflight, read-only
identity gate and Amonet unlock stages. J23 extends the same executable into the
full one-command install flow.
"""
from __future__ import annotations

import argparse
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
from jarvis_crown.boards import profile_for_board  # noqa: E402
from jarvis_crown.device_gate import DeviceGateError, identify_show  # noqa: E402
from jarvis_crown.preflight import preflight_ok, print_checks, run_preflight  # noqa: E402
from jarvis_crown.unlock import UnlockError, unlock_show  # noqa: E402


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="jarvis-show")
    parser.add_argument("command", choices=["preflight", "identify", "unlock"], nargs="?", default="preflight")
    parser.add_argument("--board", choices=("crown", "checkers"), required=True)
    parser.add_argument("--amonet-dir", type=Path)
    parser.add_argument("--lineage-zip", type=Path)
    parser.add_argument("--work-dir", type=Path)
    parser.add_argument("--backup-dir", type=Path)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    profile = profile_for_board(args.board)
    root = Path(__file__).resolve().parents[1]
    project = root.parent
    amonet = (args.amonet_dir or (project / "third_party" / profile.amonet_dir_name)).resolve()
    checks = run_preflight(
        repo_root=root,
        amonet_dir=amonet,
        lineage_zip=args.lineage_zip.resolve() if args.lineage_zip else None,
        work_dir=(args.work_dir or (project / "work")).resolve(),
        backup_dir=(args.backup_dir or (project / "backups")).resolve(),
        board=profile.board,
    )
    print_checks(checks)
    if not preflight_ok(checks):
        print("FAIL: host/input preflight failed; no device was queried or modified", file=sys.stderr)
        return 1
    if args.command == "preflight":
        print("PASS: host/input preflight complete; no device was queried or modified")
        return 0

    try:
        identity = identify_show(expected_board=profile.board)
    except DeviceGateError as exc:
        print(f"FAIL: {profile.board} identity gate: {exc}", file=sys.stderr)
        return 2
    print(f"PASS: product={identity.product}")
    print(f"PASS: fastboot serial={identity.serial}")
    print(f"PASS: unlock_status={'true' if identity.unlocked else 'false'}")
    if args.command == "identify":
        return 0
    if identity.unlocked:
        print(f"PASS: {profile.model} already unlocked; Amonet will not run")
        return 0

    print(f"WARN: the next stage intentionally runs Amonet for {profile.board}.")
    confirmation = input(f"Type {profile.unlock_confirmation} to continue: ").strip()
    try:
        result = unlock_show(identity, amonet, confirmation=confirmation)
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
