import argparse
import contextlib
import io
import importlib.util
import json
import os
import pathlib
import stat
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools"
sys.path.insert(0, str(TOOLS))

from jarvis_crown import home_assistant as ha  # noqa: E402
from jarvis_crown.settings import Asker, Settings, SettingsError, gather, load_defaults, save_defaults, secret_files  # noqa: E402
from jarvis_crown.shows import record_show, show_named  # noqa: E402
from techo5lib import Fail  # noqa: E402

SPEC = importlib.util.spec_from_file_location("jarvis_install_show_settings", TOOLS / "install-show.py")
install = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(install)

PSK = "c2VjcmV0LWtleS1mb3ItdGVzdHMtb25seS0zMmJ5dGVzIQ=="
TOKEN = "a.b.c-token"
DASH_KEY = "0123456789abcdef0123"


class AddressTests(unittest.TestCase):
    def test_dashcast_matches_the_daemon(self):
        cases = {
            "192.168.8.250": "192.168.8.250:9555",
            "http://192.168.8.250:9555/": "192.168.8.250:9555",
            "dash.example:80": "dash.example:80",
            "user@dash.example": "dash.example:9555",
            "[fd00::1]:9000": "[fd00::1]:9000",
            "fd00::1": "[fd00::1]:9555",
            "": "",
        }
        for given, want in cases.items():
            self.assertEqual(ha.normalize_dashcast(given), want, given)
        for bad in ("dash example", "-x:1", "host:0", "host:99999", "host:abc"):
            with self.assertRaises(ValueError, msg=bad):
                ha.normalize_dashcast(bad)

    def test_music_assistant_is_one_ip(self):
        self.assertEqual(ha.normalize_music_assistant("192.168.8.125"), "192.168.8.125")
        self.assertEqual(ha.normalize_music_assistant("http://192.168.8.125:8095/"), "192.168.8.125")
        self.assertEqual(ha.normalize_music_assistant("[fd00::2]:8095"), "fd00::2")
        with self.assertRaises(ValueError):
            ha.normalize_music_assistant("ma.local")
        self.assertEqual(ha.normalize_music_assistant("ma.local", lambda h: "10.1.2.3"), "10.1.2.3")
        with self.assertRaises(ValueError):
            ha.normalize_music_assistant("ma.local", lambda h: "not-an-ip")

    def test_ha_url_is_scheme_host_port(self):
        self.assertEqual(ha.normalize_ha_url("https://HA.example/"), "https://HA.example")
        self.assertEqual(ha.normalize_ha_url("http://10.0.0.2:8123"), "http://10.0.0.2:8123")
        self.assertEqual(ha.normalize_ha_url("haos.example.org"), "https://haos.example.org")
        self.assertEqual(ha.normalize_ha_url("10.0.0.2"), "http://10.0.0.2:8123")
        self.assertEqual(ha.normalize_ha_url("10.0.0.2:8124"), "http://10.0.0.2:8124")
        self.assertEqual(ha.normalize_ha_url("homeassistant.local"), "http://homeassistant.local:8123")
        for bad in ("ftp://ha", "http://u:p@ha", "http://ha/lovelace", "haos.example.org/lovelace"):
            with self.assertRaises(ValueError, msg=bad):
                ha.normalize_ha_url(bad)

    def test_names(self):
        self.assertEqual(ha.node_slug("Jarvis Show 5"), "jarvis-show-5")
        self.assertEqual(ha.node_slug("  Küche / Echo!! "), "k-che-echo")
        self.assertEqual(ha.service_prefix("Jarvis Show 5"), "jarvis_show_5")


