#!/usr/bin/env python3
"""Jarvis Show v1 installer front-end for Crown and Checkers."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
from jarvis_crown.amonet_upgrade import AdbUpgradeClient, UpgradeError, upgrade_amonet  # noqa: E402
from jarvis_crown.assets import amonet_bundle_dir, asset_path, assets_for, default_cache_dir  # noqa: E402
from jarvis_crown.boot_logo import BootLogoError, SshClient, put_logo  # noqa: E402
from jarvis_crown.boards import profile_for_board, profile_for_product  # noqa: E402
from jarvis_crown.device_gate import DeviceGateError, identify_show  # noqa: E402
from jarvis_crown.flow import FlowError, InstallInputs, run_install_flow  # noqa: E402
from jarvis_crown import home_assistant as ha_api  # noqa: E402
from jarvis_crown.shows import known_shows, show_named  # noqa: E402
from jarvis_crown.settings import Asker, Settings, SettingsError, gather, save_defaults, secret_files  # noqa: E402
from jarvis_crown.preflight import preflight_ok, print_checks, run_preflight  # noqa: E402
from jarvis_crown.recovery import RecoveryError, adb_recovery_serials, identify_recovery_show  # noqa: E402
from jarvis_crown.unlock import UnlockError, unlock_show  # noqa: E402

SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(prog="jarvis-show")
    parser.add_argument("command", help="what to do (default preflight); docs/jarvis-crown-installer.md, \"Commands and switches\"", choices=["preflight", "identify", "unlock", "install", "amonet-upgrade", "wifi", "home-assistant", "boot-logo"], nargs="?", default="preflight")
    parser.add_argument("--board", choices=("crown", "checkers"), help="optional first-gen board cross-check; install/identify auto-detect by default")
    parser.add_argument("--amonet-dir", type=Path, help="the unpacked Amonet 2.x package; default ../third_party/<board> or the asset cache")
    parser.add_argument("--amonet-zip", type=Path, help="amonet-upgrade: the pinned Amonet 2.x zip; default from the asset cache")
    parser.add_argument("--amonet-hashes", type=Path, help="trusted JSON SHA-256 map for a board whose Amonet bytes are not built-in")
    parser.add_argument("--twrp-sha256", help="trusted board-specific TWRP SHA-256 when not built-in")
    parser.add_argument("--lineage-zip", type=Path, help="install, amonet-upgrade: the LineageOS 18.1 zip for this board (fetch-show-assets.py prints it)")
    parser.add_argument("--work-dir", type=Path, help="scratch space for the install; default ../work next to the repo")
    parser.add_argument("--backup-dir", type=Path, help="where each unit's backups, key and name are kept; default ../backups")
    parser.add_argument("--name", help="the Show's name, as Home Assistant shows it; install, home-assistant and boot-logo find it by this")
    parser.add_argument("--boot-image", type=Path, help="install: the board's boot image (fetch-show-assets.py prints it)")
    parser.add_argument("--boot-sha256", help="trusted board-specific boot image SHA-256 when not built-in")
    parser.add_argument("--rootfs", type=Path, help="install: the Jarvis root filesystem from a release")
    parser.add_argument("--rootfs-sha256", help="install: its SHA-256, from the release notes")
    parser.add_argument("--wifi", help="install, wifi: the network (SSID) it joins; default: ask, or pick it on the Show's screen")
    parser.add_argument("--wifi-passphrase-file", type=Path, help="a file holding its passphrase; default the keyring, then ask")
    parser.add_argument("--serial", help="the unit's serial: wifi talks to that Show on USB (found when only one is attached); home-assistant takes its key from backups/<serial>")
    parser.add_argument("--ssh-key", type=Path, help="install: an SSH public key root accepts from the first boot (SSH is switched on)")
    setup = parser.add_argument_group("what the Show is ready with (asked for when left out)")
    setup.add_argument("--ha-url", help="Home Assistant's address as the Show reaches it")
    setup.add_argument("--ha-token-file", type=Path, help="a file holding the Show's own long-lived Home Assistant token")
    setup.add_argument("--ha-admin-token-file", type=Path, help="a file holding an admin token, used once to add the Show to Home Assistant")
    setup.add_argument("--dashcast", help="the DashCast server, host[:port]")
    setup.add_argument("--dashcast-key-file", type=Path, help="a file holding the DashCast key; default the keyring, then ask")
    setup.add_argument("--music-assistant", help="the Music Assistant server's IP address or name")
    setup.add_argument("--wake-word", help="the wake word, one the Show offers (default: ask; '' leaves it alone)")
    setup.add_argument("--assistant", help="the Assist pipeline it talks to, by name (default: ask; '' leaves it alone)")
    setup.add_argument("--room", help="the Home Assistant area it stands in, by name (default: ask; '' leaves it alone)")
    setup.add_argument("--no-questions", action="store_true", help="ask nothing; take switches and the keyring only")
    hass = parser.add_argument_group("home-assistant: add an installed Show to Home Assistant")
    hass.add_argument("--host", help="the Show's address, when Home Assistant has not discovered it")
    hass.add_argument("--key-file", type=Path, help="its encryption key (default backups/<serial>/home-assistant.key)")
    logo = parser.add_argument_group("boot-logo: the OpenJade logo at boot on an installed Show (--name, --host)")
    logo.add_argument("--amazon-logo", action="store_true", help="put Amazon's logo back instead")
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


def _print_identity(identity, profile, source: str) -> None:
    print(f"PASS: detected {profile.model}")
    print(f"PASS: product={identity.product}")
    print(f"PASS: {source} serial={identity.serial}")
    print(f"PASS: unlock_status={'true' if identity.unlocked else 'false'}")


def _detect_profile(board: str | None, *, allow_recovery: bool = False):
    try:
        identity = identify_show(expected_board=board)
        source = "fastboot"
    except DeviceGateError as fastboot_error:
        if not allow_recovery or board is None:
            raise
        try:
            identity = identify_recovery_show(board)
        except RecoveryError as recovery_error:
            raise DeviceGateError(
                f"fastboot identity unavailable ({fastboot_error}); "
                f"recovery identity unavailable ({recovery_error})"
            ) from recovery_error
        source = "recovery"
    profile = profile_for_product(identity.product)
    _print_identity(identity, profile, source)
    return identity, profile, source


def _cached(board: str, kind: str) -> Path:
    asset = next(a for a in assets_for(board) if a.kind == kind)
    return asset_path(default_cache_dir(), board, asset)


def _asker(args) -> Asker:
    return Asker(interactive=not args.no_questions and not os.environ.get("TECHO5_NO_PROMPT")
                 and sys.stdin.isatty() and sys.stdout.isatty())


def _find_host(name: str):
    """The Show's address over mDNS, for when Home Assistant has not discovered it on its own."""
    tool = shutil.which("avahi-resolve-host-name")

    def find():
        if not tool:
            return None
        try:
            out = subprocess.run([tool, "-4", ha_api.node_slug(name) + ".local"], capture_output=True,
                                 text=True, timeout=8, check=False).stdout.split()
        except (OSError, subprocess.TimeoutExpired):
            return None
        return out[1] if len(out) == 2 else None
    return find


