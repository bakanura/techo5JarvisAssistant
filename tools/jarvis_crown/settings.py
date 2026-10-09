"""What a Show is set up with, asked for once at the start of an install.

Wi-Fi, Home Assistant, the DashCast server and the Music Assistant server: each comes from its switch
first, then (secrets only) from the desktop keyring, then from a question at the terminal. The
addresses are remembered in ~/.config/jarvis-show/defaults.json, so the second Show is mostly Enter.
Secrets are never remembered by this tool, never put in an argument list, and reach the installer
only as files readable by the owner, in a folder that is gone when the install ends.

Keyring entries are looked up with secret-tool (or $JARVIS_SHOW_SECRET_TOOL) under
    application jarvis-show secret <ha-token | ha-admin-token | dashcast-key>
    application jarvis-show secret wifi network <ssid>
"""
from __future__ import annotations

import contextlib
from dataclasses import dataclass, replace
import getpass
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
from typing import Callable, Iterator

from jarvis_crown.home_assistant import (check_dashcast_key, check_token, normalize_dashcast,
                                         normalize_ha_url, normalize_music_assistant)

REMEMBERED = ("wifi", "ha_url", "dashcast", "music_assistant")


class SettingsError(ValueError):
    """A setting that was given is not one the Show would accept."""


@dataclass(frozen=True)
class Settings:
    wifi: str | None = None
    wifi_passphrase: str | None = None
    ha_url: str | None = None
    ha_token: str | None = None
    dashcast: str | None = None
    dashcast_key: str | None = None
    music_assistant: str | None = None
    ha_admin_token: str | None = None

    def summary(self) -> list[str]:
        """What will be set, without a single secret in it."""
        out = [f"Wi-Fi: {self.wifi}" if self.wifi else "Wi-Fi: picked on the Show's screen"]
        out.append(f"Home Assistant: {self.ha_url}" if self.ha_url and self.ha_token else "Home Assistant access: not now")
        out.append(f"DashCast: {self.dashcast}" if self.dashcast else "DashCast: not now")
        out.append(f"Music Assistant: {self.music_assistant}" if self.music_assistant else "Music Assistant: not now")
        out.append("added to Home Assistant automatically" if self.ha_admin_token and self.ha_url
                   else "added to Home Assistant: by hand (no admin token)")
        return out


def config_dir() -> Path:
    base = os.environ.get("XDG_CONFIG_HOME") or os.path.join(os.path.expanduser("~"), ".config")
    return Path(base) / "jarvis-show"


def load_defaults(path: Path | None = None) -> dict[str, str]:
    path = path or config_dir() / "defaults.json"
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}
    return {k: v for k, v in data.items() if k in REMEMBERED and isinstance(v, str)} if isinstance(data, dict) else {}


def save_defaults(settings: Settings, path: Path | None = None) -> None:
    """Keeps the addresses (never a secret) as the next run's defaults."""
    path = path or config_dir() / "defaults.json"
    data = {k: getattr(settings, k) for k in REMEMBERED if getattr(settings, k)}
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(data, indent=1) + "\n", encoding="utf-8")
    except OSError:
        pass


# Where secret-tool lives when it is not on PATH (on OpenJade desktops it ships with jplane).
SECRET_TOOL_FALLBACKS = ("~/.local/share/jplane/libsecret/bin/secret-tool",)


def secret_tool() -> str | None:
    found = os.environ.get("JARVIS_SHOW_SECRET_TOOL") or shutil.which("secret-tool")
    if found:
        return found
    for candidate in SECRET_TOOL_FALLBACKS:
        path = os.path.expanduser(candidate)
        if os.access(path, os.X_OK):
            return path
    return None


def keyring_lookup(secret: str, **attrs: str) -> str | None:
    """One secret from the desktop keyring, or None when there is no keyring or no such entry."""
    tool = secret_tool()
    if not tool:
        return None
    argv = [tool, "lookup", "application", "jarvis-show", "secret", secret]
    for key, value in attrs.items():
        argv += [key, value]
    try:
        out = subprocess.run(argv, capture_output=True, text=True, timeout=15, check=False)
    except (OSError, subprocess.TimeoutExpired):
        return None
    value = out.stdout.rstrip("\r\n").strip() if out.returncode == 0 else ""
    return value or None


def read_secret_file(path: Path, what: str) -> str:
    """A secret from a file. Not resolved first: /dev/fd/N from <(...) has to stay what it is."""
    try:
        value = Path(os.path.expanduser(str(path))).read_text(encoding="utf-8").rstrip("\r\n").strip()
    except OSError as exc:
        raise SettingsError(f"cannot read the {what} file: {exc.strerror}") from None
    if not value:
        raise SettingsError(f"the {what} file is empty")
    return value


def _resolve(host: str) -> str:
    return socket.getaddrinfo(host, None, socket.AF_INET, socket.SOCK_STREAM)[0][4][0]


