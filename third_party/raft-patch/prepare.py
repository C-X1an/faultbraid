#!/usr/bin/env python3
"""Fetch, verify, and patch the locked instrumented etcd/raft source tree."""

from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path, PurePath, PurePosixPath
import shutil
import stat
import subprocess
import sys
import zipfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
LOCK_PATH = HERE / "lock.json"
PATCH_PATH = HERE / "election-random.patch"
DEST = ROOT / ".local-runs" / "raft"


def fail(message: str) -> "NoReturn":
    raise SystemExit(f"prepare raft: {message}")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def reject_alias(path: Path) -> None:
    """lstat works for dangling links and Python 3.11 Windows reparse points."""
    try:
        metadata = path.lstat()
    except FileNotFoundError:
        return
    if stat.S_ISLNK(metadata.st_mode) or (
        getattr(metadata, "st_file_attributes", 0)
        & getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0x400)
    ):
        fail(f"source path is a symbolic link or reparse point: {path}")


def owned_path(path: Path) -> None:
    """Check every existing component before following or removing it."""
    try:
        relative = path.relative_to(ROOT)
    except ValueError:
        fail(f"source path is outside the public workspace: {path}")
    current = ROOT
    reject_alias(current)
    for part in relative.parts:
        current /= part
        reject_alias(current)
    resolved = path.resolve(strict=False)
    resolved_root = ROOT.resolve(strict=True)
    if resolved_root != resolved and resolved_root not in resolved.parents:
        fail(f"source path resolves outside the public workspace: {path}")


def tree_path_key(path: PurePath, root: PurePath) -> tuple[tuple[str, ...], tuple[str, ...]]:
    """Keep the original pinned component order independent of host Path flavor.

    The pin was established with Windows' casefolded component comparison.
    Original component spelling breaks ties deterministically on other hosts.
    File names and file bytes are hashed verbatim; no normalization is applied.
    """
    parts = path.relative_to(root).parts
    return tuple(part.casefold() for part in parts), parts


def tree_digest(root: Path) -> str:
    reject_alias(root)
    if not root.is_dir():
        fail(f"source tree is not a directory: {root}")
    digest = hashlib.sha256()
    files: list[Path] = []
    # Inspect directories before descent so even an alias to matching bytes fails.
    for directory, directories, names in os.walk(root, followlinks=False):
        base = Path(directory)
        for name in directories + names:
            path = base / name
            reject_alias(path)
            if path.is_file():
                files.append(path)
    files.sort(key=lambda path: tree_path_key(path, root))
    for path in files:
        relative = path.relative_to(root).as_posix().encode("utf-8")
        digest.update(relative)
        digest.update(b"\0")
        digest.update(path.read_bytes())
        digest.update(b"\0")
    return digest.hexdigest()


def run(command: list[str], cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(
            command,
            cwd=cwd,
            check=True,
            text=True,
            encoding="utf-8",
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except FileNotFoundError as exc:
        fail(f"required executable not found: {command[0]}")
    except subprocess.CalledProcessError as exc:
        detail = (exc.stderr or exc.stdout).strip()
        fail(f"command failed ({' '.join(command)}): {detail}")


def download(lock: dict[str, object]) -> dict[str, object]:
    module = lock["module"]
    version = lock["version"]
    result = run(["go", "mod", "download", "-json", f"{module}@{version}"])
    try:
        metadata = json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        fail(f"go mod download returned invalid JSON: {exc}")
    required = {
        "Path": module,
        "Version": version,
        "Sum": lock["sum"],
        "GoModSum": lock["go_mod_sum"],
    }
    for key, expected in required.items():
        if metadata.get(key) != expected:
            fail(f"download {key}={metadata.get(key)!r}, expected {expected!r}")
    if not metadata.get("Zip"):
        fail("go mod download did not return a module zip")
    return metadata


def safe_extract(zip_path: Path, destination: Path, prefix: str) -> None:
    with zipfile.ZipFile(zip_path) as archive:
        for info in archive.infolist():
            name = info.filename
            if "\\" in name:
                fail(f"zip member contains backslash: {name!r}")
            pure = PurePosixPath(name)
            if pure.is_absolute() or ".." in pure.parts:
                fail(f"unsafe zip member path: {name!r}")
            prefix_parts = PurePosixPath(prefix).parts
            if pure.parts[: len(prefix_parts)] != prefix_parts:
                fail(f"zip member has unexpected prefix: {name!r}")
            mode = info.external_attr >> 16
            if stat.S_ISLNK(mode):
                fail(f"zip member is a symbolic link: {name!r}")
            relative = PurePosixPath(*pure.parts[len(prefix_parts) :])
            if not relative.parts:
                continue
            target = destination.joinpath(*relative.parts)
            resolved = target.resolve(strict=False)
            if destination.resolve() not in (resolved, *resolved.parents):
                fail(f"zip member escapes destination: {name!r}")
            if info.is_dir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            target.parent.mkdir(parents=True, exist_ok=True)
            with archive.open(info) as source, target.open("xb") as output:
                shutil.copyfileobj(source, output)


def main() -> int:
    lock = json.loads(LOCK_PATH.read_text(encoding="utf-8"))
    if lock.get("schema") != 1:
        fail("unsupported lock schema")
    if sha256_file(PATCH_PATH) != lock["patch_sha256"]:
        fail("patch SHA-256 does not match lock.json")
    owned_path(DEST)
    if os.path.lexists(DEST):
        if tree_digest(DEST) == lock["prepared_tree_sha256"]:
            print(f"instrumented raft already prepared: {DEST.relative_to(ROOT).as_posix()}")
            return 0
        fail(f"destination exists with unexpected contents: {DEST}")

    metadata = download(lock)
    zip_path = Path(str(metadata["Zip"]))
    if sha256_file(zip_path) != lock["upstream_zip_sha256"]:
        fail("upstream zip SHA-256 does not match lock.json")

    DEST.parent.mkdir(parents=True, exist_ok=True)
    temporary = DEST.parent / f".raft.prepare.{os.getpid()}"
    owned_path(temporary)
    if os.path.lexists(temporary):
        fail(f"temporary destination already exists: {temporary}")
    temporary.mkdir()
    try:
        prefix = f"{lock['module']}@{lock['version']}"
        safe_extract(zip_path, temporary, prefix)
        apply_directory = temporary.relative_to(ROOT).as_posix()
        run(
            ["git", "-c", "core.autocrlf=false", "apply", "--check", f"--directory={apply_directory}", str(PATCH_PATH)],
            cwd=ROOT,
        )
        run(
            ["git", "-c", "core.autocrlf=false", "apply", f"--directory={apply_directory}", str(PATCH_PATH)],
            cwd=ROOT,
        )
        actual = tree_digest(temporary)
        if actual != lock["prepared_tree_sha256"]:
            fail(f"prepared tree SHA-256 {actual}, expected {lock['prepared_tree_sha256']}")
        temporary.rename(DEST)
    finally:
        if temporary.exists():
            owned_path(temporary)
            # No recursive cleanup may follow a changed source alias.
            tree_digest(temporary)
            shutil.rmtree(temporary)

    print(f"prepared instrumented raft {lock['version']} at {DEST.relative_to(ROOT).as_posix()}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