# How the assistant, wake word and room questions read, by the label deploy() asks them with.
QUESTIONS = {
    "assistant": ("Which assistant should the Show talk to?",
                  {"preferred": "the one Home Assistant has as preferred"}, ("jarvis",)),
    "wake word": ("Which wake word should it listen for?",
                  {"no_wake_word": "none, no wake word"}, ("hey jarvis", "jarvis")),
    "room": ("Which room is it in? (it plays and shows that room's music)", {}, ()),
}


def _chooser(args):
    """Asks for the assistant, wake word and room the switches left open; '' on a switch means do not touch it.
    Enter takes Jarvis when this Home Assistant and this Show have it."""
    asker = _asker(args)
    if not asker.interactive:
        return None
    given = {"assistant": args.assistant, "wake word": args.wake_word, "room": args.room}

    def choose(label: str, current: str, options: list[str], shown: dict[str, str] | None = None) -> str | None:
        if given.get(label) is not None:
            return None
        question, names, liked = QUESTIONS.get(label, (label, {}, ()))
        names = {**names, **(shown or {})}
        default = next((o for want in liked for o in options if o.casefold() == want), None) or \
            next((o for o in options if any(want in o.casefold() for want in liked)), None)
        return asker.pick(question, current, options, default=default, names=names)
    return choose


def _key_file(args, backups: Path) -> Path | None:
    """The Show's encryption key: from the switches, or from the record the installer left for this name."""
    if args.key_file:
        return args.key_file
    if args.serial:
        return backups / args.serial / "home-assistant.key"
    show = show_named(backups, args.name)
    if show is None:
        print(f"FAIL: no Show named {args.name!r} was installed from here", file=sys.stderr)
        for known in known_shows(backups):
            print(f"INFO:   installed: {known.name!r} ({known.board}, serial ending {known.serial_tail})",
                  file=sys.stderr)
        print("INFO:   or say which with --serial or --key-file", file=sys.stderr)
        return None
    print(f"PASS: {show.name} is the {show.board} Show with serial ending {show.serial_tail}")
    return show.key_file


