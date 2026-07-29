#!/usr/bin/env python3
"""Focused CLI integration tests for Trellis task readiness gates."""

from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


SOURCE_ROOT = Path(__file__).resolve().parents[1]


class TaskGateCLITest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        scripts = self.root / ".trellis" / "scripts"
        shutil.copytree(SOURCE_ROOT / ".trellis" / "scripts", scripts)
        (self.root / ".trellis" / "config.yaml").write_text(
            "codex:\n  dispatch_mode: auto\n", encoding="utf-8"
        )
        self.task_dir = self.root / ".trellis" / "tasks" / "fixture"
        self.task_dir.mkdir(parents=True)
        (self.root / "docs" / "plans").mkdir(parents=True)
        (self.root / "docs" / "plans" / "spec.md").write_text("# spec\n", encoding="utf-8")
        (self.root / "docs" / "plans" / "ticket.md").write_text(
            "# ticket\n\n**Status:** ready-for-agent\n", encoding="utf-8"
        )
        for name in ("prd.md", "design.md", "implement.md"):
            (self.task_dir / name).write_text("# artifact\n", encoding="utf-8")
        (self.task_dir / "task.json").write_text(
            json.dumps(
                {
                    "status": "planning",
                    "branch": "codex/fixture",
                    "base_branch": "main",
                    "meta": {
                        "risk": "behavioral",
                        "fixed_point": "0123456789abcdef",
                        "source_of_truth": {
                            "type": "docs-ticket",
                            "spec": "docs/plans/spec.md",
                            "ticket": "docs/plans/ticket.md",
                        },
                    },
                }
            ),
            encoding="utf-8",
        )
        for name in ("implement.jsonl", "check.jsonl"):
            (self.task_dir / name).write_text('{"_example": "curate me"}\n', encoding="utf-8")

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def run_task(self, *args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["python3", ".trellis/scripts/task.py", *args],
            cwd=self.root,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_ready_rejects_seed_only_context(self) -> None:
        result = self.run_task("validate", "fixture", "--ready")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("seed-only", result.stdout + result.stderr)

    def test_ready_rejects_missing_branch(self) -> None:
        data = json.loads((self.task_dir / "task.json").read_text(encoding="utf-8"))
        data["branch"] = None
        (self.task_dir / "task.json").write_text(json.dumps(data), encoding="utf-8")

        result = self.run_task("validate", "fixture", "--ready")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("branch is required", result.stdout + result.stderr)

    def test_ready_rejects_main_as_task_branch(self) -> None:
        data = json.loads((self.task_dir / "task.json").read_text(encoding="utf-8"))
        data["branch"] = "main"
        (self.task_dir / "task.json").write_text(json.dumps(data), encoding="utf-8")

        result = self.run_task("validate", "fixture", "--ready")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("must not be main", result.stdout + result.stderr)

    def test_ready_rejects_missing_context_path(self) -> None:
        for name in ("implement.jsonl", "check.jsonl"):
            (self.task_dir / name).write_text(
                '{"file": "docs/plans/missing.md", "reason": "fixture"}\n', encoding="utf-8"
            )
        (self.task_dir / "quality-evidence.json").write_text(
            json.dumps({"schema": 1, "approval": {"result": "passed"}}), encoding="utf-8"
        )

        result = self.run_task("validate", "fixture", "--ready")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("no valid curated entries", result.stdout + result.stderr)

    def test_ready_rejects_source_ticket_not_ready_for_agent(self) -> None:
        (self.root / "docs" / "plans" / "ticket.md").write_text(
            "# ticket\n\n**Status:** planning\n", encoding="utf-8"
        )

        result = self.run_task("validate", "fixture", "--ready")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("ready-for-agent", result.stdout + result.stderr)

    def test_ready_rejects_source_outside_docs_plans(self) -> None:
        (self.root / "docs" / "outside.md").write_text("# outside\n", encoding="utf-8")
        data = json.loads((self.task_dir / "task.json").read_text(encoding="utf-8"))
        data["meta"]["source_of_truth"]["spec"] = "docs/plans/../outside.md"
        (self.task_dir / "task.json").write_text(json.dumps(data), encoding="utf-8")

        result = self.run_task("validate", "fixture", "--ready")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("docs/plans", result.stdout + result.stderr)

    def test_forced_start_requires_and_records_reason(self) -> None:
        result = self.run_task("start", "fixture", "--force")

        self.assertNotEqual(result.returncode, 0)
        data = json.loads((self.task_dir / "task.json").read_text(encoding="utf-8"))
        self.assertEqual("planning", data["status"])

        result = self.run_task("start", "fixture", "--force", "--reason", "incident response")

        self.assertEqual(0, result.returncode, result.stdout + result.stderr)
        data = json.loads((self.task_dir / "task.json").read_text(encoding="utf-8"))
        self.assertEqual("in_progress", data["status"])
        self.assertEqual("incident response", data["meta"]["gate_overrides"][-1]["reason"])

    def test_archive_rejects_incomplete_task_without_mutation(self) -> None:
        result = self.run_task("archive", "fixture", "--no-commit")

        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(self.task_dir.is_dir())
        self.assertFalse((self.root / ".trellis" / "tasks" / "archive").exists())
        data = json.loads((self.task_dir / "task.json").read_text(encoding="utf-8"))
        self.assertEqual("planning", data["status"])

    def make_complete(self) -> None:
        for name in ("implement.jsonl", "check.jsonl"):
            (self.task_dir / name).write_text(
                '{"file": "docs/plans/spec.md", "reason": "fixture"}\n', encoding="utf-8"
            )
        (self.task_dir / "quality-evidence.json").write_text(
            json.dumps(
                {
                    "schema": 1,
                    "approval": {"result": "passed"},
                    "tdd": {"required": True, "red": {"result": "failed"}, "green": {"result": "passed"}},
                    "verification": [{"result": "passed"}],
                    "reviews": {"standards": {"result": "passed"}, "spec": {"result": "passed"}},
                    "delivery": {"commit": "abc123", "pr": "https://example.test/pr/1"},
                }
            ),
            encoding="utf-8",
        )

    def test_complete_allows_evidenced_task_to_archive(self) -> None:
        self.assertNotEqual(0, self.run_task("validate", "fixture", "--complete").returncode)
        self.make_complete()
        self.assertEqual(0, self.run_task("validate", "fixture", "--complete").returncode)

        result = self.run_task("archive", "fixture", "--no-commit")

        self.assertEqual(0, result.returncode, result.stdout + result.stderr)
        self.assertFalse(self.task_dir.exists())
        self.assertEqual(1, len(list((self.root / ".trellis" / "tasks" / "archive").glob("*/fixture"))))

    def test_complete_rejects_unchecked_source_ticket_item(self) -> None:
        self.make_complete()
        (self.root / "docs" / "plans" / "ticket.md").write_text(
            "# ticket\n\n- [ ] Ship the gate\n", encoding="utf-8"
        )

        result = self.run_task("validate", "fixture", "--complete")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("unchecked", result.stdout + result.stderr)

    def test_gate_rejects_unsupported_evidence_schema(self) -> None:
        self.make_complete()
        evidence_path = self.task_dir / "quality-evidence.json"
        evidence = json.loads(evidence_path.read_text(encoding="utf-8"))
        evidence["schema"] = 2
        evidence_path.write_text(json.dumps(evidence), encoding="utf-8")

        result = self.run_task("validate", "fixture", "--complete")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("schema=1", result.stdout + result.stderr)

    def test_forced_archive_requires_and_records_reason(self) -> None:
        result = self.run_task("archive", "fixture", "--no-commit", "--force")

        self.assertNotEqual(0, result.returncode)
        self.assertTrue(self.task_dir.is_dir())

        result = self.run_task(
            "archive", "fixture", "--no-commit", "--force", "--reason", "emergency recovery"
        )

        self.assertEqual(0, result.returncode, result.stdout + result.stderr)
        archived = next((self.root / ".trellis" / "tasks" / "archive").glob("*/fixture"))
        data = json.loads((archived / "task.json").read_text(encoding="utf-8"))
        self.assertEqual("archive", data["meta"]["gate_overrides"][-1]["action"])
        self.assertEqual("emergency recovery", data["meta"]["gate_overrides"][-1]["reason"])

    def test_inline_mode_allows_ready_without_jsonl(self) -> None:
        (self.root / ".trellis" / "config.yaml").write_text(
            "codex:\n  dispatch_mode: inline\n", encoding="utf-8"
        )
        (self.task_dir / "quality-evidence.json").write_text(
            json.dumps({"schema": 1, "approval": {"result": "passed"}}), encoding="utf-8"
        )

        result = self.run_task("validate", "fixture", "--ready")

        self.assertEqual(0, result.returncode, result.stdout + result.stderr)

    def test_bootstrap_task_is_ready_without_behavioral_evidence(self) -> None:
        data = json.loads((self.task_dir / "task.json").read_text(encoding="utf-8"))
        data["meta"] = {"bootstrap": True, "risk": "behavioral"}
        (self.task_dir / "task.json").write_text(json.dumps(data), encoding="utf-8")

        result = self.run_task("validate", "fixture", "--ready")

        self.assertEqual(0, result.returncode, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
