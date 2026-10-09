"""Home Assistant's side of a Jarvis Show deployment, over Home Assistant's REST API.

Once the installer is done, a Show still has to be taken in by Home Assistant. By hand that means
five screens per unit: add the ESPHome device with its encryption key, allow it to perform Home
Assistant actions, pick the assistant and the wake word, and give it the DashCast server and its own
Home Assistant access. Here that is one call with an admin token, used for this and kept nowhere.

Every step looks first at what is already there, so running it again on a unit Home Assistant
already has only fills in what is missing. No secret is ever put in an error message.
"""
from __future__ import annotations

from dataclasses import dataclass
import ipaddress
import json
import re
import time
from typing import Any, Callable
import urllib.error
import urllib.parse
import urllib.request

DASHCAST_PORT = 9555
DASHCAST_MIN_KEY = 16
ESPHOME_PORT = 6053
DEFAULT_WAKE_WORD = "Hey Jarvis"
DEFAULT_ASSISTANT = "Jarvis"
_ENTRY_ID_RE = re.compile(r"^[A-Za-z0-9_-]{1,64}$")


class HomeAssistantError(RuntimeError):
    """Home Assistant refused, or never got to, a step of the deployment."""


# ------------------------------------------------------------------------------- addresses and names


def node_slug(name: str) -> str:
    """The ESPHome node name the daemon gives itself from its display name (layout.Slug in echod)."""
    out: list[str] = []
    dash = False
    for ch in name.strip().lower():
        if "a" <= ch <= "z" or "0" <= ch <= "9":
            out.append(ch)
            dash = False
        elif out and not dash:
            out.append("-")
            dash = True
    return "".join(out).rstrip("-")


def service_prefix(name: str) -> str:
    """How Home Assistant names the Show's actions: esphome.<prefix>_<action>."""
    return node_slug(name).replace("-", "_")


def _is_ip(host: str) -> bool:
    try:
        ipaddress.ip_address(host)
    except ValueError:
        return False
    return "%" not in host


def _valid_host(host: str) -> bool:
    if not host:
        return False
    if _is_ip(host):
        return True
    if not re.fullmatch(r"[A-Za-z0-9.-]+", host):
        return False
    return not host.startswith(("-", "."))


def normalize_dashcast(addr: str) -> str:
    """The DashCast server as host:port, however it was typed; the same rules as the daemon's
    NormalizeServer, so what passes here is what the Show keeps. Empty stays empty."""
    s = addr.strip()
    if not s:
        return ""
    if "://" in s:
        s = s.split("://", 1)[1]
    cut = re.search(r"[/?#]", s)
    if cut:
        s = s[: cut.start()]
    if "@" in s:
        s = s[s.rindex("@") + 1:]

    split = None
    if s.startswith("["):
        end = s.find("]")
        if end > 0 and s[end + 1: end + 2] == ":":
            split = (s[1:end], s[end + 2:])
    elif s.count(":") == 1:
        split = tuple(s.split(":"))
    if split is None:
        host, port = s.removeprefix("[").removesuffix("]"), str(DASHCAST_PORT)
        if ":" in host and not _is_ip(host):
            raise ValueError("not an address: write it as host:port, with an IPv6 address in brackets")
    else:
        host, port = split
    if not _valid_host(host):
        raise ValueError("not an address: write it as host:port, like 192.168.1.20:9555")
    if not port.isdigit() or not 1 <= int(port) <= 65535:
        raise ValueError(f"not a port number: {port}")
    return f"[{host}]:{port}" if ":" in host else f"{host}:{port}"


def check_dashcast_key(key: str) -> str:
    key = key.strip()
    if len(key) < DASHCAST_MIN_KEY:
        raise ValueError(f"the DashCast key is shorter than the server accepts ({DASHCAST_MIN_KEY} characters)")
    if any(ord(ch) < 32 for ch in key):
        raise ValueError("the DashCast key has a control character in it")
    return key