def deploy_to_home_assistant(settings: Settings, args, *, name: str, key_file: Path, host: str | None,
                             wait_seconds: float) -> int:
    """Adds the Show to Home Assistant and hands it its settings through its own actions."""
    if not (settings.ha_url and settings.ha_admin_token):
        print("INFO: no Home Assistant admin token, so add the Show by hand:")
        print(f"INFO:   Settings -> Devices & services -> ESPHome, with the key in {key_file}")
        print(f"INFO:   or later: python3 tools/jarvis-show.py home-assistant --name {name!r}")
        return 0
    try:
        psk = key_file.read_text(encoding="utf-8").strip()
    except OSError as exc:
        print(f"FAIL: cannot read the Show's encryption key: {exc.strerror}", file=sys.stderr)
        return 1
    if not host:
        saved = key_file.parent / "address"
        if saved.is_file():
            host = saved.read_text(encoding="utf-8").strip() or None
    opts = ha_api.DeployOptions(
        name=name, psk=psk, host=host, wake_word=args.wake_word or None, assistant=args.assistant or None,
        room=args.room or None,
        settings=ha_api.DeviceSettings(dashcast=settings.dashcast, dashcast_key=settings.dashcast_key,
                                       ha_url=settings.ha_url, ha_token=settings.ha_token,
                                       music_assistant=settings.music_assistant),
        wait_seconds=wait_seconds)
    print(f"INFO: adding {name!r} to Home Assistant at {settings.ha_url}")
    try:
        ha_api.deploy(ha_api.HomeAssistant(settings.ha_url, settings.ha_admin_token), opts,
                      progress=lambda text: print(f"INFO: {text}"), find_host=_find_host(name),
                      choose=_chooser(args))
    except ha_api.HomeAssistantError as exc:
        print(f"FAIL: Home Assistant: {exc}", file=sys.stderr)
        return 5
    print(f"PASS: {name!r} is in Home Assistant and set up")
    return 0


def home_assistant_command(args, backups: Path) -> int:
    if not args.name:
        print("FAIL: home-assistant requires --name, the Show's name as installed", file=sys.stderr)
        return 1
    key_file = _key_file(args, backups)
    if key_file is None:
        return 1
    try:
        settings = gather(args, _asker(args), want_wifi=False)
    except SettingsError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
    save_defaults(settings)
    if not settings.ha_admin_token:
        print("FAIL: adding the Show needs a Home Assistant admin token "
              "(--ha-admin-token-file, the keyring, or the question)", file=sys.stderr)
        return 1
    return deploy_to_home_assistant(settings, args, name=args.name, key_file=key_file, host=args.host,
                                    wait_seconds=120)


def amonet_upgrade(args, backups: Path) -> int:
    """Amonet 1.x -> 2.x for a unit already in TWRP; see jarvis_crown/amonet_upgrade.py."""
    if not args.board:
        print("FAIL: amonet-upgrade requires --board", file=sys.stderr)
        return 1
    try:
        serials = adb_recovery_serials()
    except RecoveryError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 2
    if len(serials) != 1:
        print(f"FAIL: expected exactly one Show in TWRP, found {len(serials)} (adb reboot recovery first)", file=sys.stderr)
        return 2

    def confirm(phrase):
        print("WARN: this rewrites the bootloader chain: preloader, lk, tee1, tee2, expdb, recovery and swdl.")
        print("WARN: keep the Show on mains power and the USB cable in until it says PASS.")
        try:
            return input(f"Type {phrase} to continue: ").strip()
        except EOFError:  # no terminal (piped or closed stdin) is a no, not a crash
            print()
            return ""

    try:
        result = upgrade_amonet(
            AdbUpgradeClient(serials[0]),
            args.board,
            (args.amonet_zip or _cached(args.board, "amonet")).resolve(),
            (args.lineage_zip or _cached(args.board, "lineage")).resolve(),
            backups / serials[0],
            confirm=confirm,
            progress=lambda text: print(f"INFO: {text}"),
        )
    except (UpgradeError, RecoveryError, ValueError) as exc:
        print(f"FAIL: amonet-upgrade stopped: {exc}", file=sys.stderr)
        return 4
    if result.upgraded:
        print(f"PASS: Amonet 2.x installed and boot is plain; backup of the old state in {result.backup}")
    else:
        print("PASS: no Amonet 1.x microloader on boot; nothing to do")
    return 0


