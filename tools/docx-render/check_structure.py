"""Minimal OOXML regression gate for the visible-slot prototype."""

from __future__ import annotations

import sys
import zipfile
from hashlib import sha256
from pathlib import Path
from xml.etree import ElementTree as ET


W = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
WP = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
A = "http://schemas.openxmlformats.org/drawingml/2006/main"
R = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
REL = "http://schemas.openxmlformats.org/package/2006/relationships"
V = "urn:schemas-microsoft-com:vml"
WPS = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
NS = {"w": W, "wp": WP, "a": A, "r": R, "v": V, "wps": WPS}


def qn(namespace: str, tag: str) -> str:
    return f"{{{namespace}}}{tag}"


def read_xml(docx: Path, name: str) -> ET.Element:
    with zipfile.ZipFile(docx) as archive:
        return ET.fromstring(archive.read(name))


def part_names(docx: Path, prefix: str) -> list[str]:
    with zipfile.ZipFile(docx) as archive:
        return sorted(name for name in archive.namelist() if name.startswith(prefix))


def body_image_parts(docx: Path) -> set[str]:
    document = read_xml(docx, "word/document.xml")
    image_ids = {
        node.get(qn(R, attribute))
        for path, attribute in ((".//a:blip", "embed"), (".//v:imagedata", "id"))
        for node in document.findall(path, NS)
    }
    relationships = read_xml(docx, "word/_rels/document.xml.rels")
    return {
        f"word/{node.get('Target')}"
        for node in relationships.findall(qn(REL, "Relationship"))
        if node.get("Id") in image_ids
    }


def field_signature(docx: Path) -> dict[str, list[tuple[str, str]]]:
    signature = {}
    for name in part_names(docx, "word/"):
        if not (name == "word/document.xml" or "/header" in name or "/footer" in name):
            continue
        root = read_xml(docx, name)
        events = []
        for node in root.iter():
            if node.tag == qn(W, "fldChar"):
                events.append(("fldChar", node.get(qn(W, "fldCharType"), "")))
            elif node.tag == qn(W, "instrText"):
                events.append(("instrText", "".join(node.itertext())))
            elif node.tag == qn(W, "fldSimple"):
                events.append(("fldSimple", node.get(qn(W, "instr"), "")))
        signature[name] = events
    return signature


# TOC_CACHE_INSTRS are the nested field instructions that only exist inside a
# TOC field's cached result. The TOC itself is rebuilt dynamically (its cached
# zone entries are stripped), so these are excluded from the stable signature.
TOC_CACHE_INSTRS = ("TOC", "HYPERLINK", "PAGEREF")


def stable_field_signature(docx: Path) -> dict[str, list[str]]:
    """Per-part instrText signature with the dynamic TOC cache excluded.

    Stable fields (PAGE / STYLEREF / REF / etc.) must stay identical between the
    source and the generated template; only the TOC and its cached hyperlinks /
    pagerefs are allowed to change because the table of contents is rebuilt from
    the live Heading paragraphs.
    """
    signature: dict[str, list[str]] = {}
    for name in part_names(docx, "word/"):
        if not (name == "word/document.xml" or "/header" in name or "/footer" in name):
            continue
        root = read_xml(docx, name)
        instrs = [
            "".join(node.itertext()).strip()
            for node in root.iter(qn(W, "instrText"))
        ]
        signature[name] = sorted(
            i for i in instrs if not any(tag in i for tag in TOC_CACHE_INSTRS)
        )
    return signature


def toc_instruction_count(docx: Path) -> int:
    root = read_xml(docx, "word/document.xml")
    return sum(
        1
        for node in root.iter(qn(W, "instrText"))
        if "TOC" in "".join(node.itertext())
    )


def body_texts(docx: Path) -> list[str]:
    body = read_xml(docx, "word/document.xml").find("w:body", NS)
    return ["".join(node.itertext()).strip() for node in body]


def chapter_two_signature(docx: Path) -> list[str]:
    texts = body_texts(docx)
    start = texts.index("测试过程及方法")
    end = texts.index("测试结果与分析")
    return texts[start:end]


def find_paragraph(docx: Path, needle: str) -> ET.Element:
    root = read_xml(docx, "word/document.xml")
    matches = [p for p in root.findall(".//w:p", NS) if needle in "".join(p.itertext())]
    assert len(matches) == 1, f"expected one paragraph containing {needle!r}, got {len(matches)}"
    return matches[0]