def normalize_ha_url(url: str) -> str:
    """Home Assistant's address as http(s)://host[:port], the way the Show and this tool both use it.
    Without a scheme, a domain name means its https site (a reverse proxy, like haos.example.org) and
    an IP address, a .local name or a host:port means Home Assistant itself on http, port 8123."""
    url = url.strip()
    if url and "://" not in url:
        host = url.split("/", 1)[0]
        bare = host.rsplit(":", 1)[0] if host.count(":") == 1 else host
        proxied = ":" not in host and "." in bare and not bare.endswith(".local") \
            and not re.fullmatch(r"[\d.]+", bare)
        url = f"https://{url}" if proxied else f"http://{host if ':' in host else host + ':8123'}{url[len(host):]}"
    u = urllib.parse.urlsplit(url)
    scheme = u.scheme.lower()
    if scheme not in ("http", "https") or not u.hostname:
        raise ValueError("not a Home Assistant address: write it like http://homeassistant.local:8123")
    if u.username or u.password:
        raise ValueError("a Home Assistant address has no user in it; the token is asked for separately")
    if u.path not in ("", "/") or u.query or u.fragment:
        raise ValueError("a Home Assistant address is only the scheme, host and port")
    return f"{scheme}://{u.netloc}"


def normalize_music_assistant(addr: str, resolve: Callable[[str], str] | None = None) -> str:
    """The Music Assistant server as the one IP address the Show pairs its Sendspin player with.
    The Show takes a literal address only, so a name is looked up here, once, when resolve is given."""
    s = addr.strip()
    if not s:
        return ""
    if "://" in s:
        s = s.split("://", 1)[1]
    cut = re.search(r"[/?#]", s)
    if cut:
        s = s[: cut.start()]
    if s.startswith("["):
        s = s[1:s.find("]")] if "]" in s else s
    elif s.count(":") == 1:
        s = s.split(":")[0]  # a port, as a browser shows it; the Show finds the player port itself
    if _is_ip(s):
        return str(ipaddress.ip_address(s))
    if resolve is None or not _valid_host(s):
        raise ValueError("the Music Assistant server has to be an IP address, like 192.168.1.20")
    try:
        ip = resolve(s)
    except OSError as exc:
        raise ValueError(f"cannot look up {s}: {exc}") from None
    if not _is_ip(ip):
        raise ValueError(f"{s} did not resolve to an IP address")
    return str(ipaddress.ip_address(ip))


def check_token(token: str, what: str) -> str:
    token = token.strip()
    if not token or any(ord(ch) < 33 or ord(ch) > 126 for ch in token):
        raise ValueError(f"{what} is empty or has spaces or other characters a token never has")
    return token


# ----------------------------------------------------------------------------------------- the client


class HomeAssistant:
    """A small REST client. The token goes in a header and nowhere else."""

    def __init__(self, url: str, token: str, *, timeout: float = 20.0,
                 opener: Callable[..., Any] | None = None) -> None:
        self.url = normalize_ha_url(url)
        self._token = token
        self.timeout = timeout
        self._open = opener or urllib.request.urlopen

    def request(self, method: str, path: str, body: Any = None) -> Any:
        data = None if body is None else json.dumps(body).encode("utf-8")
        req = urllib.request.Request(self.url + path, data=data, method=method)
        req.add_header("Authorization", "Bearer " + self._token)
        if data is not None:
            req.add_header("Content-Type", "application/json")
        try:
            with self._open(req, timeout=self.timeout) as resp:
                raw = resp.read()
        except urllib.error.HTTPError as exc:
            detail = ""
            try:
                detail = exc.read(300).decode("utf-8", "replace").strip()
            except OSError:
                pass
            if exc.code == 401:
                raise HomeAssistantError(f"{method} {path}: Home Assistant did not accept the token (401)") from None
            raise HomeAssistantError(f"{method} {path}: HTTP {exc.code} {detail}".rstrip()) from None
        except (urllib.error.URLError, OSError) as exc:
            reason = getattr(exc, "reason", exc)
            raise HomeAssistantError(f"cannot reach Home Assistant at {self.url}: {reason}") from None
        if not raw:
            return None
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return raw.decode("utf-8", "replace")


# ----------------------------------------------------------------------------------- the ESPHome entry