class FakeHA:
    """Home Assistant's REST answers for one ESPHome device, from nothing to fully set up."""

    def __init__(self, *, flow_errors=(), discovered=False, allowed=False, entry=False):
        self.calls = []
        self.flow_errors = list(flow_errors)
        self.discovered = discovered
        self.allowed = allowed
        self.entries = [{"entry_id": "E1", "domain": "esphome", "title": "Jarvis Show 5"}] if entry else []
        self.states = {
            "select.jarvis_show_5_assistant": {"state": "preferred", "attributes": {"options": ["preferred", "Jarvis"]}},
            "select.jarvis_show_5_wake_word": {"state": "Okay Nabu", "attributes": {"options": ["Okay Nabu", "Hey Jarvis"]}},
            "switch.jarvis_show_5_playback_sendspin": {"state": "off"},
        }

    def request(self, method, path, body=None):
        self.calls.append((method, path, body))
        if path.startswith("/api/config/config_entries/entry"):
            return list(self.entries)
        if method == "GET" and path == "/api/config/config_entries/flow":
            if not self.discovered:
                return []
            return [{"flow_id": "D1", "handler": "esphome",
                     "context": {"source": "zeroconf", "title_placeholders": {"name": "jarvis-show-5"}}}]
        if method == "GET" and path == "/api/config/config_entries/flow/D1":
            return {"type": "form", "flow_id": "D1", "step_id": "discovery_confirm"}
        if method == "POST" and path == "/api/config/config_entries/flow":
            return {"type": "form", "flow_id": "F1", "step_id": "user"}
        if method == "POST" and path.startswith("/api/config/config_entries/flow/"):
            flow = path.rsplit("/", 1)[1]
            if self.flow_errors:
                return {"type": "form", "flow_id": flow, "step_id": "user", "errors": {"base": self.flow_errors.pop(0)}}
            if "noise_psk" in body:
                self.entries = [{"entry_id": "E1", "domain": "esphome", "title": "Jarvis Show 5"}]
                return {"type": "create_entry", "result": {"entry_id": "E1"}}
            return {"type": "form", "flow_id": flow, "step_id": "encryption_key"}
        if method == "DELETE":
            return None
        if path == "/api/config/config_entries/options/flow":
            return {"type": "form", "flow_id": "O1", "data_schema": [
                {"name": "allow_service_calls", "default": self.allowed},
                {"name": "subscribe_logs", "default": False}]}
        if path == "/api/config/config_entries/options/flow/O1":
            self.allowed = body["allow_service_calls"]
            return {"type": "create_entry"}
        if path == "/api/template":
            return "\n".join(list(self.states) + ["select.jarvis_show_5_assistant_2"])
        if path == "/api/services" and method == "GET":
            names = ["jarvis_show_5_dashboard_server", "jarvis_show_5_home_assistant", "jarvis_show_5_sendspin_server"]
            return [{"domain": "light", "services": {}}, {"domain": "esphome", "services": {n: {} for n in names}}]
        if path.startswith("/api/states/"):
            return self.states.get(path[len("/api/states/"):])
        if path == "/api/services/select/select_option":
            self.states[body["entity_id"]]["state"] = body["option"]
            return []
        if path == "/api/services/switch/turn_on":
            self.states[body["entity_id"]]["state"] = "on"
            return []
        if path.startswith("/api/services/esphome/"):
            return []
        raise AssertionError(f"unexpected {method} {path}")

    def posted(self, prefix):
        return [(p, b) for m, p, b in self.calls if m == "POST" and p.startswith(prefix)]


def run_deploy(fake, **kw):
    settings = ha.DeviceSettings(dashcast="10.0.0.5:9555", dashcast_key=DASH_KEY, ha_url="http://ha:8123",
                                 ha_token=TOKEN, music_assistant="10.0.0.6")
    opts = ha.DeployOptions(name="Jarvis Show 5", psk=PSK, settings=settings, **kw)
    log = []
    now = [0.0]

    def sleep(seconds):
        now[0] += seconds
    entry = ha.deploy(fake, opts, progress=log.append, sleep=sleep, clock=lambda: now[0])
    return entry, log


