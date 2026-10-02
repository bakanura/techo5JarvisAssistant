from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]


def text(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


class SecurityContractTests(unittest.TestCase):
    def test_release_workflow_runs_security_gate_before_fetch_or_build(self):
        workflow = text(".github/workflows/release.yml")
        gate = workflow.index("tools/security-regression.sh")
        fetch = workflow.index("tools/fetch-inputs.py")
        build = workflow.index("tools/linux/deploy-rootfs.sh")
        self.assertLess(gate, fetch)
        self.assertLess(gate, build)

    def test_ci_runs_security_gate(self):
        workflow = text(".github/workflows/build.yml")
        self.assertIn("tools/security-regression.sh", workflow)

    def test_security_suite_covers_runtime_trust_boundaries(self):
        script = text("tools/security-regression.sh")
        for package in (
            "./internal/feature/api",
            "./internal/feature/announce",
            "./internal/feature/mute",
            "./internal/feature/phone",
            "./internal/feature/security",
            "./internal/feature/sendspin",
            "./internal/feature/web",
            "./internal/update",
        ):
            self.assertIn(package, script)
        self.assertIn("cd dashcast", script)
        self.assertIn("go test .", script)

    def test_sendspin_is_source_ip_gated_and_off_without_pairing(self):
        listener = text("echod/internal/feature/sendspin/listen.go")
        tests = text("echod/internal/feature/sendspin/listen_test.go")
        self.assertIn("serverIP", listener)
        self.assertIn("net.ParseIP(server)", listener)
        self.assertIn("return false", listener)
        self.assertIn("TestListenerOnlyAdmitsPairedServerIP", tests)
        self.assertIn("TestUnpairedListenerAdmitsNobody", tests)

    def test_device_web_private_pages_require_authorized_setup_session(self):
        tests = text("echod/internal/feature/web/web_test.go")
        self.assertIn("TestPrivatePageNeedsAuthorizedSetupSession", tests)
        self.assertIn("TestClosedPrivatePageIsNotFoundBeforeAuthorization", tests)
        self.assertIn("TestHardenedHeadersAreApplied", tests)

    def test_house_traffic_rejects_plaintext_wrong_house_and_replay(self):
        announce = text("echod/internal/feature/announce/auth_test.go")
        intercom = text("echod/internal/feature/phone/intercom_test.go")
        self.assertIn("TestAnnouncementsAreSigned", announce)
        self.assertIn("TestPlaintextHouseWordIsRejected", announce)
        self.assertIn("the same announcement was taken twice", announce)
        self.assertIn("TestIntercomRefusesTheWrongWord", intercom)

    def test_dashcast_wrong_key_and_oversized_handshake_are_rejected(self):
        tests = text("dashcast/secure_test.go")
        self.assertIn("TestAWrongKeyFails", tests)
        self.assertIn("TestServerRefusesAnOversizedHandshake", tests)

    def test_update_path_requires_signature_identity_and_rollback(self):
        trust = text("echod/internal/update/trust_test.go")
        manifest = text("echod/internal/update/manifest_test.go")
        slot_tests = text("tests/test_slotctl_ab.py")
        self.assertIn("TestFetchNeedsTheSignatureAndTheClock", trust)
        self.assertIn("TestFetchRefusesSignedManifestForAnotherBoard", trust)
        self.assertIn("TestValidForBindsProductAndBoard", manifest)
        self.assertIn("test_failed_trial_consumes_three_boots_then_falls_back", slot_tests)
        self.assertIn("test_never_overwrites_running_slot", slot_tests)

    def test_installer_fail_closed_paths_are_security_gated(self):
        gate = text("tests/test_jarvis_crown_device_gate.py")
        unlock = text("tests/test_jarvis_crown_unlock.py")
        recovery = text("tests/test_jarvis_crown_recovery.py")
        self.assertIn("test_only_read_only_fastboot_commands_are_emitted", gate)
        self.assertIn("test_hash_mismatch_blocks_execution", unlock)
        self.assertIn("test_serial_change_after_exploit_fails_closed", unlock)
        self.assertIn("test_hash_mismatch_removes_partial_and_never_certifies_backup", recovery)

    def test_no_remote_adb_or_global_tls_bypass_controls_remain(self):
        diag_cfg = text("echod/internal/config/diag.go")
        diag_runtime = text("echod/internal/feature/diag/diag.go")
        self.assertIn("ClearLegacyRemoteADB", diag_cfg)
        self.assertIn("ClearLegacyInsecureTLS", diag_cfg)
        self.assertNotRegex(diag_cfg, r"func \(w DiagWriter\) (RemoteADB|InsecureTLS)\(")
        self.assertIn("Jarvis Show never exposes adb over the LAN", diag_runtime)
        self.assertIn("Jarvis Show always verifies certificates", diag_runtime)

        # Reolink is the deliberate exception to ordinary PKI: it pins a self-signed recorder
        # certificate by SHA-256 and verifies every subsequent connection against that pin. That is
        # not the removed global diagnostic TLS-bypass switch.
        reolink = text("echod/internal/lib/reolink/reolink.go")
        self.assertIn("Fingerprint", reolink)
        self.assertIn("VerifyConnection", reolink)
        self.assertIn("fingerprint(cs.PeerCertificates[0]) != c.Fingerprint", reolink)

    def test_firewall_policy_is_default_deny_and_names_required_ports(self):
        policy = text("docs/iot-firewall-policy.md")
        self.assertRegex(policy, r"(?i)default[- ]deny")
        for port in ("6053", "8928", "9555", "8123", "8181", "22", "5061"):
            self.assertIn(port, policy)
        self.assertIn("client isolation", policy.lower())
        self.assertIn("same IoT subnet", policy)


if __name__ == "__main__":
    unittest.main()