def find_entry(ha: HomeAssistant, name: str) -> dict | None:
    """The ESPHome entry for this Show, by the title Home Assistant gave it (its display name)."""
    entries = ha.request("GET", "/api/config/config_entries/entry?domain=esphome") or []
    node = node_slug(name)
    for entry in entries:
        if entry.get("domain") != "esphome":
            continue
        title = str(entry.get("title") or "")
        if title.strip().lower() == name.strip().lower() or node_slug(title) == node:
            return entry
    return None


def discovered_flow(ha: HomeAssistant, name: str) -> dict | None:
    """An ESPHome discovery Home Assistant already has in progress for this Show, if any."""
    node = node_slug(name)
    for flow in ha.request("GET", "/api/config/config_entries/flow") or []:
        if flow.get("handler") != "esphome":
            continue
        context = flow.get("context") or {}
        if context.get("source") not in ("zeroconf", "dhcp", "mqtt"):
            continue
        shown = str((context.get("title_placeholders") or {}).get("name") or "")
        if node_slug(shown) == node:
            return flow
    return None


def _drop_flow(ha: HomeAssistant, flow_id: str) -> None:
    try:
        ha.request("DELETE", f"/api/config/config_entries/flow/{flow_id}")
    except HomeAssistantError:
        pass


class _Unreachable(HomeAssistantError):
    """Home Assistant could not connect to the Show (yet)."""


def _drive_flow(ha: HomeAssistant, result: dict, *, host: str | None, psk: str, name: str) -> str:
    """Answers ESPHome's config flow form by form until it makes the entry; returns its entry_id."""
    seen: set[str] = set()
    for _ in range(8):
        kind = result.get("type")
        flow_id = result.get("flow_id")
        if kind == "create_entry":
            entry = result.get("result") or {}
            entry_id = entry.get("entry_id") if isinstance(entry, dict) else None
            if not entry_id:
                found = find_entry(ha, name)
                entry_id = found and found.get("entry_id")
            if not entry_id:
                raise HomeAssistantError("Home Assistant made the entry but did not say which one")
            return entry_id
        if kind == "abort":
            reason = result.get("reason")
            if reason in ("already_configured", "already_in_progress"):
                found = find_entry(ha, name)
                if found:
                    return found["entry_id"]
                if reason == "already_in_progress":
                    raise _Unreachable("another ESPHome flow for this Show is open in Home Assistant")
            raise HomeAssistantError(f"Home Assistant stopped adding the Show: {reason}")
        if kind != "form" or not flow_id:
            raise HomeAssistantError(f"Home Assistant answered with an unexpected step ({kind})")
        step = str(result.get("step_id"))
        errors = result.get("errors") or {}
        if errors:
            _drop_flow(ha, flow_id)
            base = errors.get("base") if isinstance(errors, dict) else None
            if base in ("connection_error", "cannot_connect", "unknown_error"):
                raise _Unreachable(f"Home Assistant cannot connect to the Show ({base})")
            if base == "invalid_psk":
                raise HomeAssistantError("the Show refused the encryption key: it is not this unit's key")
            raise HomeAssistantError(f"Home Assistant refused step {step}: {errors}")
        if step in seen:
            _drop_flow(ha, flow_id)
            raise HomeAssistantError(f"Home Assistant asked for step {step} twice")
        seen.add(step)
        if step == "user":
            if not host:
                _drop_flow(ha, flow_id)
                raise _Unreachable("no address for the Show yet")
            answer: dict = {"host": host, "port": ESPHOME_PORT}
        elif step == "discovery_confirm":
            answer = {}
        elif step in ("encryption_key", "reauth_confirm", "reauth_encryption_removed_confirm"):
            answer = {"noise_psk": psk}
        else:
            _drop_flow(ha, flow_id)
            raise HomeAssistantError(f"Home Assistant asked something this tool does not answer (step {step}); "
                                     "finish it in Settings -> Devices & services")
        result = ha.request("POST", f"/api/config/config_entries/flow/{flow_id}", answer)
    raise HomeAssistantError("Home Assistant's ESPHome flow did not finish")