class DeployTests(unittest.TestCase):
    def test_new_show_by_address(self):
        fake = FakeHA()
        entry, log = run_deploy(fake, host="10.0.0.9")
        self.assertEqual(entry, "E1")
        self.assertIn(("/api/config/config_entries/flow/F1", {"host": "10.0.0.9", "port": 6053}),
                      fake.posted("/api/config/config_entries/flow/F1"))
        self.assertIn(("/api/config/config_entries/flow/F1", {"noise_psk": PSK}),
                      fake.posted("/api/config/config_entries/flow/F1"))
        self.assertTrue(fake.allowed)
        self.assertEqual(fake.posted("/api/config/config_entries/options/flow/O1")[0][1],
                         {"allow_service_calls": True, "subscribe_logs": False})
        self.assertEqual(fake.states["select.jarvis_show_5_assistant"]["state"], "Jarvis")
        self.assertEqual(fake.states["select.jarvis_show_5_wake_word"]["state"], "Hey Jarvis")
        self.assertEqual(fake.states["switch.jarvis_show_5_playback_sendspin"]["state"], "on")
        actions = dict(fake.posted("/api/services/esphome/"))
        self.assertEqual(actions["/api/services/esphome/jarvis_show_5_dashboard_server"],
                         {"address": "10.0.0.5:9555", "key": DASH_KEY})
        self.assertEqual(actions["/api/services/esphome/jarvis_show_5_home_assistant"],
                         {"url": "http://ha:8123", "token": TOKEN})
        self.assertEqual(actions["/api/services/esphome/jarvis_show_5_sendspin_server"], {"ip": "10.0.0.6"})
        for line in log:
            self.assertNotIn(PSK, line)
            self.assertNotIn(TOKEN, line)
            self.assertNotIn(DASH_KEY, line)

    def test_discovered_show_needs_no_address(self):
        fake = FakeHA(discovered=True)
        entry, _ = run_deploy(fake)
        self.assertEqual(entry, "E1")
        self.assertIn(("/api/config/config_entries/flow/D1", {}), fake.posted("/api/config/config_entries/flow/D1"))
        self.assertFalse(fake.posted("/api/config/config_entries/flow/F1"))

    def test_known_show_is_only_brought_up_to_date(self):
        fake = FakeHA(entry=True, allowed=True)
        fake.states["select.jarvis_show_5_wake_word"]["state"] = "Hey Jarvis"
        _, log = run_deploy(fake)
        self.assertFalse(fake.posted("/api/config/config_entries/flow"))
        self.assertFalse(fake.posted("/api/config/config_entries/options/flow/O1"))
        self.assertIn(("DELETE", "/api/config/config_entries/options/flow/O1", None), fake.calls)
        self.assertFalse([p for p, b in fake.posted("/api/services/select/") if b["entity_id"].endswith("_wake_word")])
        self.assertTrue(any("already" in line for line in log))

    def test_waits_while_the_show_is_not_reachable_yet(self):
        fake = FakeHA(flow_errors=["cannot_connect", "cannot_connect"])
        entry, log = run_deploy(fake, host="10.0.0.9")
        self.assertEqual(entry, "E1")
        self.assertEqual(sum(1 for line in log if line.startswith("waiting")), 1)

    def test_gives_up_at_the_deadline(self):
        fake = FakeHA(flow_errors=["cannot_connect"] * 100)
        with self.assertRaisesRegex(ha.HomeAssistantError, "gave up"):
            run_deploy(fake, host="10.0.0.9", wait_seconds=30)

    def test_wrong_key_stops_at_once(self):
        fake = FakeHA(flow_errors=["invalid_psk"])
        with self.assertRaisesRegex(ha.HomeAssistantError, "not this unit's key"):
            run_deploy(fake, host="10.0.0.9")

    def test_unknown_step_is_left_to_a_person(self):
        fake = FakeHA()
        real = fake.request

        def request(method, path, body=None):
            if method == "POST" and path == "/api/config/config_entries/flow":
                fake.calls.append((method, path, body))
                return {"type": "form", "flow_id": "F1", "step_id": "name_conflict"}
            return real(method, path, body)
        fake.request = request
        with self.assertRaisesRegex(ha.HomeAssistantError, "name_conflict"):
            run_deploy(fake, host="10.0.0.9")
        self.assertIn(("DELETE", "/api/config/config_entries/flow/F1", None), fake.calls)

    def test_client_never_shows_the_token(self):
        import urllib.error

        def opener(req, timeout):
            raise urllib.error.HTTPError(req.full_url, 401, "Unauthorized", {}, None)
        client = ha.HomeAssistant("http://ha:8123", TOKEN, opener=opener)
        with self.assertRaises(ha.HomeAssistantError) as cm:
            client.request("GET", "/api/")
        self.assertNotIn(TOKEN, str(cm.exception))
        self.assertIn("401", str(cm.exception))


def args(**kw):
    base = dict(wifi=None, wifi_passphrase_file=None, ha_url=None, ha_token_file=None, ha_admin_token_file=None,
                dashcast=None, dashcast_key_file=None, music_assistant=None)
    base.update(kw)
    return argparse.Namespace(**base)