def boot_logo_command(args, backups: Path) -> int:
    """OpenJade's logo in kaeru's wordmark slot, over the Show's SSH; see jarvis_crown/boot_logo.py."""
    if not args.name:
        print("FAIL: boot-logo requires --name, the Show's name as installed", file=sys.stderr)
        return 1
    show = show_named(backups, args.name)
    if show is None:
        print(f"FAIL: no Show named {args.name!r} was installed from here", file=sys.stderr)
        for known in known_shows(backups):
            print(f"INFO:   installed: {known.name!r} ({known.board}, serial ending {known.serial_tail})", file=sys.stderr)
        return 1
    host = args.host
    if not host:
        try:
            host = (show.folder / "address").read_text(encoding="utf-8").strip()
        except OSError:
            host = ""
    if not host:
        print("FAIL: the Show's address is not known; give it with --host", file=sys.stderr)
        return 1
    saved = show.folder / "partitions" / "p7-expdb.img"
    print(f"PASS: {show.name} is the {show.board} Show with serial ending {show.serial_tail}, at {host}")

    def confirm(phrase):
        print("WARN: this rewrites the logo inside the bootloader (kaeru, in expdb); nothing else is written.")
        print("WARN: keep the Show on power until it says PASS. A broken kaeru does not start at all.")
        try:
            return input(f"Type {phrase} to continue: ").strip()
        except EOFError:  # no terminal (piped or closed stdin) is a no, not a crash
            print()
            return ""

    try:
        done = put_logo(SshClient(host), show.board, saved_expdb=saved, amazon=args.amazon_logo,
                        confirm=confirm, progress=lambda text: print(f"INFO: {text}"))
    except BootLogoError as exc:
        print(f"FAIL: boot-logo stopped: {exc}", file=sys.stderr)
        return 4
    print(f"PASS: {done}; it shows on the next cold start (unplug, plug back in)")
    return 0


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

    if args.command == "amonet-upgrade":
        return amonet_upgrade(args, backups)

    if args.command == "boot-logo":
        return boot_logo_command(args, backups)
    if args.command == "home-assistant":
        return home_assistant_command(args, backups)

    # identify/unlock require fastboot. install can resume from a verified TWRP
    # session only when an explicit board cross-check is supplied.
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
            identity, profile, identity_source = _detect_profile(
                args.board, allow_recovery=args.command == "install"
            )
        except DeviceGateError as exc:
            print(f"FAIL: device identity gate: {exc}", file=sys.stderr)
            return 2

    # A bundle checked out next to the repo wins; otherwise the one fetch-show-assets unpacked from the pinned zip.
    bundled = project / "third_party" / profile.amonet_dir_name
    amonet = (args.amonet_dir or (bundled if bundled.is_dir()
                                  else amonet_bundle_dir(default_cache_dir(), profile.board))).resolve()

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

        # Everything the Show starts with, asked before anything on it is touched.
        try:
            settings = gather(args, _asker(args))
        except SettingsError as exc:
            print(f"FAIL: {exc}", file=sys.stderr)
            return 1
        save_defaults(settings)
        for line in settings.summary():
            print(f"INFO: {line}")

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

        with secret_files(settings) as files:
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
                wifi=settings.wifi,
                wifi_passphrase_file=files.get("wifi_passphrase") or (
                    args.wifi_passphrase_file.resolve() if args.wifi_passphrase_file and settings.wifi else None),
                ha_url=settings.ha_url if settings.ha_token else None,
                ha_token_file=files.get("ha_token") if settings.ha_url else None,
                dashcast=settings.dashcast,
                dashcast_key_file=files.get("dashcast_key") if settings.dashcast else None,
                music_assistant=settings.music_assistant,
                ssh_key=args.ssh_key.resolve() if args.ssh_key else None,
                expected_fastboot_serial=identity.serial if identity is not None else None,
            )
            try:
                result = run_install_flow(
                    inputs,
                    confirm_unlock=confirm_unlock,
                    confirm_install=confirm_install,
                    progress=lambda stage: print(f"INFO: stage={stage}"),
                    initial_identity=identity if identity_source == "recovery" else None,
                )
            except (FlowError, DeviceGateError, UnlockError, RuntimeError) as exc:
                print(f"FAIL: install stopped: {exc}", file=sys.stderr)
                return 4
        print(f"PASS: {result.profile.product_id} installation flow completed")
        if not settings.wifi:
            print("INFO: join Wi-Fi on the Show's screen, then add it to Home Assistant with:")
            print(f"INFO:   python3 tools/jarvis-show.py home-assistant --name {args.name!r}")
            return 0
        key_file = backups / result.recovery.adb_serial / "home-assistant.key"
        return deploy_to_home_assistant(settings, args, name=args.name, key_file=key_file, host=None,
                                        wait_seconds=300)

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
