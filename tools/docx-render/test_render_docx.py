from __future__ import annotations

import copy
import json
import tempfile
import unittest
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET

from render_docx import image_box, render


ROOT = Path(__file__).parent
W = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
NS = {"w": W}


def jpeg_header(width: int, height: int) -> bytes:
    app0 = b"\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"
    sof0 = (
        b"\xff\xc0\x00\x11\x08"
        + height.to_bytes(2, "big")
        + width.to_bytes(2, "big")
        + b"\x03\x01\x11\x00\x02\x11\x00\x03\x11\x00"
    )
    return b"\xff\xd8" + app0 + sof0 + b"\xff\xd9"


class RenderDocxTests(unittest.TestCase):
    def test_runtime_maps_summary_rows_to_formal_template(self) -> None:
        context = json.loads((ROOT / "fixtures/project_report.json").read_text())
        context = copy.deepcopy(context)
        for zone in context["network_zones"]:
            for key in ("confirmed", "not_observed"):
                for verification in zone[key]:
                    verification["evidence"] = []

        with tempfile.TemporaryDirectory() as tmp:
            destination = Path(tmp) / "report.docx"
            render(context, ROOT / "templates/project-report.docx", destination)
            with zipfile.ZipFile(destination) as archive:
                document = ET.fromstring(archive.read("word/document.xml"))

        rows = document.findall(".//w:tbl", NS)[0].findall("w:tr", NS)
        values = [
            ["".join(cell.itertext()) for cell in row.findall("w:tc", NS)]
            for row in rows[1:]
        ]
        self.assertEqual(
            values,
            [
                ["1", "弱口令", "10.10.1.10:22", "严重"],
                ["2", "过期组件", "10.10.3.20:443", "中危"],
                ["3", "不安全默认配置", "172.16.1.30:80", "低危"],
            ],
        )
        text = "".join(document.itertext())
        self.assertIn("\u3000\u3000\u3000\u300010.10.1.10", text)
        remediation_paragraphs = [
            paragraph
            for paragraph in document.findall(".//w:p", NS)
            if "".join(paragraph.itertext()).startswith(("第一条：", "第二条：", "第三条："))
        ]
        self.assertEqual(len(remediation_paragraphs), 3)
        for paragraph in remediation_paragraphs:
            indent = paragraph.find("w:pPr/w:ind", NS)
            self.assertIsNotNone(indent)
            self.assertEqual(indent.get(f"{{{W}}}firstLineChars"), "200")

    def test_runtime_renders_critical_conclusion(self) -> None:
        context = json.loads((ROOT / "fixtures/project_report.json").read_text())
        for zone in context["network_zones"]:
            for key in ("confirmed", "not_observed"):
                for verification in zone[key]:
                    verification["evidence"] = []

        with tempfile.TemporaryDirectory() as tmp:
            destination = Path(tmp) / "report.docx"
            render(context, ROOT / "templates/project-report.docx", destination)
            with zipfile.ZipFile(destination) as archive:
                document = ET.fromstring(archive.read("word/document.xml"))

        text = "".join(document.itertext())
        self.assertIn("其中严重漏洞1个、高危漏洞0个、中危漏洞1个、低危漏洞1个", text)
        self.assertIn("Redis 未授权访问漏洞相关漏洞不存在证明，端口（6379）", text)

    def test_jpeg_images_keep_landscape_and_portrait_aspect_ratios(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            landscape = Path(tmp) / "landscape.jpg"
            portrait = Path(tmp) / "portrait.jpg"
            landscape.write_bytes(jpeg_header(800, 400))
            portrait.write_bytes(jpeg_header(400, 800))

            landscape_box = image_box(landscape)
            portrait_box = image_box(portrait)

        self.assertAlmostEqual(landscape_box[0] / landscape_box[1], 2.0)
        self.assertAlmostEqual(portrait_box[0] / portrait_box[1], 0.5)
        self.assertLessEqual(landscape_box[0], 150)
        self.assertLessEqual(portrait_box[1], 180)


if __name__ == "__main__":
    unittest.main()
