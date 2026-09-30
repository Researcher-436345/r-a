#!/usr/bin/env python3
"""Generates the fixture PDFs in ./pdf without external libraries.

Each page carries a known text line, so a test can assert that page N of the
parsed document contains marker N. Re-run after changing the page texts.
"""
from pathlib import Path


def build_pdf(pages: list[list[str]]) -> bytes:
    objects: list[bytes] = []

    def add(obj: bytes) -> int:
        objects.append(obj)
        return len(objects)

    font = add(b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
    page_ids: list[int] = []
    pages_obj_id = len(objects) + 1 + 2 * len(pages)  # allocated after pages+streams
    for lines in pages:
        content_lines = ["BT", "/F1 14 Tf", "72 720 Td", "16 TL"]
        for line in lines:
            esc = line.replace("\\", "\\\\").replace("(", "\\(").replace(")", "\\)")
            content_lines.append(f"({esc}) Tj T*")
        content_lines.append("ET")
        stream = "\n".join(content_lines).encode("latin-1")
        content_id = add(b"<< /Length %d >>\nstream\n" % len(stream) + stream + b"\nendstream")
        page_id = add(
            (
                f"<< /Type /Page /Parent {pages_obj_id} 0 R /MediaBox [0 0 612 792] "
                f"/Resources << /Font << /F1 {font} 0 R >> >> /Contents {content_id} 0 R >>"
            ).encode()
        )
        page_ids.append(page_id)
    kids = " ".join(f"{pid} 0 R" for pid in page_ids)
    real_pages_id = add(f"<< /Type /Pages /Kids [{kids}] /Count {len(page_ids)} >>".encode())
    assert real_pages_id == pages_obj_id
    catalog = add(f"<< /Type /Catalog /Pages {real_pages_id} 0 R >>".encode())

    out = bytearray(b"%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
    offsets = []
    for i, obj in enumerate(objects, start=1):
        offsets.append(len(out))
        out += f"{i} 0 obj\n".encode() + obj + b"\nendobj\n"
    xref = len(out)
    out += f"xref\n0 {len(objects) + 1}\n".encode()
    out += b"0000000000 65535 f \n"
    for off in offsets:
        out += f"{off:010d} 00000 n \n".encode()
    out += f"trailer\n<< /Size {len(objects) + 1} /Root {catalog} 0 R >>\nstartxref\n{xref}\n%%EOF\n".encode()
    return bytes(out)


def main() -> None:
    out = Path(__file__).parent / "pdf"
    out.mkdir(exist_ok=True)

    five = []
    for n in range(1, 6):
        lines = [
            f"Fixture paper: five pages. This is page {n}.",
            f"MARKER-PAGE-{n}",
            "The quick brown fox jumps over the lazy dog. " * 3,
        ]
        if n == 3:
            lines.append("HYPOTHESIS-ON-PAGE-THREE: deterministic fixtures make tests honest.")
        five.append(lines)
    (out / "paper-5p.pdf").write_bytes(build_pdf(five))

    one = [[
        "Fixture paper: one page.",
        "MARKER-PAGE-1",
        "A single page is enough to check upload, storage and parsing.",
    ]]
    (out / "paper-1p.pdf").write_bytes(build_pdf(one))

    oa = [[
        "Open-access copy served by a publisher, found through OpenAlex.",
        "OA-MARKER-PAGE-1",
        "Its bytes differ from every other fixture, so SHA-256 dedup keeps it apart.",
    ]]
    (out / "paper-oa-1p.pdf").write_bytes(build_pdf(oa))

    private = [[
        "Private upload: this file exists only in one user's library.",
        "PRIVATE-MARKER-PAGE-1",
        "Its bytes differ from every arXiv fixture, so SHA-256 dedup never links it to a public paper.",
    ]]
    (out / "private-1p.pdf").write_bytes(build_pdf(private))

    (out / "not-a-pdf.txt").write_text("This file only pretends to be a PDF.\n")
    (out / "not-a-pdf.pdf").write_text("<html><body>Publisher bot wall</body></html>\n")


if __name__ == "__main__":
    main()
