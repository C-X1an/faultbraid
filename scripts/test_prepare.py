#!/usr/bin/env python3
"""Local preparation provenance controls; fixtures are not an upstream build."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath, PureWindowsPath
import subprocess
import tempfile
import unittest
import zipfile
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
    def test_tree_order_is_independent_of_path_flavor(self):
        """Mixed case, directory prefixes and case ties have a fixed order."""
        names = ['z.go', 'LICENSE', 'b.go', 'B/go.mod', 'a.go', 'A.go']
        expected = ['A.go', 'a.go', 'B/go.mod', 'b.go', 'LICENSE', 'z.go']
        for flavor, root_name in ((PurePosixPath, '/source'), (PureWindowsPath, 'C:/source')):
            with self.subTest(flavor=flavor.__name__):
                root = flavor(root_name)
                ordered = sorted((root / name for name in names),
                                 key=lambda path: prepare.tree_path_key(path, root))
                self.assertEqual([path.relative_to(root).as_posix() for path in ordered], expected)

    def test_tree_digest_preserves_pinned_component_order_and_bytes(self):
        """The aggregate hashes exact names/bytes in the portable pin order."""
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            fixtures = {'z.go': b'z\n', 'LICENSE': b'license\r\n',
                        'b.go': b'b\n', 'B/go.mod': b'module fixture\n', 'a.go': b'a\n'}
            for name, data in fixtures.items():
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(data)
            expected_bytes = (b'a.go\0a\n\0B/go.mod\0module fixture\n\0'
                              b'b.go\0b\n\0LICENSE\0license\r\n\0z.go\0z\n\0')
            self.assertEqual(prepare.tree_digest(root), hashlib.sha256(expected_bytes).hexdigest())

    def test_archive_fixture_ignores_inherited_autocrlf(self):
        """Real Git application in a non-Git source copy; upstream is a fixture."""
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            destination = root / ".local-runs/raft"
            patch_path = root / "fixture.patch"
            patch_path.write_bytes(b"diff --git a/source.go b/source.go\n--- a/source.go\n+++ b/source.go\n@@ -1 +1 @@\n-before\n+after\n")
            archive = root / "fixture.zip"
            with zipfile.ZipFile(archive, "w") as output:
                output.writestr("example.invalid/raft@v0.0.0/source.go", b"before\n")
            lock_path = root / "fixture-lock.json"
            lock_path.write_text(json.dumps({
                "schema": 1, "module": "example.invalid/raft", "version": "v0.0.0",
                "patch_sha256": prepare.sha256_file(patch_path),
                "upstream_zip_sha256": prepare.sha256_file(archive),
                "prepared_tree_sha256": hashlib.sha256(b"source.go\0after\n\0").hexdigest(),
            }), encoding="utf-8")
            self.assertFalse((root / ".git").exists())
            # No persistent Git/OS settings are modified. Only downloading is
            # replaced by this explicitly synthetic zip; real git apply runs.
            with patch.multiple(prepare, ROOT=root, DEST=destination,
                                LOCK_PATH=lock_path, PATCH_PATH=patch_path), \
                 patch.object(prepare, "download", return_value={"Zip": str(archive)}), \
                 patch.dict(os.environ, {"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.autocrlf", "GIT_CONFIG_VALUE_0": "true"}):
                self.assertEqual(prepare.main(), 0)
            self.assertEqual((destination / "source.go").read_bytes(), b"after\n")

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
