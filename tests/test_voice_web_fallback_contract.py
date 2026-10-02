from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]
ASSISTANT = ROOT / "echod/internal/feature/assistant/assistant.go"
WEB = ROOT / "echod/internal/feature/assistant/web.go"
BACKEND = ROOT / "echod/internal/feature/voice/backend.go"


class VoiceWebFallbackContractTests(unittest.TestCase):
    def test_information_questions_are_not_device_actions_by_default(self):
        src = ASSISTANT.read_text(encoding="utf-8")
        self.assertIn("Never turn an informational question into a device action", src)
        self.assertIn("food, recipes, definitions, facts, people, places or the web", src)
        self.assertIn("unless they explicitly ask to control something", src)

    def test_explicit_search_and_recipe_requests_use_searxng_when_configured(self):
        assistant = ASSISTANT.read_text(encoding="utf-8")
        web = WEB.read_text(encoding="utf-8")
        for phrase in ("search, look up, find online, find a source, or find a recipe", "use web_search"):
            self.assertIn(phrase, assistant)
        self.assertIn("Always use it when the person explicitly asks to search", web)
        self.assertIn("find a recipe", web)
        self.assertIn('Name: "web_search"', web)
        self.assertIn('Name: "read_page"', web)

    def test_automatic_brain_falls_back_only_between_turns_when_ha_is_unavailable(self):
        src = BACKEND.read_text(encoding="utf-8")
        self.assertIn("Automatic mode prefers a live Home", src)
        self.assertIn("falls back to the local Direct Brain only between turns", src)
        self.assertIn("if c.ha.Ready()", src)
        self.assertIn("if b.DirectReady()", src)

    def test_firmware_does_not_claim_to_override_a_live_ha_agent_decision(self):
        # This is intentional: a turn pinned to Home Assistant stays there. A bad Klar intent result
        # must be fixed/tested in HA/Klar rather than secretly reinterpreted by the satellite.
        src = BACKEND.read_text(encoding="utf-8")
        self.assertIn("pins", src)
        self.assertNotIn("reject", src.lower())


if __name__ == "__main__":
    unittest.main()
