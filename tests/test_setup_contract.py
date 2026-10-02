from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
SETUP = ROOT / "echod/internal/feature/setup"


def source(name: str) -> str:
    return (SETUP / name).read_text(encoding="utf-8")


class SetupPageContractTests(unittest.TestCase):
    def test_every_rendered_form_action_has_a_save_handler(self):
        rendered = set()
        for path in SETUP.glob("*.go"):
            if path.name.endswith("_test.go"):
                continue
            rendered.update(re.findall(r'hidden\(w, token, "([^"]+)"', path.read_text(encoding="utf-8")))

        page = source("page.go")
        direct = set(re.findall(r'case "([^"]+)"', page))
        grouped = set()
        for group in re.findall(r'case ((?:"[^"]+"(?:,\s*)?)+):', page):
            grouped.update(re.findall(r'"([^"]+)"', group))
        handled = direct | grouped

        alarms = source("alarms.go")
        alarm_actions = {"alarm_add", "alarm_edit", "alarm_prefs", "snoozes_cancel", "timer_cancel"}
        for action in alarm_actions:
            self.assertIn(action, alarms)
            handled.add(action)

        self.assertEqual(set(), rendered - handled, f"rendered setup actions without a handler: {sorted(rendered-handled)}")

    def test_every_settings_form_posts_to_the_single_csrf_checked_endpoint(self):
        for path in SETUP.glob("*.go"):
            if path.name.endswith("_test.go"):
                continue
            text = path.read_text(encoding="utf-8")
            for form in re.findall(r'<form[^>]*>', text):
                if 'method="post"' in form and '/setup/wait' not in form:
                    self.assertIn('action="/setup/save"', form, f"{path.name}: {form}")

    def test_setup_save_checks_session_token_before_dispatch(self):
        page = source("page.go")
        token_check = page.index('r.PostFormValue("token") != token')
        dispatch = page.index('switch what := r.PostFormValue("what")')
        self.assertLess(token_check, dispatch)

    def test_show_build_does_not_claim_an_action_button_exists(self):
        codes = (ROOT / "echod/internal/hardware/buttons/codes_cronos.go").read_text(encoding="utf-8")
        self.assertNotIn("Action,", codes)
        page = source("page.go")
        updates = source("updates.go")
        self.assertIn("buttons.HasAction()", page)
        self.assertIn("tap Allow on the device screen", page)
        self.assertIn("open Setup page again from this device's Settings screen", page)
        self.assertNotIn("press the action button again", updates)

    def test_management_routes_have_explicit_method_guards(self):
        page = source("page.go")
        self.assertIn('"get the setup page"', page)
        self.assertIn('"get setup state"', page)
        self.assertIn('"get diagnostics"', page)
        self.assertIn('"post to ask"', page)
        self.assertIn('"post to save"', page)

    def test_setup_wait_cookie_remains_private(self):
        page = source("page.go")
        self.assertIn("HttpOnly: true", page)
        self.assertIn("SameSite: http.SameSiteStrictMode", page)
        self.assertIn('Path: "/"', page)


if __name__ == "__main__":
    unittest.main()
