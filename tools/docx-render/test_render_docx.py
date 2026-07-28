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
        self.assertIn("10.10.1.10", text)
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

    def test_runtime_renders_one_zone_with_multi_run_access_context(self) -> None:
        context = json.loads((ROOT / "fixtures/project_report.json").read_text())
        context = copy.deepcopy(context)
        zone = context["network_zones"][0]
        self.assertEqual(zone["access_points_text"].splitlines(), ["调度数据网接入点", "生产控制补充接入点"])
        self.assertEqual(zone["tester_ips_text"].splitlines(), ["10.10.1.250", "10.10.1.251"])
        for report_zone in context["network_zones"]:
            for key in ("confirmed", "not_observed"):
                for verification in report_zone[key]:
                    verification["evidence"] = []

        with tempfile.TemporaryDirectory() as tmp:
            destination = Path(tmp) / "report.docx"
            render(context, ROOT / "templates/project-report.docx", destination)
            with zipfile.ZipFile(destination) as archive:
                document = ET.fromstring(archive.read("word/document.xml"))

        text = "".join(document.itertext())
        for value in ("调度数据网接入点", "生产控制补充接入点", "10.10.1.250", "10.10.1.251", "10.10.1.12", "10.10.2.0/24"):
            self.assertEqual(text.count(value), 1, value)
        self.assertEqual(
            sum("".join(paragraph.itertext()).strip() == "I区" for paragraph in document.findall(".//w:p", NS)),
            1,
        )
        paragraphs_with_value = {
            value: [
                paragraph
                for paragraph in document.findall(".//w:p", NS)
                if value in "".join(paragraph.itertext())
            ]
            for value in ("调度数据网接入点", "生产控制补充接入点", "10.10.1.250", "10.10.1.251", "10.10.1.12", "10.10.2.0/24")
        }
        for value, paragraphs in paragraphs_with_value.items():
            self.assertEqual(len(paragraphs), 1, f"{value} should appear in exactly one paragraph")
            indent = paragraphs[0].find("w:pPr/w:ind", NS)
            self.assertIsNotNone(indent, f"{value} paragraph must have an indent")
            self.assertIsNotNone(indent.get(f"{{{W}}}leftChars"), f"{value} paragraph must use leftChars indentation")
            self.assertGreater(int(indent.get(f"{{{W}}}leftChars")), 0, f"{value} paragraph must be indented")
        for label in ("测试设备接入点：", "测试设备 IP：", "测试范围："):
            label_paragraphs = [p for p in document.findall(".//w:p", NS) if label in "".join(p.itertext())]
            self.assertEqual(len(label_paragraphs), len(context["network_zones"]), f"label {label!r} should appear once per zone")
            for label_paragraph in label_paragraphs:
                indent = label_paragraph.find("w:pPr/w:ind", NS)
                if indent is not None:
                    left_chars = indent.get(f"{{{W}}}leftChars")
                    self.assertTrue(left_chars is None or int(left_chars) == 0, f"label {label!r} must not be indented")

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
