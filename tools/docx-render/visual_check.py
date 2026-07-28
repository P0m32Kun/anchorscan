"""Render the multi-run DOCX fixture into per-page PNGs for layout review.

This deliberately uses LibreOffice and Poppler, the same headless tools used in
CI. Rasterization catches DOCX conversion failures and leaves every page as an
artifact that reviewers can inspect for clipping, overlap, broken tables, and
unexpected pagination.
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import tempfile
from pathlib import Path


ROOT = Path(__file__).parent


def command(name: str) -> str:
    path = shutil.which(name)
    if not path:
        raise SystemExit(
            f"missing required renderer {name!r}; install LibreOffice and Poppler "
            "(for example: apt-get install libreoffice poppler-utils)"
        )
    return path


def png_size(path: Path) -> tuple[int, int]:
    data = path.read_bytes()[:24]
    if data[:8] != b"\x89PNG\r\n\x1a\n" or data[12:16] != b"IHDR":
        raise ValueError(f"{path} is not a PNG")
    return int.from_bytes(data[16:20], "big"), int.from_bytes(data[20:24], "big")


def main() -> None:
    parser = argparse.ArgumentParser(description="Render the multi-run DOCX fixture to reviewable PNG pages.")
    parser.add_argument("--out", type=Path, default=ROOT / "artifacts" / "multi-run-pages")
    args = parser.parse_args()

    soffice = command("soffice")
    pdftoppm = command("pdftoppm")
    output = args.out.resolve()
    output.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory() as temporary:
        work = Path(temporary)
        docx = work / "multi-run-report.docx"
        context = json.loads((ROOT / "fixtures" / "project_report.json").read_text(encoding="utf-8"))
        for zone in context["network_zones"]:
            for outcome in ("confirmed", "not_observed"):
                for verification in zone[outcome]:
                    verification["evidence"] = []
        context_path = work / "multi-run-context.json"
        context_path.write_text(json.dumps(context), encoding="utf-8")
        subprocess.run(
            [
                "uv", "run", "--project", str(ROOT), "python", str(ROOT / "render_docx.py"),
                "--template", str(ROOT / "templates" / "project-report.docx"),
                "--context", str(context_path),
                "--out", str(docx),
            ],
            check=True,
        )
        subprocess.run(
            [soffice, "--headless", "--convert-to", "pdf", "--outdir", str(work), str(docx)],
            check=True,
        )
        pdf = docx.with_suffix(".pdf")
        if not pdf.is_file() or pdf.stat().st_size == 0:
            raise SystemExit("LibreOffice did not produce a non-empty PDF")
        prefix = output / "multi-run"
        for page in output.glob("multi-run-*.png"):
            page.unlink()
        subprocess.run([pdftoppm, "-png", "-r", "144", str(pdf), str(prefix)], check=True)

    pages = sorted(output.glob("multi-run-*.png"))
    if not pages:
        raise SystemExit("PDF rasterization produced no pages")
    for page in pages:
        width, height = png_size(page)
        if width < 500 or height < 500 or page.stat().st_size < 1024:
            raise SystemExit(f"invalid rendered page {page}: {width}x{height}, {page.stat().st_size} bytes")
    print(f"Rendered {len(pages)} full-page PNGs to {output}")


if __name__ == "__main__":
    main()