def paragraph_style(docx: Path, needle: str) -> str:
    paragraph = find_paragraph(docx, needle)
    style = paragraph.find("w:pPr/w:pStyle", NS)
    return "" if style is None else style.get(qn(W, "val"), "")


def slot_is_underlined(docx: Path, slot: str) -> bool:
    root = read_xml(docx, "word/document.xml")
    for paragraph in root.findall(".//w:p", NS):
        for run in paragraph.findall("w:r", NS):
            if slot not in "".join(run.itertext()):
                continue
            underline = run.find("w:rPr/w:u", NS)
            if underline is not None and underline.get(qn(W, "val"), "single") != "none":
                return True
    return False


def find_underlined_slot_paragraph(docx: Path, slot: str) -> ET.Element:
    root = read_xml(docx, "word/document.xml")
    matches = []
    for paragraph in root.findall(".//w:p", NS):
        for run in paragraph.findall("w:r", NS):
            if slot not in "".join(run.itertext()):
                continue
            underline = run.find("w:rPr/w:u", NS)
            if underline is not None and underline.get(qn(W, "val"), "single") != "none":
                matches.append(paragraph)
    assert len(matches) == 1, f"expected one underlined slot paragraph for {slot!r}"
    return matches[0]


def underlined_run_text(docx: Path, needle: str) -> str:
    paragraph = find_underlined_slot_paragraph(docx, needle)
    matches = []
    for run in paragraph.findall("w:r", NS):
        underline = run.find("w:rPr/w:u", NS)
        if underline is not None and underline.get(qn(W, "val"), "single") != "none":
            matches.append("".join(run.itertext()))
    assert len(matches) == 1, f"expected one underlined run containing {needle!r}"
    return matches[0]


def cover_line_width(value: str) -> float:
    import unicodedata

    return value.count("\u00a0") + sum(
        3.5 if unicodedata.east_asian_width(char) in "WFA" else 3
        for char in value.replace("\u00a0", "")
    )


def first_underlined_run_properties(docx: Path, paragraph_needle: str) -> bytes:
    paragraph = find_paragraph(docx, paragraph_needle)
    for run in paragraph.findall("w:r", NS):
        underline = run.find("w:rPr/w:u", NS)
        if underline is not None and underline.get(qn(W, "val"), "single") != "none":
            return ET.tostring(run.find("w:rPr", NS))
    raise AssertionError(f"missing underlined run in paragraph containing {paragraph_needle!r}")