def add_device(ha: HomeAssistant, *, name: str, psk: str, host: str | None) -> tuple[str, bool]:
    """The Show's ESPHome entry, made when it is missing. Returns (entry_id, made_now)."""
    found = find_entry(ha, name)
    if found:
        return found["entry_id"], False
    flow = discovered_flow(ha, name)
    if flow is not None:
        result = ha.request("GET", f"/api/config/config_entries/flow/{flow['flow_id']}")
        return _drive_flow(ha, result, host=host, psk=psk, name=name), True
    if not host:
        raise _Unreachable("Home Assistant has not discovered the Show, and its address is not known")
    result = ha.request("POST", "/api/config/config_entries/flow", {"handler": "esphome", "show_advanced_options": False})
    return _drive_flow(ha, result, host=host, psk=psk, name=name), True


def allow_actions(ha: HomeAssistant, entry_id: str) -> bool:
    """Turns on "Allow the device to perform Home Assistant actions". Returns whether it changed."""
    if not _ENTRY_ID_RE.fullmatch(entry_id):
        raise HomeAssistantError("Home Assistant returned an entry id this tool does not trust")
    form = ha.request("POST", "/api/config/config_entries/options/flow", {"handler": entry_id})
    flow_id = form.get("flow_id") if isinstance(form, dict) else None
    if not flow_id or form.get("type") != "form":
        raise HomeAssistantError("the ESPHome entry has no options form")
    answer: dict = {}
    current = None
    for field in form.get("data_schema") or []:
        key = field.get("name")
        if key and "default" in field:
            answer[key] = field["default"]
        if key == "allow_service_calls":
            current = field.get("default")
    if current is None and not any(f.get("name") == "allow_service_calls" for f in form.get("data_schema") or []):
        ha.request("DELETE", f"/api/config/config_entries/options/flow/{flow_id}")
        raise HomeAssistantError("this Home Assistant's ESPHome options have no allow_service_calls")
    if current is True:
        ha.request("DELETE", f"/api/config/config_entries/options/flow/{flow_id}")
        return False
    answer["allow_service_calls"] = True
    done = ha.request("POST", f"/api/config/config_entries/options/flow/{flow_id}", answer)
    if not isinstance(done, dict) or done.get("type") != "create_entry":
        raise HomeAssistantError(f"Home Assistant did not save the ESPHome options: {done.get('errors') if isinstance(done, dict) else done}")
    return True


# ----------------------------------------------------------------------------- entities and actions


def entry_entities(ha: HomeAssistant, entry_id: str) -> list[str]:
    """The entity ids belonging to the ESPHome entry, through the template API (REST has no registry)."""
    if not _ENTRY_ID_RE.fullmatch(entry_id):
        raise HomeAssistantError("Home Assistant returned an entry id this tool does not trust")
    template = ("{% for e in integration_entities('esphome') if config_entry_id(e) == '" + entry_id
                + "' %}{{ e }}\n{% endfor %}")
    text = ha.request("POST", "/api/template", {"template": template})
    return [line.strip() for line in str(text or "").splitlines() if line.strip()]


def _select(ha: HomeAssistant, entities: list[str], suffix: str, option: str) -> str:
    """Sets the select ending in suffix to option. Returns what happened, for the log."""
    matches = [e for e in entities if e.startswith("select.") and e.endswith(suffix)]
    if not matches:
        return f"no {suffix.lstrip('_')} select on this Show"
    entity = min(matches, key=len)
    state = ha.request("GET", f"/api/states/{entity}") or {}
    if state.get("state") == option:
        return f"{entity} is already {option}"
    options = (state.get("attributes") or {}).get("options") or []
    if option not in options:
        return f"{entity} has no option {option!r} (it has {', '.join(map(str, options)) or 'none yet'})"
    ha.request("POST", "/api/services/select/select_option", {"entity_id": entity, "option": option})
    return f"{entity} set to {option}"


def _switch_on(ha: HomeAssistant, entities: list[str], suffix: str) -> str:
    matches = [e for e in entities if e.startswith("switch.") and e.endswith(suffix)]
    if not matches:
        return f"no {suffix.lstrip('_')} switch on this Show"
    entity = min(matches, key=len)
    if (ha.request("GET", f"/api/states/{entity}") or {}).get("state") == "on":
        return f"{entity} is already on"
    ha.request("POST", "/api/services/switch/turn_on", {"entity_id": entity})
    return f"{entity} switched on"


