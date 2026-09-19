"""Synthetic package corruption tests; no cloud credentials or remote writes."""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

from check_snapshot import (MANIFEST, MANIFEST_CONTENT, PLATFORMS, SUMS,
                            archive_name, binary_name, isolated_environment, validate_packages)


class PackageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / MANIFEST).write_text(json.dumps(MANIFEST_CONTENT))
        for target in PLATFORMS:
            self.write_archive(target)
        self.write_checksums()

    def write_archive(self, target, extra=None, executable=True):
        with zipfile.ZipFile(self.root / archive_name(target), "w") as archive:
            entry = zipfile.ZipInfo(binary_name(target))
            entry.create_system = 3
            entry.external_attr = (0o100755 if executable else 0o100644) << 16
            archive.writestr(entry, b"synthetic binary: packaging validation only")
            archive.writestr("LICENSE", b"synthetic license")
            if extra:
                archive.writestr(extra, b"unexpected")

    def write_checksums(self):
        names = {archive_name(p) for p in PLATFORMS} | {MANIFEST}
        (self.root / SUMS).write_text("".join(
            f"{hashlib.sha256((self.root / name).read_bytes()).hexdigest()}  {name}\n"
            for name in sorted(names)))

    def test_complete_package(self):
        validate_packages(self.root)

    def test_install_environment_excludes_credentials_and_user_configuration(self):
        with patch.dict('os.environ', {'IWINV_SECRET_KEY': 'synthetic-secret',
                                      'TF_CLI_CONFIG_FILE': '/user/settings',
                                      'TF_PLUGIN_CACHE_DIR': '/user/cache',
                                      'TF_CLI_ARGS_init': '-plugin-dir=/user/plugins',
                                      'GITHUB_TOKEN': 'synthetic-token'}, clear=False):
            env = isolated_environment(self.root)
        self.assertEqual(env['HOME'], str(self.root))
        for key in ('IWINV_SECRET_KEY', 'TF_CLI_CONFIG_FILE', 'TF_PLUGIN_CACHE_DIR',
                    'TF_CLI_ARGS_init', 'GITHUB_TOKEN'):
            self.assertNotIn(key, env)

    def test_corrupt_and_missing_archives(self):
        archive = self.root / archive_name("linux_amd64")
        archive.write_bytes(archive.read_bytes() + b"changed after checksumming")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            validate_packages(self.root)
        archive.unlink()
        with self.assertRaises(ValueError):
            validate_packages(self.root)

    def test_manifest_protocol(self):
        (self.root / MANIFEST).write_text('{"version":1,"metadata":{"protocol_versions":["5.0"]}}')
        self.write_checksums()
        with self.assertRaisesRegex(ValueError, "manifest mismatch"):
            validate_packages(self.root)

    def test_checksum_duplicate_missing_and_path(self):
        original = (self.root / SUMS).read_text()
        for bad in (original + original.splitlines()[0] + "\n",
                    "\n".join(original.splitlines()[1:]),
                    "0" * 64 + "  ../outside\n"):
            with self.subTest(bad=bad[:64]):
                (self.root / SUMS).write_text(bad)
                with self.assertRaises(ValueError):
                    validate_packages(self.root)

    def test_extra_or_unsafe_member_and_mode(self):
        for extra in ("README.md", "../outside", "/absolute"):
            with self.subTest(extra=extra):
                self.write_archive("linux_amd64", extra=extra)
                self.write_checksums()
                with self.assertRaisesRegex(ValueError, "archive members"):
                    validate_packages(self.root)
        self.write_archive("linux_amd64", executable=False)
        self.write_checksums()
        with self.assertRaisesRegex(ValueError, "not executable"):
            validate_packages(self.root)


if __name__ == "__main__":
    unittest.main()