def verify_template_slots(source: Path, template: Path) -> None:
    document = read_xml(template, "word/document.xml")
    body = document.find("w:body", NS)
    children = list(body)
    texts = ["".join(node.itertext()).strip() for node in children]
    template_text = "".join(document.itertext())
    required = {
        "{{ report.title }}",
        "{{ report.test_subject }}",
        "{{ report.test_subject|cover_line }}",
        "{{ report.project_created_date|cover_line }}",
        "{{ report.testers_text|cover_line }}",
        "{{ report.project_created_month }}",
        "{{ report.client_name }}",
        "{{ report.test_period }}",
        "{%tr for r in summary_rows %}",
        "{{ r.no }}",
        "{{ r.title }}",
        "{{ r.assets }}",
        "{{ r.level }}",
        "{%tr if summary_empty %}",
        "{%p for network_zone in network_zones %}",
        "{{ network_zone.name }}",
        "{%p for session in network_zone.sessions %}",
        "{{ session.label }}",
        "{{ session.access_point }}",
        "{{ session.tester_ip }}",
        "{{ session.targets_text }}",
        "{{ session.exclusions_text }}",
        "{{ session.notes }}",
        "{%p for verification in network_zone.confirmed %}",
        "{{ verification.heading }}",
        "{{ verification.description }}",
        "{{ verification.assets_text }}",
        "{{ verification.remediation }}",
        "{%p for verification in network_zone.not_observed %}",
        "{{ verification.title }}",
        "{{ verification.ports_text }}",
        "{%p for evidence in verification.evidence %}",
        "{{ evidence.image }}",
        "{{ report.client_name }}",
        "{{ conclusion.network_zone_names_text }}",
        "{{ conclusion.total }}",
        "{{ conclusion.critical }}",
        "{{ conclusion.high }}",
        "{{ conclusion.medium }}",
        "{{ conclusion.low }}",
        "{{ conclusion.focus_text }}",
    }
    missing = sorted(token for token in required if token not in template_text)
    assert not missing, f"missing visible slots: {missing}"
    assert not body_image_parts(template), "template contains example body images"
    assert chapter_two_signature(template) == chapter_two_signature(source), "chapter 2 changed"
    assert "南京南瑞信息通信科技有限公司" in template_text
    assert "{{ verification.detail }}" not in template_text
    assert "{{ evidence.caption }}" not in template_text
    assert "证据截图" not in template_text
    assert "{{ conclusion.text }}" not in template_text
    cover_slots = (
        "{{ report.test_subject|cover_line }}",
        "{{ report.project_created_date|cover_line }}",
        "{{ report.testers_text|cover_line }}",
    )
    cover_run_properties = []
    for slot in cover_slots:
        assert slot_is_underlined(template, slot), f"cover slot lost underline: {slot}"
        cover_run_properties.append(paragraph_run_properties(template, slot))
        paragraph = find_underlined_slot_paragraph(template, slot)
        assert not paragraph.findall("w:pPr/w:tabs/w:tab", NS), (
            "cover lines must not mix tab leaders with text underlines"
        )
        runs = paragraph.findall("w:r", NS)
        slot_index = next(i for i, run in enumerate(runs) if slot in "".join(run.itertext()))
        assert len(runs[slot_index].findall("w:tab", NS)) == 0
        underline = runs[slot_index].find("w:rPr/w:u", NS)
        assert underline is not None and underline.get(qn(W, "val")) == "single", (
            "cover spaces and value must share one default underline"
        )
    assert len(set(cover_run_properties)) == 1, "all three cover lines must use one format"
    month = find_paragraph(template, "{{ report.project_created_month }}")
    indent = month.find("w:pPr/w:ind", NS)
    assert indent is not None
    assert {
        key: indent.get(qn(W, key))
        for key in ("right", "rightChars", "firstLine", "firstLineChars")
    } == {
        "right": "-50",
        "rightChars": "-21",
        "firstLine": "3048",
        "firstLineChars": "1016",
    }
    month_fonts = month.find("w:pPr/w:rPr/w:rFonts", NS)
    assert month_fonts is not None
    assert month_fonts.get(qn(W, "hint")) == "eastAsia"
    assert month_fonts.get(qn(W, "eastAsia")) == "黑体"
    assert month.find("w:pPr/w:jc", NS) is None, "saved month format is not simple centering"
    assert paragraph_style(template, "{{ verification.heading }}")
    negative_heading = find_paragraph(
        template, "{{ network_zone.name }}其它漏洞验证不存在的截图："
    )
    assert paragraph_style(
        template, "{{ network_zone.name }}其它漏洞验证不存在的截图："
    ) == paragraph_style(
        template, "{{ verification.heading }}"
    ), "not-observed section heading must match vulnerability-heading level"
    negative_num_id = negative_heading.find("w:pPr/w:numPr/w:numId", NS)
    assert negative_num_id is not None and negative_num_id.get(qn(W, "val")) == "0", (
        "not-observed H3 must not display or consume a 3.x.y vulnerability number"
    )

    conclusion_text = "".join(find_paragraph(template, "{{ conclusion.focus_text }}").itertext())
    assert conclusion_text == (
        "本次测试共测试{{ report.client_name }}{{ conclusion.network_zone_names_text }}所有设备，"
        "共发现漏洞{{ conclusion.total }}个，其中严重漏洞{{ conclusion.critical }}个、高危漏洞{{ conclusion.high }}个、中危漏洞"
        "{{ conclusion.medium }}个、低危漏洞{{ conclusion.low }}个。其中问题主要集中在"
        "{{ conclusion.focus_text }}这几个方面。"
    )
    assert "需要及时整改，加强安全管理规范，并进行复测，预防安全事故发生。" in template_text

    zone_start = texts.index("{%p for network_zone in network_zones %}")
    conclusion = texts.index("渗透测试结论")
    zone_end = max(i for i in range(zone_start, conclusion) if texts[i] == "{%p endfor %}")
    confirmed_start = texts.index("{%p for verification in network_zone.confirmed %}")
    not_observed_start = texts.index("{%p if network_zone.not_observed %}")
    not_observed_heading = texts.index(
        "{{ network_zone.name }}其它漏洞验证不存在的截图："
    )
    assert (
        confirmed_start < not_observed_start < not_observed_heading < zone_end < conclusion
    ), "not-observed H3 must be the final section inside each zone"
    assert zone_start < zone_end < conclusion
    assert not any(node.tag == qn(W, "tbl") for node in children[zone_start : zone_end + 1])
    assert all("Zone" not in text for text in texts[zone_start : zone_end + 1])
    assert texts[-1] == "", "business content appears after the final section"


