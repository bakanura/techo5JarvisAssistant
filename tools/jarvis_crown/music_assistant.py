"""Music Assistant left to the metadata a house already has.

Some houses tag their music before Music Assistant ever sees it, and want it to stop looking songs
up on the internet (MusicBrainz, fanart.tv, TheAudioDB, Wikipedia and the rest). That is a switch
on the installer, --music-assistant-local-metadata, and off unless asked for: a house that streams
from a service, or never tagged anything, gets its pictures and biographies from those lookups.

Over Music Assistant's own websocket API (port 8095, a long-lived token made in its settings):
  1. its "online metadata" switch goes off, which stops the lookups for artists, albums and songs
     from every metadata provider, MusicBrainz included, which Music Assistant cannot disable;
  2. the providers that only ever look things up online and that Music Assistant lets be disabled
     are disabled, so none of them is asked for anything else either.
Both are Music Assistant settings, turned back on in its own settings pages.
"""
from __future__ import annotations

import socket
from typing import Any, Callable

from jarvis_crown.home_assistant import ws_open

PORT = 8095

# The online metadata providers Music Assistant (2.10) lets be disabled. musicbrainz and
# playlist_metadata are not among them: it refuses to disable those.
ONLINE_PROVIDERS = ("lrclib", "fanarttv", "theaudiodb", "coverartarchive", "wikipedia", "itunes_artwork",
                    "lastfm_recommendations", "acoustid_lookup")


class MusicAssistantError(RuntimeError):
    """Music Assistant could not be reached, did not take the token, or refused a setting."""


class _Session:
    def __init__(self, frames: Any) -> None:
        self.frames = frames
        self.next_id = 0

    def call(self, command: str, **args: Any) -> Any:
        self.next_id += 1
        message_id = str(self.next_id)
        self.frames.send({"message_id": message_id, "command": command, "args": args})
        while True:
            answer = self.frames.recv()
            if answer.get("message_id") != message_id or answer.get("partial"):
                continue
            if "error_code" in answer:
                raise MusicAssistantError(f"{command}: {answer.get('details') or answer['error_code']}")
            return answer.get("result")


def exchange(sock: Any, host: str, token: str, *, key: str | None = None) -> list[str]:
    """Logs in on a connected socket and switches the online lookups off; returns what changed."""
    frames = ws_open(sock, host, "/ws", key=key, peer="Music Assistant")
    info = frames.recv()
    if "server_version" not in info:
        raise MusicAssistantError("that is not a Music Assistant server")
    session = _Session(frames)
    try:
        session.call("auth", token=token)
    except MusicAssistantError:
        raise MusicAssistantError("Music Assistant did not accept the token") from None
    done = []
    session.call("config/core/save", domain="metadata", values={"enable_online_metadata": False})
    done.append("online metadata lookups off")
    for provider in session.call("config/providers") or []:
        if provider.get("domain") in ONLINE_PROVIDERS and provider.get("enabled"):
            session.call("config/providers/save", provider_domain=provider["domain"],
                         instance_id=provider["instance_id"], values={"enabled": False})
            done.append(f"{provider['domain']} disabled")
    return done


def local_metadata_only(host: str, token: str, *, timeout: float = 20.0,
                        connect: Callable[..., Any] = socket.create_connection) -> list[str]:
    """Music Assistant at host stops looking anything up online; returns what changed."""
    try:
        sock = connect((host, PORT), timeout=timeout)
    except OSError as exc:
        raise MusicAssistantError(f"cannot reach Music Assistant at {host}:{PORT}: {exc}") from None
    try:
        return exchange(sock, f"[{host}]:{PORT}" if ":" in host else f"{host}:{PORT}", token)
    except (OSError, ValueError) as exc:
        raise MusicAssistantError(str(exc)) from None
    finally:
        sock.close()
