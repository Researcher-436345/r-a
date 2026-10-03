"""Миниатюра и консервативное извлечение аффилиаций с первой страницы PDF."""

import base64
import re

import fitz


INSTITUTION = re.compile(
    r"\b(?:university|universit[äéà]|institute\s+(?:of|for)|"
    r"[\w-]+\s+institute|école|college|polytechnic|университет\w*|институт\w*)\b",
    re.IGNORECASE,
)
SECTION = re.compile(r"^\s*(?:abstract|summary|[1I][.\s]+introduction)\b", re.IGNORECASE)
CAPTION = re.compile(r"^\s*(?:figure|fig\.|table)\s*\d+\b", re.IGNORECASE)
DEPARTMENT = re.compile(r"^(?:department|faculty|division|laboratory|lab|research group)\b", re.IGNORECASE)
ABBREVIATION = re.compile(
    r"^(?:UC (?:Berkeley|San Diego|Los Angeles|Davis|Irvine|Santa Barbara|Santa Cruz)|"
    r"MIT|UCLA|USC|EPFL|ETH (?:Zurich|Zürich)|Caltech|KAIST|HKUST|Skoltech)$",
    re.IGNORECASE,
)
COMPANY = re.compile(
    r"^(?:Physical Intelligence|Google(?: DeepMind| Research)?|DeepMind|"
    r"Microsoft(?: Research)?|Meta(?: AI| FAIR)?|FAIR|OpenAI|Anthropic|"
    r"NVIDIA|Amazon(?: AWS| Research)?|Apple|IBM(?: Research)?|"
    r"ByteDance|TikTok|Tencent|Alibaba|Baidu|Huawei|Samsung(?: Research)?|"
    r"Adobe(?: Research)?|Salesforce(?: Research)?|Mistral AI|Cohere|DeepSeek)$",
    re.IGNORECASE,
)
AFFILIATION_MARKER = re.compile(r"(?:^|[\s,;])(?:[1-9]\d?|[¹²³⁴⁵⁶⁷⁸⁹])[.)]?\s*(?=[A-Z])")


def is_named_organization(text: str) -> bool:
    # Неизвестную компанию принимаем только в явно размеченной строке аффилиаций.
    words = text.split()
    return (
        1 <= len(words) <= 10
        and all(word[0].isupper() or word in {"of", "the", "and", "for", "&"} for word in words)
        and not re.search(r"[=@{}\\]|\b(?:Figure|Table|Abstract|Introduction)\b", text)
    )


def is_institution(text: str) -> bool:
    return bool(INSTITUTION.search(text) or ABBREVIATION.fullmatch(text) or COMPANY.fullmatch(text))


def affiliation_parts(text: str) -> list[list[str]]:
    parts = re.split(r"[;|]|\s*[¹²³⁴⁵⁶⁷⁸⁹]+\s*|(?<!\w)[1-9][.)]?\s*(?=[A-Z])", text)
    result = []
    for part in parts:
        part = re.sub(r"^[\s\d*†‡,]+|[\s\d*†‡,]+$", "", part)
        if "@" in part or "http" in part.lower():
            continue
        result.append([re.sub(r"\s+", " ", segment).strip() for segment in part.split(",")])
    return result


def extract_affiliations(page: fitz.Page) -> list[str]:
    # Только шапка до аннотации: упоминания вузов в основном тексте не аффилиации.
    lines = []
    for block in page.get_text("dict", sort=True)["blocks"]:
        for line in block.get("lines", []):
            # Вертикальная надпись arXiv не относится к шапке и не задаёт размер заголовка.
            if line.get("dir", (1, 0))[0] < 0.9:
                continue
            spans = line.get("spans", [])
            text = "".join(span["text"] for span in spans).strip()
            if text:
                lines.append((line["bbox"][1], line["bbox"][0], text, max(s["size"] for s in spans)))
    lines.sort(key=lambda line: (line[0], line[1]))
    header = []
    for y, x, text, size in lines:
        if SECTION.match(text) or CAPTION.match(text):
            break
        if y < page.rect.height * 0.48:
            header.append((y, x, text, size))
    if not header:
        return []
    title_size = max(size for _, _, _, size in header)
    rows = []
    for line in header:
        if not rows or abs(line[0] - rows[-1][0][0]) > 1.5:
            rows.append([])
        rows[-1].append(line)
    result = []
    seen = set()
    for row in rows:
        candidates = [
            (text, affiliation_parts(text)) for _, _, text, size in row
            if not (size >= title_size and title_size > 12)
        ]
        organization_row = any(
            is_institution(segment)
            for _, parts in candidates for segments in parts for segment in segments
        )
        for text, parts in candidates:
            numbered = bool(AFFILIATION_MARKER.search(text))
            for segments in parts:
                # Кафедру и почтовый адрес отделяем от названия организации.
                known_part = any(is_institution(segment) for segment in segments)
                for index, segment in enumerate(segments):
                    # PDF часто разбивает один ряд организаций на отдельные колонки:
                    # UC Berkeley | Amazon FAR или MBZUAI | MWS AI.
                    contextual = (
                        organization_row and not known_part and index == 0
                        and (numbered or (len(candidates) > 1 and len(segment.split()) >= 2))
                        and not DEPARTMENT.match(segment)
                        and is_named_organization(segment)
                    )
                    if not is_institution(segment) and not contextual:
                        continue
                    key = segment.casefold()
                    if 2 <= len(segment) <= 120 and key not in seen:
                        seen.add(key)
                        result.append(segment)
    return result[:5]


def preview_pdf(raw: bytes) -> dict:
    with fitz.open(stream=raw, filetype="pdf") as doc:
        if doc.needs_pass or not doc.page_count:
            raise ValueError("PDF has no readable pages")
        page = doc[0]
        # Ограничиваем обе стороны даже для нестандартных размеров страниц.
        scale = min(1200 / page.rect.width, 1800 / page.rect.height)
        pix = page.get_pixmap(matrix=fitz.Matrix(scale, scale), alpha=False)
        return {
            "image": "data:image/png;base64," + base64.b64encode(pix.tobytes("png")).decode(),
            "affiliations": extract_affiliations(page),
        }