def paragraph_run_properties(docx: Path, needle: str) -> bytes:
    root = read_xml(docx, "word/document.xml")
    for paragraph in root.findall(".//w:p", NS):
        if needle not in "".join(paragraph.itertext()):
            continue
        for run in paragraph.findall("w:r", NS):
            if needle in "".join(run.itertext()):
                props = run.find("w:rPr", NS)
                return ET.tostring(props) if props is not None else b""
    raise AssertionError(f"missing text {needle!r}")


def png_colour(archive: zipfile.ZipFile, name: str) -> tuple[int, int, int]:
    data = archive.read(name)
    offset = 8
    payload = b""
    while offset < len(data):
        size = int.from_bytes(data[offset : offset + 4], "big")
        kind = data[offset + 4 : offset + 8]
        content = data[offset + 8 : offset + 8 + size]
        offset += 12 + size
        if kind == b"IDAT":
            payload += content
        if kind == b"IEND":
            raw = __import__("zlib").decompress(payload)
            return tuple(raw[1:4])
    raise AssertionError(f"invalid PNG {name}")


def verify_images(template: Path, docx: Path) -> None:
    expected_colours = [
        (228, 74, 48),
        (48, 125, 228),
        (61, 159, 100),
        (61, 159, 100),
        (228, 74, 48),
        (48, 125, 228),
        (61, 159, 100),
    ]
    expected_ratios = [2.0, 0.5, 2.0, 2.0, 2.0, 0.5, 2.0]
    document = read_xml(docx, "word/document.xml")
    rels = read_xml(docx, "word/_rels/document.xml.rels")
    targets = {node.get("Id"): node.get("Target") for node in rels.findall(qn(REL, "Relationship"))}
    drawings = []
    for inline in document.findall(".//wp:inline", NS):
        embed = inline.find(".//a:blip", NS).get(qn(R, "embed"))
        target = targets[embed]
        extent = inline.find("wp:extent", NS)
        drawings.append(("word/" + target, int(extent.get("cx")), int(extent.get("cy"))))
    with zipfile.ZipFile(docx) as archive:
        assert len(drawings) == 7, f"expected 7 evidence images, got {len(drawings)}"
        assert [png_colour(archive, name) for name, _, _ in drawings] == expected_colours
    for (_, width, height), ratio in zip(drawings, expected_ratios, strict=True):
        assert abs(width / height - ratio) < 0.001, (width, height, ratio)


def verify_package(docx: Path, base: dict) -> None:
    with zipfile.ZipFile(docx) as archive:
        assert archive.testzip() is None
    document = read_xml(docx, "word/document.xml")
    assert len(document.findall(".//w:sectPr", NS)) == base["sections"] == 3
    assert part_names(docx, "word/header") == base["headers"]
    assert part_names(docx, "word/footer") == base["footers"]
    assert stable_field_signature(docx) == base["stable_fields"], "stable fields (PAGE/STYLEREF/REF) changed"
    assert toc_instruction_count(docx) == 1, "document must keep exactly one TOC field"
    footer6 = read_xml(docx, "word/footer6.xml")
    assert len(footer6.findall(".//wp:anchor", NS)) == base["footer6_anchor"]
    assert len(footer6.findall(".//wps:wsp", NS)) == base["footer6_shape"]