class GatherTests(unittest.TestCase):
    def asker(self, answers, secrets):
        answers, secrets, said = list(answers), list(secrets), []
        return Asker(True, ask=lambda _: answers.pop(0), ask_secret=lambda _: secrets.pop(0), say=said.append), said

    def test_everything_asked(self):
        asker, _ = self.asker(["Home", "http://ha:8123/", "10.0.0.5", "10.0.0.6"],
                              ["wifi-pass", TOKEN, "admin-token", DASH_KEY])
        s = gather(args(), asker, defaults={}, lookup=lambda *a, **k: None)
        self.assertEqual(s, Settings(wifi="Home", wifi_passphrase="wifi-pass", ha_url="http://ha:8123",
                                     ha_token=TOKEN, dashcast="10.0.0.5:9555", dashcast_key=DASH_KEY,
                                     music_assistant="10.0.0.6", ha_admin_token="admin-token"))
        for line in s.summary():
            for secret in ("wifi-pass", TOKEN, "admin-token", DASH_KEY):
                self.assertNotIn(secret, line)

    def test_enter_takes_the_remembered_addresses_and_the_keyring(self):
        keyring = {"ha-token": TOKEN, "ha-admin-token": "admin-token", "dashcast-key": DASH_KEY,
                   "wifi": "wifi-pass"}
        asker, _ = self.asker(["", "", "", ""], [])
        defaults = {"wifi": "Home", "ha_url": "http://ha:8123", "dashcast": "10.0.0.5:9555", "music_assistant": "10.0.0.6"}
        s = gather(args(), asker, defaults=defaults, lookup=lambda secret, **attrs: keyring.get(secret))
        self.assertEqual((s.wifi, s.ha_url, s.dashcast, s.music_assistant),
                         ("Home", "http://ha:8123", "10.0.0.5:9555", "10.0.0.6"))
        self.assertEqual((s.wifi_passphrase, s.ha_token, s.dashcast_key), ("wifi-pass", TOKEN, DASH_KEY))

    def test_bad_answer_is_asked_again(self):
        asker, said = self.asker(["-", "ftp://ha.example", "http://ha.example", "-", "nope.example", "10.0.0.6"], ["", ""])
        s = gather(args(), asker, defaults={}, lookup=lambda *a, **k: None,
                   resolve=lambda h: (_ for _ in ()).throw(OSError("no such host")))
        self.assertEqual((s.wifi, s.ha_url, s.ha_token, s.dashcast, s.music_assistant),
                         (None, "http://ha.example", None, None, "10.0.0.6"))
        self.assertTrue(any("not a Home Assistant address" in line for line in said))
        self.assertTrue(any("cannot look up" in line for line in said))

    def test_dashcast_without_key_is_dropped(self):
        asker, said = self.asker(["-", "-", "10.0.0.5"], [""])
        s = gather(args(music_assistant="10.0.0.6"), asker, defaults={}, lookup=lambda *a, **k: None)
        self.assertIsNone(s.dashcast)
        self.assertEqual(s.music_assistant, "10.0.0.6")

    def test_switches_without_a_terminal(self):
        with tempfile.TemporaryDirectory() as td:
            token = pathlib.Path(td) / "t"
            token.write_text(TOKEN + "\n")
            s = gather(args(ha_url="http://ha:8123", ha_token_file=token, music_assistant="10.0.0.6"),
                       Asker(False), defaults={"dashcast": "10.0.0.5:9555"}, lookup=lambda *a, **k: None)
        self.assertEqual((s.ha_url, s.ha_token, s.dashcast, s.music_assistant, s.ha_admin_token),
                         ("http://ha:8123", TOKEN, None, "10.0.0.6", None))
        with self.assertRaises(SettingsError):
            gather(args(dashcast="bad host"), Asker(False), defaults={}, lookup=lambda *a, **k: None)

    def test_defaults_keep_no_secret(self):
        with tempfile.TemporaryDirectory() as td:
            path = pathlib.Path(td) / "jarvis-show" / "defaults.json"
            save_defaults(Settings(wifi="Home", wifi_passphrase="p", ha_url="http://ha:8123", ha_token=TOKEN,
                                   dashcast="10.0.0.5:9555", dashcast_key=DASH_KEY), path)
            text = path.read_text()
            for secret in ("\"p\"", TOKEN, DASH_KEY):
                self.assertNotIn(secret, text)
            self.assertEqual(load_defaults(path), {"wifi": "Home", "ha_url": "http://ha:8123", "dashcast": "10.0.0.5:9555"})

    def test_secret_files_are_private_and_go_away(self):
        with secret_files(Settings(ha_token=TOKEN, dashcast_key=DASH_KEY)) as files:
            self.assertEqual(set(files), {"ha_token", "dashcast_key"})
            for path in files.values():
                self.assertEqual(stat.S_IMODE(os.stat(path).st_mode), 0o600)
            self.assertEqual(files["ha_token"].read_text(), TOKEN + "\n")
            folder = files["ha_token"].parent
        self.assertFalse(folder.exists())


