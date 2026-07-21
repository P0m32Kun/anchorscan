"""Render the project DOCX report from a JSON context.

Usage:
    uv run --project tools/docx-render python render_docx.py \
        --template tools/docx-render/templates/project-report.docx \
        --context context.json \
        --out project-report.docx

The context shape mirrors fixtures/project_report.json, except each evidence
entry carries an absolute ``path`` to a screenshot on disk; this script converts
it into an InlineImage sized inside the 150x180mm box while preserving aspect
ratio. The template is the version-managed artifact produced by prototype.py.
"""

from __future__ import annotations

import argparse
import copy
import json
import struct
import unicodedata
import zlib
from pathlib import Path

from docxtpl import DocxTemplate, InlineImage
from docx.shared import Mm
from jinja2 import Environment


def cover_line(value: object, width: float = 52) -> str:
    text = str(value)
    text_width = sum(
        3.5 if unicodedata.east_asian_width(char) in "WFA" else 3 for char in text
    )
    padding = max(0, round(width - text_width))
    left = padding // 2
    return "\u00a0" * left + text + "\u00a0" * (padding - left)


def image_box(path: Path, max_width_mm: float = 150, max_height_mm: float = 180) -> tuple[float, float]:
    data = path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        # JPEG or unknown: fall back to the max box; docxtpl still embeds it.
        return max_width_mm, max_height_mm
    width, height = struct.unpack(">II", data[16:24])
    scale = min(max_width_mm / width, max_height_mm / height)
    return width * scale, height * scale


def render(context: dict, template_path: Path, destination: Path) -> None:
    template = DocxTemplate(template_path)
    context = copy.deepcopy(context)
    context["summary_empty"] = not context.get("summary_rows")

    def attach(zone: dict, key: str) -> None:
        for verification in zone.get(key, []):
            for evidence in verification.get("evidence", []):
                path = Path(evidence["path"])
                width, height = image_box(path)
                evidence["image"] = InlineImage(
                    template, str(path), width=Mm(width), height=Mm(height)
                )

    for zone in context.get("network_zones", []):
        attach(zone, "confirmed")
        attach(zone, "not_observed")

    environment = Environment(autoescape=True)
    environment.filters["cover_line"] = cover_line
    template.render(context, jinja_env=environment, autoescape=True)
    template.save(destination)


def main() -> None:
    parser = argparse.ArgumentParser(description="Render the project DOCX report.")
    parser.add_argument("--template", required=True, type=Path)
    parser.add_argument("--context", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args()

    context = json.loads(args.context.read_text(encoding="utf-8"))
    render(context, args.template, args.out)
    print(str(args.out))


if __name__ == "__main__":
    main()
