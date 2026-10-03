import base64
import gzip
import hashlib
import importlib.util
import io
import pathlib
import shutil
import subprocess
import tarfile
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
FETCH_INPUTS = ROOT / "tools" / "fetch-inputs.py"

spec = importlib.util.spec_from_file_location("jarvis_fetch_inputs", FETCH_INPUTS)
fetch_inputs = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fetch_inputs)


def gzip_tar(entries):
    out = io.BytesIO()
    with gzip.GzipFile(fileobj=out, mode="wb", mtime=0) as gz:
        with tarfile.open(fileobj=gz, mode="w|") as tar:
            for name, body in entries:
                info = tarfile.TarInfo(name)
                info.size = len(body)
                info.mode = 0o644
                info.mtime = 0
                tar.addfile(info, io.BytesIO(body))
    return out.getvalue()


class SupplyChainTests(unittest.TestCase):
    def setUp(self):
        if shutil.which("openssl") is None:
            self.skipTest("openssl is required for the release-input signature test")
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = pathlib.Path(self.tmp.name)

    def make_signed_apk(self, *, trust_public_key=True, key_dir="etc/apk/keys"):
        private_key = self.dir / "private.pem"
        public_key = self.dir / "alpine-test.rsa.pub"
        subprocess.run(
            ["openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", private_key],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        subprocess.run(
            ["openssl", "pkey", "-in", private_key, "-pubout", "-out", public_key],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

        data_stream = gzip_tar([("usr/bin/example", b"trusted payload")])
        datahash = hashlib.sha256(data_stream).hexdigest().encode()
        control_stream = gzip_tar([(".PKGINFO", b"pkgname = example\ndatahash = " + datahash + b"\n")])
        signature = self.dir / "signature.bin"
        control_file = self.dir / "control.gz"
        control_file.write_bytes(control_stream)
        subprocess.run(
            ["openssl", "dgst", "-sha1", "-sign", private_key, "-out", signature, control_file],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        sig_stream = gzip_tar([
            (".SIGN.RSA.alpine-test.rsa.pub", signature.read_bytes()),
        ])
        apk = self.dir / "example-1.0-r0.apk"
        apk.write_bytes(sig_stream + control_stream + data_stream)

        keyring = self.dir / "alpine-minirootfs.tar.gz"
        key_entries = []
        if trust_public_key:
            key_entries.append((key_dir + "/alpine-test.rsa.pub", public_key.read_bytes()))
        keyring.write_bytes(gzip_tar(key_entries))

        checksum = "Q1" + base64.b64encode(hashlib.sha1(control_stream).digest()).decode()
        return apk, keyring, checksum

    def test_signed_apk_is_accepted_from_pinned_keyring(self):
        apk, keyring, checksum = self.make_signed_apk()
        old = fetch_inputs._alpine_keys_archive
        self.addCleanup(setattr, fetch_inputs, "_alpine_keys_archive", old)
        fetch_inputs._alpine_keys_archive = str(keyring)
        self.assertIsNone(fetch_inputs.apk_mismatch(str(apk), checksum))

    def test_host_apk_is_accepted_from_pinned_shared_keys(self):
        apk, keyring, checksum = self.make_signed_apk(key_dir="usr/share/apk/keys")
        old = fetch_inputs._alpine_keys_archive
        self.addCleanup(setattr, fetch_inputs, "_alpine_keys_archive", old)
        fetch_inputs._alpine_keys_archive = str(keyring)
        self.assertIsNone(fetch_inputs.apk_mismatch(str(apk), checksum))

    def test_shared_key_still_requires_valid_apk_signature(self):
        apk, keyring, _ = self.make_signed_apk(key_dir="usr/share/apk/keys")
        old = fetch_inputs._alpine_keys_archive
        self.addCleanup(setattr, fetch_inputs, "_alpine_keys_archive", old)
        fetch_inputs._alpine_keys_archive = str(keyring)
        segments = fetch_inputs.apk_segments(apk.read_bytes())
        forged = gzip_tar([(".PKGINFO", b"pkgname = forged\n")])
        apk.write_bytes(segments[0] + forged + segments[2])
        checksum = "Q1" + base64.b64encode(hashlib.sha1(forged).digest()).decode()
        self.assertIn("signature", fetch_inputs.apk_mismatch(str(apk), checksum))

    def test_tampered_apk_payload_is_rejected(self):
        apk, keyring, checksum = self.make_signed_apk()
        old = fetch_inputs._alpine_keys_archive
        self.addCleanup(setattr, fetch_inputs, "_alpine_keys_archive", old)
        fetch_inputs._alpine_keys_archive = str(keyring)
        data = bytearray(apk.read_bytes())
        data[-16] ^= 0x01
        apk.write_bytes(data)
        error = fetch_inputs.apk_mismatch(str(apk), checksum)
        self.assertIsNotNone(error)

    def test_apk_signed_by_untrusted_key_is_rejected(self):
        apk, keyring, checksum = self.make_signed_apk(trust_public_key=False)
        old = fetch_inputs._alpine_keys_archive
        self.addCleanup(setattr, fetch_inputs, "_alpine_keys_archive", old)
        fetch_inputs._alpine_keys_archive = str(keyring)
        error = fetch_inputs.apk_mismatch(str(apk), checksum)
        self.assertIn("signing key", error)

    def test_release_build_has_no_untrusted_apk_fallback(self):
        source = (ROOT / "tools/linux/mkrootfs.sh").read_text(encoding="utf-8")
        self.assertNotIn("--allow-untrusted", source)
        self.assertIn("refusing an untrusted release build", source)

    def test_fetcher_does_not_silently_substitute_package_versions(self):
        source = FETCH_INPUTS.read_text(encoding="utf-8")
        self.assertIn("update the package pin deliberately instead of substituting bytes", source)
        self.assertNotIn("taking %s", source)


if __name__ == "__main__":
    unittest.main()
