#!/usr/bin/env python3
"""Read-only live-device gate for Jarvis Crown v1.

This module may invoke fastboot read/query operations only. It never flashes,
boots, erases, reboots, unlocks, or writes device state.
"""
from __future__ import annotations

from dataclasses import dataclass
import re
import subprocess
from typing import Callable, Sequence

from jarvis_crown.boards import PROFILES, profile_for_board, profile_for_product, unsupported_product_message

FASTBOOT_TIMEOUT_SECONDS = 8
SUPPORTED_PRODUCTS = {profile.fastboot_product for profile in PROFILES.values()}
SUPPORTED_PRODUCT = "CROWN"  # compatibility alias for Crown-only callers/tests


class DeviceGateError(RuntimeError):
    """Raised when live device identity cannot be proven safe."""


@dataclass(frozen=True)
class DeviceIdentity:
    serial: str
    product: str
    unlocked: bool
    lk_build_desc: str | None


def _run(
    argv: Sequence[str],
    *,
    timeout: int = FASTBOOT_TIMEOUT_SECONDS,
    run: Callable = subprocess.run,
) -> subprocess.CompletedProcess:
    try:
        return run(
            list(argv),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        raise DeviceGateError(f"fastboot query timed out after {timeout}s: {' '.join(argv)}") from exc
    except OSError as exc:
        raise DeviceGateError(f"cannot execute fastboot query {' '.join(argv)}: {exc}") from exc


def _output(result: subprocess.CompletedProcess) -> str:
    return "\n".join(part for part in (result.stdout, result.stderr) if part)


def _require_success(result: subprocess.CompletedProcess, action: str) -> str:
    text = _output(result)
    if result.returncode != 0:
        summary = text.strip().replace("\n", " | ") or f"exit {result.returncode}"
        raise DeviceGateError(f"{action} failed: {summary}")
    return text


def fastboot_serials(*, run: Callable = subprocess.run) -> list[str]:
    result = _run(["fastboot", "devices"], run=run)
    text = _require_success(result, "fastboot devices")
    serials: list[str] = []
    for raw in text.splitlines():
        line = raw.strip()
        if not line:
            continue
        fields = line.split()
        if len(fields) < 2 or fields[1].lower() != "fastboot":
            continue
        serials.append(fields[0])
    return serials


def _getvar(serial: str, name: str, *, run: Callable = subprocess.run) -> str:
    result = _run(["fastboot", "-s", serial, "getvar", name], run=run)
    text = _require_success(result, f"fastboot getvar {name}")
    pattern = re.compile(rf"^\s*{re.escape(name)}\s*:\s*(.*?)\s*$", re.IGNORECASE)
    for raw in text.splitlines():
        match = pattern.match(raw)
        if match:
            value = match.group(1).strip()
            if value:
                return value
    raise DeviceGateError(f"fastboot getvar {name} returned no parseable {name}: value")


def identify_show(*, expected_board: str | None = None, run: Callable = subprocess.run) -> DeviceIdentity:
    """Prove exactly one connected fastboot device is a supported Jarvis Show.

    All operations in this function are read-only fastboot queries. When
    expected_board is supplied, a different supported board still fails closed.
    """
    serials = fastboot_serials(run=run)
    if not serials:
        raise DeviceGateError("no fastboot device detected")
    if len(serials) != 1:
        raise DeviceGateError(
            "exactly one fastboot device is required; detected: " + ", ".join(serials)
        )

    serial = serials[0]
    product = _getvar(serial, "product", run=run).upper()
    try:
        detected = profile_for_product(product)
    except ValueError as exc:
        raise DeviceGateError(unsupported_product_message(product)) from exc
    if expected_board is not None:
        expected = profile_for_board(expected_board)
        if detected.board != expected.board:
            raise DeviceGateError(
                f"connected product {product!r} is {detected.board}, expected {expected.board}"
            )

    raw_unlock = _getvar(serial, "unlock_status", run=run).strip().lower()
    if raw_unlock not in {"true", "false"}:
        raise DeviceGateError(
            f"unrecognized unlock_status {raw_unlock!r}; refusing to continue"
        )

    lk_build_desc: str | None
    try:
        lk_build_desc = _getvar(serial, "lk_build_desc", run=run)
    except DeviceGateError:
        # Already-unlocked units can continue to the TWRP layout gate without
        # LK metadata. Locked units are refused by unlock_show(): we require the
        # bootloader build to be readable before invoking the destructive Amonet
        # exploit, even though the pinned Crown bundle has a default payload.
        lk_build_desc = None

    return DeviceIdentity(
        serial=serial,
        product=product,
        unlocked=raw_unlock == "true",
        lk_build_desc=lk_build_desc,
    )


def identify_crown(*, run: Callable = subprocess.run) -> DeviceIdentity:
    """Compatibility wrapper requiring the Crown profile."""
    return identify_show(expected_board="crown", run=run)


def identify_checkers(*, run: Callable = subprocess.run) -> DeviceIdentity:
    """Require an Echo Show 5 1st gen / checkers target."""
    return identify_show(expected_board="checkers", run=run)
