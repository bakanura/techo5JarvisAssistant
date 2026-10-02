#!/usr/bin/env python3
"""Fail-closed Amonet unlock wrapper for Jarvis Crown v1."""
from __future__ import annotations

from dataclasses import dataclass
import hashlib
from pathlib import Path
import subprocess
import time
from typing import Callable, Mapping

from jarvis_crown.device_gate import DeviceGateError, DeviceIdentity, SUPPORTED_PRODUCT, identify_crown

CONFIRM_PHRASE = "UNLOCK CROWN"
FASTBRICK_TIMEOUT_SECONDS = 300
VERIFY_ATTEMPTS = 45
VERIFY_INTERVAL_SECONDS = 2.0

# Pinned from the user's known-good amonet-crown-v2.0.1 archive.
AMONET_UNLOCK_SHA256: dict[str, str] = {
    "fastbrick.sh": "aa93be2a752f8f4d4da31fa4bb733e9ce1dcdb9a55feda3795f52b95a9062ec5",
    "profile.sh": "844e584acc0bda882ff4c897cf2f55a5c9b8b9d913b9f5b7f897e6780371605a",
    "device.prop": "4313cbc0c8a75d418ec96890a317b89eae71bc78c7c92ace358000cb7d8d7bd5",
    "bin/fastbrick.img": "6b899c2919933b94a8c3e3454c22fe733d1a346d87c27640554a828aae9c6e9a",
    "bin/fastboot": "cb3d13b850143da85eb4b2099462514894459213da93d03d6ff4782d927d6e20",
    "bin/fastboot32": "8912e9926cfc45503ad960866501305a684462b1dd1a44c5d82b1f075d6dd8c1",
}


class UnlockError(RuntimeError):
    """Raised when unlock cannot be proven safe/successful."""


@dataclass(frozen=True)
class UnlockResult:
    identity: DeviceIdentity
    amonet_invoked: bool


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def verify_amonet_unlock_assets(
    amonet_dir: Path,
    *,
    expected_hashes: Mapping[str, str] = AMONET_UNLOCK_SHA256,
) -> None:
    root = amonet_dir / "amonet"
    for relative, expected in expected_hashes.items():
        path = root / relative
        if not path.is_file():
            raise UnlockError(f"required Amonet unlock asset missing: {relative}")
        actual = _sha256(path)
        if actual.lower() != expected.lower():
            raise UnlockError(
                f"Amonet unlock asset hash mismatch: {relative} "
                f"(expected {expected}, got {actual})"
            )


def _invoke_fastbrick(
    amonet_dir: Path,
    *,
    run: Callable = subprocess.run,
) -> None:
    root = amonet_dir / "amonet"
    try:
        result = run(
            ["bash", "fastbrick.sh"],
            cwd=root,
            input="YES\nx\n",
            text=True,
            timeout=FASTBRICK_TIMEOUT_SECONDS,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        raise UnlockError(
            "Amonet invocation timed out; device state is unknown. "
            "Do not retry blindly: rerun the read-only identity gate first."
        ) from exc
    except OSError as exc:
        raise UnlockError(f"cannot execute Amonet fastbrick: {exc}") from exc
    if result.returncode != 0:
        raise UnlockError(
            f"Amonet fastbrick exited with status {result.returncode}; "
            "device state must be re-identified before any retry"
        )


def _wait_for_unlocked(
    original: DeviceIdentity,
    *,
    identify: Callable[[], DeviceIdentity] = identify_crown,
    sleep: Callable[[float], None] = time.sleep,
    attempts: int = VERIFY_ATTEMPTS,
) -> DeviceIdentity:
    last_error: str | None = None
    for attempt in range(attempts):
        try:
            current = identify()
        except DeviceGateError as exc:
            last_error = str(exc)
        else:
            if current.serial != original.serial:
                raise UnlockError(
                    f"fastboot serial changed from {original.serial} to {current.serial}; "
                    "refusing to continue on an ambiguous device"
                )
            if current.product != SUPPORTED_PRODUCT:
                raise UnlockError(
                    f"post-unlock device reports {current.product}, not {SUPPORTED_PRODUCT}"
                )
            if current.unlocked:
                return current
            last_error = "Crown still reports unlock_status=false"
        if attempt + 1 < attempts:
            sleep(VERIFY_INTERVAL_SECONDS)
    raise UnlockError(
        "unlock could not be confirmed read-only after Amonet; "
        f"last state: {last_error or 'unknown'}"
    )


def unlock_crown(
    identity: DeviceIdentity,
    amonet_dir: Path,
    *,
    confirmation: str | None,
    run: Callable = subprocess.run,
    identify: Callable[[], DeviceIdentity] = identify_crown,
    sleep: Callable[[float], None] = time.sleep,
    expected_hashes: Mapping[str, str] = AMONET_UNLOCK_SHA256,
) -> UnlockResult:
    """Unlock a proven locked Crown, or safely skip an already-unlocked one."""
    if identity.product != SUPPORTED_PRODUCT:
        raise UnlockError(
            f"unlock wrapper received unsupported product {identity.product!r}; refusing Amonet"
        )
    if identity.unlocked:
        return UnlockResult(identity=identity, amonet_invoked=False)

    verify_amonet_unlock_assets(amonet_dir, expected_hashes=expected_hashes)

    if confirmation != CONFIRM_PHRASE:
        raise UnlockError(
            f"locked Crown requires exact confirmation phrase {CONFIRM_PHRASE!r}"
        )

    _invoke_fastbrick(amonet_dir, run=run)
    unlocked = _wait_for_unlocked(identity, identify=identify, sleep=sleep)
    return UnlockResult(identity=unlocked, amonet_invoked=True)
