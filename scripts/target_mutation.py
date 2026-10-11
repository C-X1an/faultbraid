#!/usr/bin/env python3
"""Run a real-target persistence PASS / semantic FAIL / restored PASS control.

The sole mutation bypasses the production target's Store.Flush call. It is
applied only to a copied source tree. The same three-voter write/read test must
fail on its durability assertion; compilation errors and timeouts do not count.
All command streams and measured source identities are retained by run_evidence.
This is one selected integration defect, not a mutation-coverage percentage.
"""

from __future__ import annotations

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import sys
import uuid

from run_evidence import execute, source_identity


ROOT = Path(__file__).resolve().parents[1]
SITE = "internal/target/target.go"
OLD = b"if err := t.cfg.Store.Flush(); err != nil {"
NEW = b"if err := error(nil); err != nil {"
TEST = "TestAC013FlushBeforeSendAndResponse"
ASSERTION = "durability invariant: after-flush barrier preceded successful Store.Flush"
POSITIVE = "AC-013 Flush ordering positive control completed real three-voter write/read"
REQUIREMENTS = ["AC-013", "REL-002", "REL-001"]


def reject_alias_parents(path: Path) -> None:
    """Reject symbolic links and Windows reparse points before creating output."""
    for parent in reversed((path, *path.parents)):
        try:
            metadata = parent.lstat()
        except FileNotFoundError:
            continue
        if stat.S_ISLNK(metadata.st_mode) or (
            getattr(metadata, "st_file_attributes", 0)
            & getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0x400)
        ):
            raise ValueError("control output has a symbolic link or reparse-point parent")
        if not stat.S_ISDIR(metadata.st_mode):
            raise ValueError("control output parent is not a directory")


def diagnostics(record: dict) -> str:
    return "\n".join(
        Path(record[stream]["path"]).read_text(encoding="utf-8", errors="replace")
        for stream in ("stdout", "stderr")
    )


def ran_positive(record: dict) -> bool:
    text = diagnostics(record)
    return (
        record["status"] == "PASS"
        and record["reason"] == "completed"
        and record["exit_code"] == 0
        and "--- PASS: " + TEST in text
        and POSITIVE in text
    )


def copy_measured_source(before: dict, work: Path) -> None:
    """Copy measured regular source bytes; never copy Git metadata or old runs."""
    work.mkdir(mode=0o700)
    for entry in before["files"]:
        source = ROOT / entry["path"]
        destination = work / entry["path"]
        data = source.read_bytes()
        if hashlib.sha256(data).hexdigest() != entry["sha256"]:
            raise ValueError("source changed while the control copy was being made")
        destination.parent.mkdir(parents=True, exist_ok=True)
        with destination.open("xb") as output:
            output.write(data)
    if source_identity(work)["source_tree_sha256"] != before["source_tree_sha256"]:
        raise ValueError("isolated copy does not match the measured source")


def remove_control_git(work: Path) -> None:
    """Remove only this fresh copy's Git metadata after validating containment."""
    git = work / ".git"
    if not git.exists():
        return
    reject_alias_parents(git)
    resolved = git.resolve(strict=True)
    if (
        not work.resolve(strict=True).is_relative_to((ROOT / ".local-runs").resolve(strict=True))
        or resolved != work.resolve(strict=True) / ".git"
    ):
        raise ValueError("control Git cleanup target is outside the owned copy")
    for directory, directories, files in os.walk(git, followlinks=False):
        for name in directories + files:
            metadata = (Path(directory) / name).lstat()
            if stat.S_ISLNK(metadata.st_mode) or (
                getattr(metadata, "st_file_attributes", 0)
                & getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0x400)
            ):
                raise ValueError("control Git cleanup contains a symbolic link or reparse point")
    shutil.rmtree(git)