def verify(source: Path, template: Path, out: Path) -> None:
    assert sha256(source.read_bytes()).hexdigest() == (
        "e5f6a89663065737db00c1aa94ab97b86da308c8a75571db5a4e19c238a2c6e5"
    )
    verify_template_slots(source, template)
    source_doc = read_xml(source, "word/document.xml")
    base = {
        "sections": len(source_doc.findall(".//w:sectPr", NS)),
        "headers": part_names(source, "word/header"),
        "footers": part_names(source, "word/footer"),
        "fields": field_signature(source),
        "stable_fields": stable_field_signature(source),
        "footer6_anchor": len(read_xml(source, "word/footer6.xml").findall(".//wp:anchor", NS)),
        "footer6_shape": len(read_xml(source, "word/footer6.xml").findall(".//wps:wsp", NS)),
    }
    verify_package(template, base)

    for count in (0, 1, 3):
        docx = out / f"table-{count}.docx"
        verify_package(docx, base)
        document = read_xml(docx, "word/document.xml")
        rows = document.findall(".//w:tbl", NS)[0].findall("w:tr", NS)
        assert len(rows) == (2 if count == 0 else count + 1)
        actual = [["".join(cell.itertext()) for cell in row.findall("w:tc", NS)] for row in rows[1:]]
        if count == 0:
            assert actual == [["本次纳入报告范围内无已确认漏洞"]]
        else:
            assert actual == [
                ["1", "弱口令", "10.10.1.10:22", "严重"],
                ["2", "过期组件", "10.10.3.20:443", "中危"],
                ["3", "不安全默认配置", "172.16.1.30:80", "低危"],
            ][:count]
        rendered_text = "".join(document.itertext())
        assert "{{" not in rendered_text and "{%" not in rendered_text
        texts = body_texts(docx)
        conclusion = texts.index("渗透测试结论")
        results = texts.index("测试结果与分析")
        report_body = texts[results:conclusion]
        assert all(name in report_body for name in ("I区", "III区", "互联网接入区"))
        assert "II区" not in report_body
        assert max(texts.index(name) for name in ("I区", "III区", "互联网接入区")) < conclusion
        assert all("接入点：" not in text for text in texts[conclusion + 1 :])
        finding_style = paragraph_style(template, "{{ verification.heading }}")
        numbered_findings = []
        for paragraph in document.findall(".//w:body/w:p", NS):
            style = paragraph.find("w:pPr/w:pStyle", NS)
            num_id = paragraph.find("w:pPr/w:numPr/w:numId", NS)
            if (
                style is not None
                and style.get(qn(W, "val")) == finding_style
                and (num_id is None or num_id.get(qn(W, "val")) != "0")
            ):
                numbered_findings.append("".join(paragraph.itertext()).strip())
        assert numbered_findings == [
            "弱口令（严重）",
            "过期组件（中危）",
            "不安全默认配置（低危）",
        ], "unexpected Heading 3 sequence for the current fixture"

    fake_rendered = out / "project-report-fake.docx"
    rendered = fake_rendered if fake_rendered.exists() else out / "project-report.docx"
    assert paragraph_run_properties(template, "{{ report.title }}") == paragraph_run_properties(
        rendered, "示例电力有限公司安全渗透测试分析报告"
    )
    rendered_cover_lines = []
    for value in (
        "示例电力有限公司生产控制系统",
        "2026年7月1日",
        "张三、李四",
    ):
        assert slot_is_underlined(rendered, value), f"rendered cover value lost underline: {value}"
        line = underlined_run_text(rendered, value)
        assert line.startswith("\u00a0") and line.endswith("\u00a0"), (
            f"rendered cover line lacks underlined space padding: {value}"
        )
        rendered_cover_lines.append(line)
    assert all(
        51.5 <= cover_line_width(line) <= 52.5 for line in rendered_cover_lines
    ), (
        "rendered cover lines must have the same display width"
    )
    month = find_paragraph(rendered, "二零二六年七月")
    template_month = find_paragraph(template, "{{ report.project_created_month }}")
    assert ET.tostring(month.find("w:pPr", NS)) == ET.tostring(
        template_month.find("w:pPr", NS)
    ), "rendered cover month paragraph formatting changed"
    settings = read_xml(rendered, "word/settings.xml")
    assert settings.find("w:updateFields", NS).get(qn(W, "val")) == "true"
    verify_images(template, rendered)


if __name__ == "__main__":
    verify(Path(sys.argv[1]), Path(sys.argv[2]), Path(sys.argv[3]))
    print("structure check passed")
