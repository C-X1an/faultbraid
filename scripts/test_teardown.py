"""Deletion tests use only marked synthetic temporary directories."""

import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("teardown.py")
NAME = "demo-20261011T120000-a1b2c3"
MARKER = ".faultbraid-owned"


class TeardownTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="faultbraid-teardown-")
        self.root = Path(self.temporary.name) / "public"
        self.root.mkdir()
        (self.root / "scripts").mkdir()
        (self.root / "scripts" / "teardown.py").write_bytes(SCRIPT.read_bytes())
        self.runs = self.root / ".local-runs"
        self.runs.mkdir()
        self.demo = self.runs / NAME
        self.demo.mkdir()
        (self.demo / MARKER).write_bytes(b"synthetic-demo-v1\n")
        (self.demo / "README.txt").write_bytes(b"Synthetic local kernel demo.\n")

    def tearDown(self):
        self.temporary.cleanup()

    def run_script(self):
        return subprocess.run(
            [sys.executable, str(self.root / "scripts" / "teardown.py")],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=20,
        )

    def test_valid_flat_demo_removed_evidence_preserved(self):
        evidence = self.runs / "evidence" / "run"
        evidence.mkdir(parents=True)
        (evidence / "stdout.txt").write_bytes(b"Synthetic command evidence.\n")
        result = self.run_script()
        self.assertEqual(result.returncode, 0)
        self.assertFalse(self.demo.exists())
        self.assertEqual((evidence / "stdout.txt").read_bytes(), b"Synthetic command evidence.\n")

    def test_hardlinked_marker_rejected_before_delete(self):
        (self.demo / MARKER).unlink()
        outside = Path(self.temporary.name) / "external-marker"
        outside.write_bytes(b"synthetic-demo-v1\n")
        os.link(outside, self.demo / MARKER)
        result = self.run_script()
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(self.demo.exists(), "hard-linked marker authorized an unsafe delete")
        self.assertEqual(outside.read_bytes(), b"synthetic-demo-v1\n")

    def test_hardlinked_demo_file_rejected_before_delete(self):
        outside = Path(self.temporary.name) / "external-data"
        outside.write_bytes(b"Synthetic external file.\n")
        os.link(outside, self.demo / "valid.json")
        result = self.run_script()
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(self.demo.exists())
        self.assertTrue(outside.exists())

    def test_marker_absent_or_wrong_preserves_directory(self):
        for value in (None, b"not-owned\n", b"synthetic-demo-v1\nextra\n", b"\xff"):
            marker = self.demo / MARKER
            if marker.exists():
                marker.unlink()
            if value is not None:
                marker.write_bytes(value)
            result = self.run_script()
            self.assertEqual(result.returncode, 0)
            self.assertTrue(self.demo.exists())

    def test_windows_text_marker_crlf_is_compatible(self):
        (self.demo / MARKER).write_bytes(b"synthetic-demo-v1\r\n")
        result = self.run_script()
        self.assertEqual(result.returncode, 0)
        self.assertFalse(self.demo.exists())

    def test_unknown_file_or_nested_evidence_preserved(self):
        (self.demo / "run.json").write_bytes(b"{\"synthetic\":true}\n")
        result = self.run_script()
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(self.demo.exists())
        (self.demo / "run.json").unlink()
        (self.demo / "evidence").mkdir()
        (self.demo / "evidence" / "stdout.txt").write_bytes(b"Synthetic retained evidence.\n")
        result = self.run_script()
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue((self.demo / "evidence" / "stdout.txt").exists())

    def test_non_tool_name_cannot_be_marked_for_delete(self):
        invalid = self.runs / "demo-user-files"
        self.demo.rename(invalid)
        result = self.run_script()
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(invalid.exists())


if __name__ == "__main__":
    unittest.main()
