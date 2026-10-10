#!/usr/bin/env python3
"""Local preparation provenance controls; fixtures are not an upstream build."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / "third_party/raft-patch/prepare.py"
SPEC = importlib.util.spec_from_file_location("raft_prepare", SCRIPT)
prepare = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(prepare)


def directory_alias(alias, target):
    if os.name == "nt":
        subprocess.run(["cmd", "/c", "mklink", "/J", str(alias), str(target)],
                       check=True, capture_output=True)
    else:
        alias.symlink_to(target, target_is_directory=True)


class PrepareTests(unittest.TestCase):
    def test_exact_regular_fixture_is_accepted(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            destination = root / ".local-runs/raft"
            destination.mkdir(parents=True)
            (destination / "source.go").write_bytes(b"fixture source\n")
            patch_path = root / "fixture.patch"
            patch_path.write_bytes(b"fixture patch\n")
            lock_path = root / "fixture-lock.json"
            lock_path.write_text(json.dumps({
                "schema": 1, "patch_sha256": prepare.sha256_file(patch_path),
                "prepared_tree_sha256": prepare.tree_digest(destination),
            }), encoding="utf-8")
            with patch.multiple(prepare, ROOT=root, DEST=destination,
                                LOCK_PATH=lock_path, PATCH_PATH=patch_path):
                self.assertEqual(prepare.main(), 0)
                (destination / "source.go").write_bytes(b"changed source\n")
                with self.assertRaisesRegex(SystemExit, "unexpected contents"):
                    prepare.main()

    def test_aliases_are_rejected_even_when_bytes_match(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            external = root / "external"
            external.mkdir()
            (external / "source.go").write_bytes(b"fixture source\n")
            regular_digest = prepare.tree_digest(external)
            self.assertEqual(regular_digest,
                             hashlib.sha256(b"source.go\0fixture source\n\0").hexdigest())
            alias = root / "alias"
            directory_alias(alias, external)
            with self.assertRaisesRegex(SystemExit, "symbolic link or reparse point"):
                prepare.tree_digest(alias)
            owned = root / "owned"
            owned.mkdir()
            directory_alias(owned / "nested", external)
            with self.assertRaisesRegex(SystemExit, "symbolic link or reparse point"):
                prepare.tree_digest(owned)
            with patch.object(prepare, "ROOT", root):
                with self.assertRaisesRegex(SystemExit, "symbolic link or reparse point"):
                    prepare.owned_path(alias / "source.go")

    def test_outside_workspace_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder) / "workspace"
            root.mkdir()
            with patch.object(prepare, "ROOT", root):
                with self.assertRaisesRegex(SystemExit, "outside the public workspace"):
                    prepare.owned_path(Path(folder) / "unowned")


if __name__ == "__main__":
    unittest.main()
