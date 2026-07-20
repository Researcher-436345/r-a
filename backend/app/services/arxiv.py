from __future__ import annotations

import re
import xml.etree.ElementTree as ET
from dataclasses import dataclass

import httpx

ARXIV_ID_RE = re.compile(
    r"(?:(?:https?://)?(?:www\.)?arxiv\.org/(?:abs|pdf)/)?"
    r"(?P<id>\d{4}\.\d{4,5}(?:v\d+)?|[a-z\-]+(?:\.[A-Z]{2})?/\d{7}(?:v\d+)?)",
    re.IGNORECASE,
)
ATOM_NS = {"atom": "http://www.w3.org/2005/Atom", "arxiv": "http://arxiv.org/schemas/atom"}


@dataclass
class ArxivPaper:
    arxiv_id: str
    title: str
    abstract: str | None
    authors: list[str]
    year: int | None
    pdf_url: str


def normalize_arxiv_id(value: str) -> str:
    raw = value.strip()
    match = ARXIV_ID_RE.search(raw)
    if not match:
        raise ValueError("Invalid arXiv id or URL")
    arxiv_id = match.group("id")
    # drop version suffix for storage uniqueness (keep pdf fetch with version if present)
    return arxiv_id


def canonical_arxiv_id(arxiv_id: str) -> str:
    return re.sub(r"v\d+$", "", arxiv_id, flags=re.IGNORECASE)


def fetch_arxiv_metadata(arxiv_id: str) -> ArxivPaper:
    query_id = arxiv_id
    url = f"https://export.arxiv.org/api/query?id_list={query_id}"
    with httpx.Client(timeout=30.0) as client:
        response = client.get(url)
        response.raise_for_status()

    root = ET.fromstring(response.text)
    entry = root.find("atom:entry", ATOM_NS)
    if entry is None:
        raise ValueError("arXiv paper not found")

    title = " ".join((entry.findtext("atom:title", default="", namespaces=ATOM_NS) or "").split())
    abstract = entry.findtext("atom:summary", default=None, namespaces=ATOM_NS)
    if abstract:
        abstract = " ".join(abstract.split())

    authors = [
        " ".join((node.findtext("atom:name", default="", namespaces=ATOM_NS) or "").split())
        for node in entry.findall("atom:author", ATOM_NS)
    ]
    authors = [name for name in authors if name]

    published = entry.findtext("atom:published", default=None, namespaces=ATOM_NS)
    year = int(published[:4]) if published and len(published) >= 4 else None

    pdf_url = f"https://arxiv.org/pdf/{canonical_arxiv_id(arxiv_id)}.pdf"
    for link in entry.findall("atom:link", ATOM_NS):
        if link.attrib.get("title") == "pdf" or link.attrib.get("type") == "application/pdf":
            href = link.attrib.get("href")
            if href:
                pdf_url = href
                break

    return ArxivPaper(
        arxiv_id=canonical_arxiv_id(arxiv_id),
        title=title or f"arXiv:{canonical_arxiv_id(arxiv_id)}",
        abstract=abstract,
        authors=authors,
        year=year,
        pdf_url=pdf_url,
    )


def download_pdf(pdf_url: str) -> bytes:
    with httpx.Client(timeout=60.0, follow_redirects=True) as client:
        response = client.get(pdf_url)
        response.raise_for_status()
        return response.content