def run_control(out: Path, timeout: float) -> dict:
    before = source_identity(ROOT)
    # Retain a restored copy in a short ignored path. Windows archive extraction
    # can exceed pathname limits if the copy is nested under the evidence ledger.
    # The original checkout and Git configuration remain untouched. Only the
    # copied tree's fresh Git metadata is removed after containment checks.
    control_id = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%S") + "-" + uuid.uuid4().hex[:10]
    folder = out / control_id
    folder.mkdir(mode=0o700)
    work = ROOT / ".local-runs" / ("tm-" + uuid.uuid4().hex[:8])
    reject_alias_parents(work.parent)
    work.parent.mkdir(exist_ok=True)
    summary = {
        "schema_version": 1,
        "control_id": control_id,
        "status": "FAIL",
        "scope": "one selected skipped-Store.Flush integration mutation; not overall mutation coverage",
        "mutation": "skip_ready_store_flush",
        "site": SITE,
        "test": TEST,
        "source_before": before,
        "isolated_source": work.relative_to(ROOT).as_posix(),
        "runs": [],
        "source_original_unchanged": False,
        "isolated_copy_restored": False,
        "isolated_git_removed": False,
        "killed_by_semantic_assertion": False,
    }
    original = None
    site = work / SITE
    try:
        copy_measured_source(before, work)
        original = site.read_bytes()
        if original.count(OLD) != 1:
            raise ValueError("mutation site changed; inspect rather than silently skipping the control")
        mutated_bytes = original.replace(OLD, NEW, 1)
        summary["original_site_sha256"] = hashlib.sha256(original).hexdigest()
        summary["mutated_site_sha256"] = hashlib.sha256(mutated_bytes).hexdigest()
        evidence = folder / "evidence"
        # The short ignored copy is inside this checkout. Establish its own Git
        # root so git apply cannot discover the enclosing repository and resolve
        # the preparer's relative destination against that different checkout.
        initialized = execute(
            ["git", "init", "--quiet"], work, evidence,
            "target-mutation-isolation", REQUIREMENTS, timeout,
        )
        summary["runs"].append(initialized)
        if initialized["status"] != "PASS":
            raise ValueError("isolated Git root could not be established")
        prepare = execute(
            [sys.executable, "third_party/raft-patch/prepare.py"], work, evidence,
            "target-mutation-prepare", REQUIREMENTS, timeout,
        )
        summary["runs"].append(prepare)
        if prepare["status"] != "PASS":
            raise ValueError("locked target preparation did not pass; no semantic control result")
        command = ["go", "test", "-count=1", "-v", "-run", "^" + TEST + "$", "./tests/system"]
        baseline = execute(command, work, evidence, "target-mutation-baseline", REQUIREMENTS, timeout)
        summary["runs"].append(baseline)
        if not ran_positive(baseline):
            raise ValueError("the original target did not execute the positive control successfully")
        site.write_bytes(mutated_bytes)
        mutated = execute(command, work, evidence, "target-mutation-negative-control", REQUIREMENTS, timeout)
        summary["runs"].append(mutated)
        text = diagnostics(mutated)
        killed = (
            mutated["status"] == "FAIL"
            and mutated["reason"] == "completed"
            and mutated["exit_code"] == 1
            and "--- FAIL: " + TEST in text
            and ASSERTION in text
            and "[build failed]" not in text
            and "panic:" not in text
        )
        summary["killed_by_semantic_assertion"] = killed
        site.write_bytes(original)
        restored = execute(command, work, evidence, "target-mutation-restored", REQUIREMENTS, timeout)
        summary["runs"].append(restored)
        if not killed:
            raise ValueError("the selected defect was not killed by the declared semantic assertion")
        if not ran_positive(restored):
            raise ValueError("the restored target did not execute the positive control successfully")
    except (OSError, ValueError) as error:
        summary["error"] = str(error)
    finally:
        if original is not None:
            # Restore exact original bytes even if preparation or execution fails.
            site.write_bytes(original)
        try:
            remove_control_git(work)
            summary["isolated_git_removed"] = True
        except (OSError, ValueError) as error:
            summary["error"] = "isolated Git metadata cleanup failed: " + str(error)
        after = source_identity(ROOT)
        summary["source_after_sha256"] = after["source_tree_sha256"]
        summary["source_original_unchanged"] = before["source_tree_sha256"] == after["source_tree_sha256"]
        summary["isolated_copy_restored"] = (
            work.is_dir()
            and before["source_tree_sha256"] == source_identity(work)["source_tree_sha256"]
        )
        if (
            "error" not in summary
            and summary["killed_by_semantic_assertion"]
            and summary["source_original_unchanged"]
            and summary["isolated_copy_restored"]
            and summary["isolated_git_removed"]
        ):
            summary["status"] = "PASS"
        elif any(run["status"] == "BLOCKED" for run in summary["runs"]):
            summary["status"] = "BLOCKED"
        with (folder / "summary.json").open("x", encoding="utf-8") as output:
            json.dump(summary, output, indent=2)
            output.write("\n")
    return summary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=ROOT / ".local-runs" / "target-mutation")
    parser.add_argument("--timeout", type=float, default=180)
    args = parser.parse_args()
    if not 0 < args.timeout <= 1800:
        parser.error("timeout must be positive and at most 1800 seconds")
    out = Path(os.path.abspath(args.out))
    try:
        reject_alias_parents(out)
        if out.is_relative_to(ROOT) and not out.is_relative_to(ROOT / ".local-runs"):
            raise ValueError("an in-tree output must be inside ignored .local-runs")
        out.mkdir(parents=True, exist_ok=True)
        result = run_control(out, args.timeout)
    except (OSError, ValueError) as error:
        print("target persistence control failed: " + str(error), file=sys.stderr)
        return 1
    print(json.dumps({key: result[key] for key in (
        "control_id", "status", "killed_by_semantic_assertion",
        "source_original_unchanged", "isolated_copy_restored",
        "isolated_git_removed",
    )}))
    return 0 if result["status"] == "PASS" else 125 if result["status"] == "BLOCKED" else 1


if __name__ == "__main__":
    sys.exit(main())
