"""Synthetic privacy fixtures; no real credentials or private source material."""

import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest
import zipfile


SCRIPT = Path(__file__).with_name("public_audit.py")


class PublicAuditTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="faultbraid-audit-")
        self.root = Path(self.temporary.name) / "public"
        self.root.mkdir()
        (self.root / "README.md").write_bytes(b"Synthetic public engineering fixture.\n")
        self.inventory(["README.md", "PUBLIC_FILES.txt"])
        self.git("init", "-q")
        self.git("config", "user.name", "Synthetic audit fixture")
        self.git("config", "user.email", "fixture@example.invalid")
        self.commit()

    def tearDown(self):
        self.temporary.cleanup()

    def git(self, *arguments, data=None):
        result = subprocess.run(
            ["git", "-C", str(self.root), *arguments], input=data,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=20,
        )
        if result.returncode:
            self.fail("synthetic Git fixture command failed")
        return result.stdout

    def commit(self):
        self.git("add", "-A")
        self.git("commit", "-qm", "Synthetic audit fixture state")

    def inventory(self, names):
        (self.root / "PUBLIC_FILES.txt").write_bytes(("\n".join(sorted(names)) + "\n").encode())

    def audit(self, *arguments):
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root), *map(str, arguments)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60,
        )
        try:
            payload = json.loads(result.stdout)
        except ValueError:
            payload = None
        return result, payload

    def rejected(self, *arguments):
        result, payload = self.audit(*arguments)
        self.assertNotEqual(result.returncode, 0, "unsafe synthetic export was accepted")
        self.assertIsInstance(payload, dict, "rejection must retain structured audit evidence")
        self.assertEqual(payload["status"], "FAIL")
        self.assertTrue(payload["errors"])
        return result, payload

    def test_clean_real_git_history_and_inventory(self):
        result, payload = self.audit()
        self.assertEqual(result.returncode, 0)
        self.assertEqual(payload["status"], "PASS")
        self.assertEqual(payload["files"], 2)
        self.assertEqual(payload["history"]["commits"], 1)
        self.assertEqual(payload["history"]["blobs"], 2)
        self.assertEqual(len(payload["inventory_sha256"]), 64)

    def test_deleted_historical_secret_is_rejected(self):
        # Construct the synthetic corpus at runtime; no complete credential is
        # stored in this test's source or echoed by assertion diagnostics.
        value = ("gh" + "p_" + "A" * 36).encode()
        (self.root / "old.txt").write_bytes(value)
        self.git("add", "old.txt")
        self.git("commit", "-qm", "Synthetic historical defect")
        (self.root / "old.txt").unlink()
        self.commit()
        result, _ = self.rejected()
        self.assertNotIn(value, result.stdout + result.stderr)

    def test_nested_annotated_tag_message_is_scanned(self):
        value = ("gh" + "p_" + "A" * 36).encode()
        self.git("tag", "-a", "inner", "-F", "-", data=value)
        self.git("-c", "advice.nestedTag=false", "tag", "-a", "outer", "-m", "Synthetic outer annotation", "inner")
        self.git("tag", "-d", "inner")
        result, payload = self.rejected()
        self.assertFalse(payload["history"]["complete"])
        self.assertEqual(payload["history"]["tags"], 2)
        self.assertNotIn(value, result.stdout + result.stderr)

    def test_tagged_and_direct_ref_blobs_are_scanned(self):
        value = ("gh" + "p_" + "B" * 36).encode()
        oid = self.git("hash-object", "-w", "--stdin", data=value).decode().strip()
        self.git("tag", "-a", "blob-annotation", "-m", "Synthetic blob annotation", oid)
        result, payload = self.rejected()
        self.assertFalse(payload["history"]["complete"])
        self.assertNotIn(value, result.stdout + result.stderr)
        self.git("tag", "-d", "blob-annotation")
        self.git("update-ref", "refs/tags/direct-blob", oid)
        result, payload = self.rejected()
        self.assertFalse(payload["history"]["complete"])
        self.assertNotIn(value, result.stdout + result.stderr)

    def test_tree_ref_is_rejected_without_claiming_complete_history(self):
        oid = self.git("rev-parse", "HEAD^{tree}").decode().strip()
        self.git("tag", "-a", "tree-annotation", "-m", "Synthetic unsupported tree annotation", oid)
        _, payload = self.rejected()
        self.assertFalse(payload["history"]["complete"])

    def test_environment_file_case_spellings_rejected_current_and_history(self):
        name = ".ENV.local"
        file = self.root / name
        file.write_bytes(b"SYNTHETIC_FIXTURE=1\n")
        self.inventory(["README.md", "PUBLIC_FILES.txt", name])
        self.rejected()
        self.commit()
        file.unlink()
        self.inventory(["README.md", "PUBLIC_FILES.txt"])
        self.commit()
        self.rejected()

    def test_exact_environment_example_exception_is_safe(self):
        (self.root / ".env.example").write_bytes(b"SYNTHETIC_PLACEHOLDER=\n")
        self.inventory(["README.md", "PUBLIC_FILES.txt", ".env.example"])
        self.commit()
        result, payload = self.audit()
        self.assertEqual(result.returncode, 0)
        self.assertEqual(payload["status"], "PASS")

    def test_deleted_historical_private_path_is_rejected(self):
        private = self.root / ("pri" + "vate")
        private.mkdir()
        file = private / "notes.md"
        file.write_bytes(b"Synthetic omitted private path.\n")
        self.git("add", "-f", str(file.relative_to(self.root)))
        self.git("commit", "-qm", "Synthetic historical path defect")
        file.unlink()
        private.rmdir()
        self.commit()
        self.rejected()

    def test_current_content_corpus_is_rejected_without_echo(self):
        corpus = [
            "AK" + "IA" + "A" * 16,
            "AS" + "IA" + "B" * 16,
            "git" + "hub_pat_" + "C" * 60,
            "sk-" + "proj-" + "D" * 48,
            "-----BEGIN " + "PRIVATE KEY-----",
            "AI" + "za" + "E" * 35,
            "xox" + "b-" + "F" * 28,
            "# " + "Curriculum" + " Vitae\nSynthetic private content",
            "portfolio_" + "profile: synthetic",
            "candidate_" + "scores: synthetic",
            "# Synthetic autonomous " + "implementation contract\n",
            json.dumps({"ro" + "le": "sys" + "tem", "con" + "tent": "synthetic session material"}),
            "C:" + "\\Users\\" + "SyntheticOwner\\source",
        ]
        for value in corpus:
            with self.subTest(corpus_sha=__import__("hashlib").sha256(value.encode()).hexdigest()[:12]):
                (self.root / "README.md").write_bytes(value.encode())
                result, _ = self.rejected()
                self.assertNotIn(value.encode(), result.stdout + result.stderr)

    def test_ordinary_privacy_documentation_is_allowed(self):
        (self.root / "README.md").write_bytes(
            b"Do not publish private/, credentials, resumes, candidate research or raw agent sessions.\n"
        )
        self.commit()
        result, payload = self.audit()
        self.assertEqual(result.returncode, 0)
        self.assertEqual(payload["status"], "PASS")

    def test_invalid_allowlist_spellings_and_duplicates(self):
        variants = [
            ["README.md", "README.md", "PUBLIC_FILES.txt"],
            ["README.md", "readme.md", "PUBLIC_FILES.txt"],
            ["./README.md", "PUBLIC_FILES.txt"],
            ["a/../README.md", "PUBLIC_FILES.txt"],
            ["a//b", "PUBLIC_FILES.txt"],
            ["a\\b", "PUBLIC_FILES.txt"],
            ["/README.md", "PUBLIC_FILES.txt"],
            ["C:/README.md", "PUBLIC_FILES.txt"],
            ["README.md ", "PUBLIC_FILES.txt"],
            ["README.md.", "PUBLIC_FILES.txt"],
            ["CON.txt", "PUBLIC_FILES.txt"],
            ["LPT1.txt", "PUBLIC_FILES.txt"],
            ["bad\x01.txt", "PUBLIC_FILES.txt"],
            ["e\u0301.md", "PUBLIC_FILES.txt"],
            ["README.md"],
        ]
        for names in variants:
            with self.subTest(variant=variants.index(names)):
                self.inventory(names)
                self.rejected()

    def test_unapproved_and_missing_files(self):
        (self.root / "extra.md").write_bytes(b"Synthetic unapproved source.\n")
        self.rejected()
        (self.root / "extra.md").unlink()
        (self.root / "README.md").unlink()
        self.rejected()

    def test_private_names_cannot_be_allowlisted(self):
        for name in ["pri" + "vate/notes.md", "resume" + "_extracted.txt", "candidate_" + "research.md", ".env", ".local-runs/leak.txt"]:
            self.inventory(["README.md", "PUBLIC_FILES.txt", name])
            self.rejected()

    def test_invalid_utf8_and_bounded_size(self):
        for data in [b"\xff", b"A" * (8 * 1024 * 1024 + 1)]:
            (self.root / "README.md").write_bytes(data)
            self.rejected()

    def test_regular_file_hardlink_rejected(self):
        outside = Path(self.temporary.name) / "external.md"
        (self.root / "README.md").unlink()
        outside.write_bytes(b"Synthetic hard-linked content.\n")
        os.link(outside, self.root / "README.md")
        self.rejected()

    def test_git_symlink_mode_rejected_even_without_os_symlink(self):
        oid = self.git("hash-object", "-w", "--stdin", data=b"README.md").decode().strip()
        self.git("update-index", "--add", "--cacheinfo", "120000," + oid + ",link")
        self.git("commit", "-qm", "Synthetic historical alias defect")
        self.git("rm", "--cached", "link")
        self.git("commit", "-qm", "Remove synthetic alias")
        self.rejected()

    def test_non_repository_and_shallow_history_fail_closed(self):
        metadata = self.root / ".git"
        metadata.rename(self.root / "git-metadata")
        self.rejected()
        metadata = self.root / "git-metadata"
        metadata.rename(self.root / ".git")
        head = self.git("rev-parse", "HEAD").strip()
        (self.root / ".git" / "shallow").write_bytes(head + b"\n")
        self.rejected()

    def test_deterministic_reviewed_export_and_exact_archive_audit(self):
        one = Path(self.temporary.name) / "one.zip"
        two = Path(self.temporary.name) / "two.zip"
        for path in (one, two):
            result, payload = self.audit("--export", path)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(payload["archive"]["status"], "PASS")
        self.assertEqual(one.read_bytes(), two.read_bytes())
        result, payload = self.audit("--archive", one)
        self.assertEqual(result.returncode, 0)
        self.assertEqual(payload["archive"]["files"], 2)
        self.rejected("--export", one)

    def archive(self, entries):
        path = Path(self.temporary.name) / "unsafe.zip"
        with zipfile.ZipFile(path, "w") as archive:
            for name, data, mode in entries:
                info = zipfile.ZipInfo(name)
                info.create_system = 3
                info.external_attr = mode << 16
                archive.writestr(info, data)
        return path

    def test_archive_traversal_duplicates_aliases_and_content(self):
        originals = [(name, (self.root / name).read_bytes(), stat.S_IFREG | 0o644) for name in ["PUBLIC_FILES.txt", "README.md"]]
        bad = [
            [("../outside.md", b"synthetic", stat.S_IFREG | 0o644)],
            [("C:/outside.md", b"synthetic", stat.S_IFREG | 0o644)],
            [("a\\b", b"synthetic", stat.S_IFREG | 0o644)],
            [originals[0], originals[0], originals[1]],
            [("README.md", b"outside.md", stat.S_IFLNK | 0o777), originals[0]],
            [("README.md", b"changed payload", stat.S_IFREG | 0o644), originals[0]],
            [("README.md", ("gh" + "p_" + "Z" * 36).encode(), stat.S_IFREG | 0o644), originals[0]],
            [originals[0]],
            [*originals, ("extra.md", b"synthetic", stat.S_IFREG | 0o644)],
        ]
        for index, entries in enumerate(bad):
            with self.subTest(variant=index):
                self.rejected("--archive", self.archive(entries))


if __name__ == "__main__":
    unittest.main()