def _services(ha: HomeAssistant) -> set[str]:
    for domain in ha.request("GET", "/api/services") or []:
        if domain.get("domain") == "esphome":
            return set((domain.get("services") or {}).keys())
    return set()


@dataclass(frozen=True)
class DeviceSettings:
    """What the Show itself is given through its actions. Any of them may be missing."""

    dashcast: str | None = None
    dashcast_key: str | None = None
    ha_url: str | None = None
    ha_token: str | None = None
    music_assistant: str | None = None


@dataclass(frozen=True)
class DeployOptions:
    name: str
    psk: str
    host: str | None = None
    wake_word: str | None = DEFAULT_WAKE_WORD
    assistant: str | None = DEFAULT_ASSISTANT
    settings: DeviceSettings = DeviceSettings()
    wait_seconds: float = 300.0


def deploy(ha: HomeAssistant, opts: DeployOptions, *, progress: Callable[[str], None] = print,
           sleep: Callable[[float], None] = time.sleep, clock: Callable[[], float] = time.monotonic,
           find_host: Callable[[], str | None] | None = None) -> str:
    """Takes the Show into Home Assistant and sets it up; returns the ESPHome entry_id."""
    deadline = clock() + opts.wait_seconds
    host = opts.host
    last = ""
    while True:
        try:
            entry_id, made = add_device(ha, name=opts.name, psk=opts.psk, host=host)
            break
        except _Unreachable as exc:
            if clock() >= deadline:
                raise HomeAssistantError(f"{exc}; gave up after {int(opts.wait_seconds)} s "
                                         "(is it on Wi-Fi, and on a network Home Assistant reaches?)") from None
            if str(exc) != last:
                progress(f"waiting: {exc}")
                last = str(exc)
            sleep(10)
            if find_host is not None and not opts.host:
                host = find_host() or host
    progress(f"ESPHome entry {'added' if made else 'already there'}: {opts.name}")
    progress("device actions " + ("allowed" if allow_actions(ha, entry_id) else "were already allowed"))

    prefix = service_prefix(opts.name)
    wanted = []
    s = opts.settings
    if s.dashcast and s.dashcast_key:
        wanted.append((f"{prefix}_dashboard_server", {"address": s.dashcast, "key": s.dashcast_key},
                       f"DashCast server {s.dashcast}"))
    if s.ha_url and s.ha_token:
        wanted.append((f"{prefix}_home_assistant", {"url": s.ha_url, "token": s.ha_token},
                       f"Home Assistant access {s.ha_url}"))
    if s.music_assistant:
        wanted.append((f"{prefix}_sendspin_server", {"ip": s.music_assistant},
                       f"Music Assistant server {s.music_assistant}"))

    # The options change reloads the entry, so the entities and actions come back a moment later.
    entities: list[str] = []
    want_selects = bool(opts.wake_word or opts.assistant)
    while True:
        entities = entry_entities(ha, entry_id) if want_selects or s.music_assistant else []
        services = _services(ha) if wanted else set()
        ready = (not (want_selects or s.music_assistant) or any(e.startswith("select.") for e in entities)) and \
            all(svc in services for svc, _, _ in wanted)
        if ready or clock() >= deadline:
            break
        sleep(5)

    if opts.assistant:
        progress(_select(ha, entities, "_assistant", opts.assistant))
    if opts.wake_word:
        progress(_select(ha, entities, "_wake_word", opts.wake_word))
    services = _services(ha) if wanted else set()
    for svc, data, what in wanted:
        if svc not in services:
            progress(f"WARN: the Show has no esphome.{svc} action in Home Assistant yet; {what} not set")
            continue
        ha.request("POST", f"/api/services/esphome/{svc}", data)
        progress(f"{what} given to the Show")
    if s.music_assistant and f"{prefix}_sendspin_server" in services:
        progress(_switch_on(ha, entities, "_sendspin"))
    return entry_id
