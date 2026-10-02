#!/usr/bin/env python3
"""Jarvis Crown v1 one-installer entry point.

Only the non-destructive host preflight is enabled at this checkpoint. Later
jobs add Crown detection, unlock, recovery and install phases behind this gate.
"""
from __future__ import annotations

import argparse
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
from jarvis_crown.preflight import preflight_ok, print_checks, run_preflight  # noqa: E402


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="jarvis-crown")
    parser.add_argument("command", choices=["preflight"], nargs="?", default="preflight")
    parser.add_argument("--amonet-dir", type=Path, help="local amonet-crown-v2.0.1 package directory")
    parser.add_argument("--lineage-zip", type=Path, help="optional Crown LineageOS ZIP to validate host-side")
    parser.add_argument("--work-dir", type=Path)
    parser.add_argument("--backup-dir", type=Path)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = Path(__file__).resolve().parents[1]
    project = root.parent
    checks = run_preflight(
        repo_root=root,
        amonet_dir=(args.amonet_dir or (project / "third_party" / "amonet-crown-v2.0.1")).resolve(),
        lineage_zip=args.lineage_zip.resolve() if args.lineage_zip else None,
        work_dir=(args.work_dir or (project / "work")).resolve(),
        backup_dir=(args.backup_dir or (project / "backups")).resolve(),
    )
    print_checks(checks)
    if preflight_ok(checks):
        print("PASS: host preflight complete; no device was queried or modified")
        return 0
    print("FAIL: host preflight failed; no device was queried or modified", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