@dataclass
class Asker:
    """The terminal, or nothing when nobody is at it (then every answer has to come from a switch)."""

    interactive: bool
    ask: Callable[[str], str] = input
    ask_secret: Callable[[str], str] = getpass.getpass
    say: Callable[[str], None] = print

    def text(self, label: str, default: str | None, check: Callable[[str], str]) -> str | None:
        """A value, re-asked until the Show would take it. Enter keeps the default; '-' is none."""
        if not self.interactive:
            return None
        hint = f" [{default}]" if default else " (Enter to skip)"
        while True:
            try:
                raw = self.ask(f"   {label}{hint}: ").strip()
            except EOFError:
                return None
            if raw == "-" or (not raw and not default):
                return None
            try:
                return check(raw or default or "")
            except ValueError as exc:
                self.say(f"   {exc}")

    def pick(self, label: str, current: str, options: list[str]) -> str | None:
        """One of options by number or name. Enter keeps current; None when nobody is asked."""
        if not self.interactive or not options:
            return None
        self.say(f"   {label}:")
        for i, option in enumerate(options, 1):
            self.say(f"     {i}) {option}" + ("  (now)" if option == current else ""))
        while True:
            try:
                raw = self.ask(f"   {label} [{current or 'keep'}]: ").strip()
            except EOFError:
                return None
            if not raw:
                return None
            if raw.isdigit() and 1 <= int(raw) <= len(options):
                return options[int(raw) - 1]
            named = [o for o in options if o.casefold() == raw.casefold()]
            if named:
                return named[0]
            self.say(f"   pick 1 to {len(options)}, or Enter to keep it")

    def secret(self, label: str, check: Callable[[str], str]) -> str | None:
        if not self.interactive:
            return None
        while True:
            try:
                raw = self.ask_secret(f"   {label} (hidden; Enter to skip): ").strip()
            except EOFError:
                return None
            if not raw:
                return None
            try:
                return check(raw)
            except ValueError as exc:
                self.say(f"   {exc}")


def _checked(value: str | None, check: Callable[[str], str]) -> str | None:
    if value is None:
        return None
    try:
        return check(value)
    except ValueError as exc:
        raise SettingsError(str(exc)) from None


def gather(args, asker: Asker, *, defaults: dict[str, str] | None = None,
           lookup: Callable[..., str | None] = keyring_lookup,
           resolve: Callable[[str], str] = _resolve, want_wifi: bool = True) -> Settings:
    """Everything the Show is set up with: switches first, then the keyring, then the terminal."""
    d = defaults if defaults is not None else load_defaults()
    say = asker.say if asker.interactive else (lambda _text: None)

    def ma(value: str) -> str:
        return normalize_music_assistant(value, resolve)

    def ha_token(value: str) -> str:
        return check_token(value, "the token")

    def from_file(attr: str, what: str, check: Callable[[str], str]) -> str | None:
        path = getattr(args, attr, None)
        return _checked(read_secret_file(path, what), check) if path else None

    s = Settings()
    if want_wifi:
        say("Wi-Fi (the Show joins it on first boot):")
        wifi = getattr(args, "wifi", None) or asker.text("network", d.get("wifi"), lambda v: v)
        passphrase = None
        if wifi and not getattr(args, "wifi_passphrase_file", None):
            passphrase = lookup("wifi", network=wifi) or asker.secret(f"passphrase for {wifi!r}", lambda v: v)
            if passphrase is None and asker.interactive:
                say("   no passphrase: the Show will ask for the network on its screen instead")
                wifi = None
        s = replace(s, wifi=wifi, wifi_passphrase=passphrase)

    say("Home Assistant:")
    ha_url = _checked(getattr(args, "ha_url", None), normalize_ha_url) or \
        asker.text("address, as the Show reaches it", d.get("ha_url"), normalize_ha_url)
    token = admin = None
    if ha_url:
        token = from_file("ha_token_file", "Home Assistant token", ha_token) or \
            _checked(lookup("ha-token"), ha_token) or \
            asker.secret("the Show's own long-lived token (photos, weather, cameras)", ha_token)
        admin = from_file("ha_admin_token_file", "Home Assistant admin token", ha_token) or \
            _checked(lookup("ha-admin-token"), ha_token) or \
            asker.secret("an admin token, used once to add the Show to Home Assistant", ha_token)
    s = replace(s, ha_url=ha_url, ha_token=token, ha_admin_token=admin)

    say("DashCast (the streamed dashboard):")
    dashcast = _checked(getattr(args, "dashcast", None), normalize_dashcast) or \
        asker.text("server, host[:port]", d.get("dashcast"), normalize_dashcast)
    key = None
    if dashcast:
        key = from_file("dashcast_key_file", "DashCast key", check_dashcast_key) or \
            _checked(lookup("dashcast-key"), check_dashcast_key) or \
            asker.secret("the DashCast key", check_dashcast_key)
        if key is None:
            say("   no key: DashCast left for later")
            dashcast = None
    s = replace(s, dashcast=dashcast, dashcast_key=key)

    say("Music Assistant (the Sendspin player):")
    music = _checked(getattr(args, "music_assistant", None), ma) or \
        asker.text("server IP or name", d.get("music_assistant"), ma)
    s = replace(s, music_assistant=music)
    return s


@contextlib.contextmanager
def secret_files(settings: Settings) -> Iterator[dict[str, Path]]:
    """The secrets as owner-only files in a private folder that is removed afterwards."""
    with tempfile.TemporaryDirectory(prefix="jarvis-show-") as tmp:
        out: dict[str, Path] = {}
        for name in ("wifi_passphrase", "ha_token", "dashcast_key"):
            value = getattr(settings, name)
            if not value:
                continue
            path = Path(tmp) / name
            fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                f.write(value + "\n")
            out[name] = path
        yield out
