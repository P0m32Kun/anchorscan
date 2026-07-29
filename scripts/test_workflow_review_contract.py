#!/usr/bin/env python3
"""Static contract tests for the project-local AI review workflow."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class WorkflowReviewContractTest(unittest.TestCase):
    def test_workflow_orders_tdd_independent_reviews_and_pr(self) -> None:
        text = (ROOT / ".trellis/workflow.md").read_text(encoding="utf-8")
        self.assertIn(
            "TDD Red -> Green -> self-check -> Standards review -> Spec/AC review -> full verification -> PR",
            text,
        )
        self.assertIn("trellis-check is a write-capable self-check", text)
        self.assertIn("code-review", text)

    def test_every_implement_dispatch_requires_tdd(self) -> None:
        text = (ROOT / ".trellis/workflow.md").read_text(encoding="utf-8")
        for platform_block in (
            "Claude Code, Cursor, OpenCode, codex-sub-agent, CodeBuddy, Droid, Pi, ZCode, Snow, Oh My Pi",
            "Gemini, Qoder, Copilot, Reasonix, Trae, Grok, Kimi Code",
            "Kiro",
        ):
            start = text.index(f"[{platform_block}]")
            end = text.index(f"[/{platform_block}]", start)
            self.assertIn("TDD Red", text[start:end], platform_block)

    def test_all_agent_surfaces_preserve_role_boundaries(self) -> None:
        for relative in (
            ".codex/agents/trellis-implement.toml",
            ".pi/agents/trellis-implement.md",
            ".trellis/agents/implement.md",
        ):
            self.assertIn("TDD Red", (ROOT / relative).read_text(encoding="utf-8"), relative)
        for relative in (
            ".codex/agents/trellis-check.toml",
            ".pi/agents/trellis-check.md",
            ".trellis/agents/check.md",
        ):
            text = (ROOT / relative).read_text(encoding="utf-8")
            self.assertIn("write-capable self-check", text, relative)
            self.assertIn("must not claim independent review", text, relative)

    def test_pi_continue_routes_to_review_before_delivery(self) -> None:
        for relative in (
            ".pi/prompts/trellis-continue.md",
            ".agents/skills/trellis-continue/SKILL.md",
        ):
            text = (ROOT / relative).read_text(encoding="utf-8")
            self.assertIn("Standards review", text, relative)
            self.assertIn("Spec/AC review", text, relative)


if __name__ == "__main__":
    unittest.main()