class InstallShowSettingsTests(unittest.TestCase):
    def test_device_settings_from_switches(self):
        with tempfile.TemporaryDirectory() as td:
            token = pathlib.Path(td) / "t"
            key = pathlib.Path(td) / "k"
            token.write_text(TOKEN + "\n")
            key.write_text(DASH_KEY + "\n")
            a = argparse.Namespace(ha_url="http://ha:8123/", ha_token_file=str(token), dashcast="10.0.0.5",
                                   dashcast_key_file=str(key), music_assistant="10.0.0.6")
            got = install.device_settings(a)
        self.assertEqual(got, {"ha_url": "http://ha:8123", "ha_token": TOKEN, "dashcast": "10.0.0.5:9555",
                               "dashcast_key": DASH_KEY, "music_assistant": "10.0.0.6"})
        with self.assertRaises(Fail):
            install.device_settings(argparse.Namespace(ha_url="http://ha:8123", ha_token_file=None, dashcast=None,
                                                       dashcast_key_file=None, music_assistant=None))

    def test_first_state_only_has_what_was_asked(self):
        self.assertEqual(install.initial_state(False, {}), {})
        self.assertEqual(install.initial_state(True, {"dashcast": "h:1", "dashcast_key": DASH_KEY, "music_assistant": "10.0.0.6"}),
                         {"security": {"ssh": True}, "dashboard": {"server": "h:1", "key": DASH_KEY},
                          "sendspin": {"enabled": True, "server_ip": "10.0.0.6"}})

    def test_write_once_keeps_a_file_that_is_there(self):
        with tempfile.TemporaryDirectory() as td:
            path = os.path.join(td, "state.json")
            text = json.dumps({"dashboard": {"server": "h:1", "key": "it's a key"}})
            cmd = install.write_once_cmd(path, text)
            import subprocess
            first = subprocess.run(["sh", "-c", cmd], capture_output=True, text=True).stdout.strip()
            again = subprocess.run(["sh", "-c", install.write_once_cmd(path, "{}")], capture_output=True, text=True).stdout.strip()
            self.assertEqual((first, again), ("WRITE-OK", "WRITE-KEPT"))
            self.assertEqual(json.loads(pathlib.Path(path).read_text()), json.loads(text))
            self.assertEqual(stat.S_IMODE(os.stat(path).st_mode), 0o600)


if __name__ == "__main__":
    unittest.main()


SHOW_SPEC = importlib.util.spec_from_file_location("jarvis_show_cli", TOOLS / "jarvis-show.py")
show_cli = importlib.util.module_from_spec(SHOW_SPEC)
SHOW_SPEC.loader.exec_module(show_cli)


class KeyFileTests(unittest.TestCase):
    """home-assistant finds the Show by the name the installer recorded, never by guessing."""

    def _args(self, name):
        return argparse.Namespace(key_file=None, serial=None, name=name)

    def test_finds_the_show_by_its_installed_name(self):
        with tempfile.TemporaryDirectory() as tmp:
            backups = pathlib.Path(tmp)
            record_show(backups / "AAAA05BJ", name="Jarvis Show 5", board="checkers")
            record_show(backups / "BBBB08TU", name="Kitchen", board="crown")
            (backups / "CCCC0003").mkdir()
            (backups / "CCCC0003" / "home-assistant.key").write_text("k\n")
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(show_cli._key_file(self._args("jarvis show 5"), backups),
                                 backups / "AAAA05BJ" / "home-assistant.key")

    def test_unknown_name_stops_and_lists_the_known_shows(self):
        with tempfile.TemporaryDirectory() as tmp:
            backups = pathlib.Path(tmp)
            record_show(backups / "BBBB08TU", name="Kitchen", board="crown")
            err = io.StringIO()
            with contextlib.redirect_stderr(err):
                self.assertIsNone(show_cli._key_file(self._args("Jarvis Show 5"), backups))
            self.assertIn("'Kitchen' (crown, serial ending 08TU)", err.getvalue())
            self.assertNotIn("BBBB08TU", err.getvalue())

    def test_two_shows_with_one_name_are_not_guessed(self):
        with tempfile.TemporaryDirectory() as tmp:
            backups = pathlib.Path(tmp)
            record_show(backups / "A1", name="Show", board="checkers")
            record_show(backups / "A2", name="Show", board="crown")
            self.assertIsNone(show_named(backups, "Show"))
