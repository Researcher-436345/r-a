from __future__ import annotations

import json
import os
import re
from collections.abc import AsyncIterator
from datetime import date
from typing import Any, Literal
from urllib.parse import urlparse

import httpx
from fastapi import Depends, FastAPI, Header, HTTPException, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

ResearchMode = Literal["web", "deep"]

app = FastAPI(title="researcher-websearch", version="0.1.0")

LLM_BASE_URL = os.getenv(
    "WEBSEARCH_LLM_BASE_URL", "https://api.proxyapi.ru/openrouter/v1"
).rstrip("/")
LLM_API_KEY = os.getenv("WEBSEARCH_LLM_API_KEY", "") or os.getenv("LLM_API_KEY", "")
# Default is a model with built-in web search: it works both through direct
# OpenRouter and through proxies (proxyapi) that strip OpenRouter server tools.
LLM_MODEL = os.getenv("WEBSEARCH_LLM_MODEL", "perplexity/sonar-pro").strip()
DEEP_LLM_MODEL = os.getenv(
    "WEBSEARCH_DEEP_LLM_MODEL", "perplexity/sonar-deep-research"
).strip()
LLM_TIMEOUT_SECONDS = float(os.getenv("WEBSEARCH_TIMEOUT_SECONDS", "180"))
DEEP_LLM_TIMEOUT_SECONDS = float(os.getenv("WEBSEARCH_DEEP_TIMEOUT_SECONDS", "600"))
LLM_HTTP_REFERER = os.getenv("LLM_HTTP_REFERER", "http://localhost:5173")
LLM_APP_TITLE = os.getenv("LLM_APP_TITLE", "Researcher")
INTERNAL_TOKEN = os.getenv("INTERNAL_TOKEN", "")

# Scholarly-only search. Perplexity's search_domain_filter takes up to 10
# domains; without it a beginner question returns tutorials and blogs instead
# of papers, and the reader cannot open those inside the app.
# Only venues the catalog can turn into a readable paper: arXiv/DOI directly,
# open proceedings via their PDF, Semantic Scholar via the title fallback.
# OpenReview challenges every client and publisher sites are paywalled.
DEFAULT_SEARCH_DOMAINS = [
    "arxiv.org",
    "doi.org",
    "semanticscholar.org",
    "aclanthology.org",
    "proceedings.mlr.press",
    "openaccess.thecvf.com",
    "proceedings.neurips.cc",
    "papers.nips.cc",
    "ojs.aaai.org",
]

# Index, listing, home, help and account pages the model likes to cite. They
# are not papers, so they never become sources. Same URL classes as the
# not_a_paper check in backend/internal/modules/catalog/indexpages.go and
# frontend/src/features/papers/link-kind.ts: keep the three in sync. This copy
# and the frontend are deliberately broader (any site root, /vol/N anywhere).
NON_PAPER_HOSTS = frozenset(
    {
        "info.arxiv.org", "blog.arxiv.org", "status.arxiv.org", "search.arxiv.org",
        "labs.arxiv.org", "confluence.arxiv.org",
    }
)
# Scholarly sites whose search, login and help pages are portals.
SCHOLARLY_HOSTS = frozenset(
    {
        "arxiv.org", "export.arxiv.org", "ar5iv.labs.arxiv.org", "ar5iv.org",
        "openreview.net", "semanticscholar.org", "sciencedirect.com",
        "aclanthology.org", "proceedings.neurips.cc", "papers.nips.cc", "papers.neurips.cc",
        "openaccess.thecvf.com", "proceedings.mlr.press", "jmlr.org",
        "dl.acm.org", "ieeexplore.ieee.org", "link.springer.com", "springer.com",
        "nature.com", "science.org", "cell.com", "pnas.org",
        "pubmed.ncbi.nlm.nih.gov", "ncbi.nlm.nih.gov", "pmc.ncbi.nlm.nih.gov", "europepmc.org",
        "researchgate.net", "paperswithcode.com", "doi.org", "dx.doi.org",
        "openalex.org", "crossref.org", "biorxiv.org", "medrxiv.org", "ssrn.com",
        "papers.ssrn.com", "jstor.org", "onlinelibrary.wiley.com", "tandfonline.com",
        "mdpi.com", "frontiersin.org", "journals.plos.org", "hindawi.com",
        "ojs.aaai.org", "ijcai.org", "dblp.org", "cyberleninka.ru", "elibrary.ru",
    }
)
PORTAL_SECTIONS = frozenset(
    {
        "search", "login", "logout", "signin", "sign-in", "signup",
        "register", "help", "about", "contact", "faq", "subscribe",
    }
)
_ARXIV_SECTIONS = frozenset(
    {
        "list", "archive", "catchup", "year", "search", "a", "login", "logout", "help",
        "user", "auth", "register", "stats", "submit", "localization", "corr", "rss",
        "new", "multi", "institutional_banner", "ignoreme",
    }
)
# First path segment that is a listing on hosts whose papers live elsewhere.
NON_PAPER_SECTIONS: dict[str, frozenset[str]] = {
    "arxiv.org": _ARXIV_SECTIONS,
    "export.arxiv.org": _ARXIV_SECTIONS,
    "openreview.net": frozenset(
        {
            "venues", "venue", "group", "profile", "search", "login", "signup", "about",
            "tasks", "activity", "messages", "sponsors", "legal", "invitation",
        }
    ),
    "semanticscholar.org": frozenset(
        {
            "topic", "search", "product", "author", "venue", "about", "faq", "me", "sign-in",
            "alerts", "feed", "library", "api", "research", "cord19", "faqs",
        }
    ),
    "sciencedirect.com": frozenset({"journal", "browse", "search", "topics", "user"}),
    "aclanthology.org": frozenset(
        {"events", "venues", "volumes", "people", "search", "sigs", "faq", "info", "posts"}
    ),
}
# Whole-year pages ("Advances in Neural Information Processing Systems 36"),
# book, author and admin pages.
_NEURIPS_INDEX_RE = re.compile(
    r"^(?:/paper_files)?(?:/paper)?(?:/\d{4})?$|^/(?:book|author|admin)(?:/|$)", re.IGNORECASE
)
NON_PAPER_PATHS: dict[str, re.Pattern[str]] = {
    "proceedings.neurips.cc": _NEURIPS_INDEX_RE,
    "papers.nips.cc": _NEURIPS_INDEX_RE,
    "papers.neurips.cc": _NEURIPS_INDEX_RE,
    "proceedings.mlr.press": re.compile(r"^/v\d+$", re.IGNORECASE),
    "jmlr.org": re.compile(r"^/papers(?:/v\d+)?$", re.IGNORECASE),
}
# ar5iv serves papers only under these sections.
AR5IV_HOSTS = frozenset({"ar5iv.labs.arxiv.org", "ar5iv.org"})
AR5IV_PAPER_SECTIONS = frozenset({"html", "abs", "pdf"})
# Journal volume/issue tables of contents on any publisher.
VOLUME_PATH_RE = re.compile(r"/vol/\d+(/|$)", re.IGNORECASE)
ROOT_PATHS = frozenset({"", "/index.html", "/index.htm", "/index.php"})


def is_non_paper_url(url: str) -> bool:
    """Pages that list, search or introduce papers instead of being one.
    Decided from the URL alone: no network calls."""
    parsed = urlparse(url.strip())
    host = (parsed.hostname or "").lower().removeprefix("www.")
    if not host:
        return False
    path = parsed.path.rstrip("/")
    if (
        host in NON_PAPER_HOSTS
        or host.startswith("scholar.google.")
        or path.lower() in ROOT_PATHS
    ):
        return True
    section = path.lstrip("/").split("/", 1)[0].lower()
    if host in SCHOLARLY_HOSTS and section in PORTAL_SECTIONS:
        return True
    if section in NON_PAPER_SECTIONS.get(host, ()):
        return True
    pattern = NON_PAPER_PATHS.get(host)
    if pattern is not None and pattern.search(path):
        return True
    if host in AR5IV_HOSTS and section not in AR5IV_PAPER_SECTIONS:
        return True
    # CVF papers all live under /content*; the rest are menus and day indexes.
    if host == "openaccess.thecvf.com" and not section.startswith("content"):
        return True
    return VOLUME_PATH_RE.search(path) is not None


SEARCH_DOMAINS = [
    d.strip()
    for d in os.getenv("WEBSEARCH_DOMAINS", ",".join(DEFAULT_SEARCH_DOMAINS)).split(",")
    if d.strip()
][:10]

RESEARCH_SYSTEM_PROMPT = """
Ты — экспертный исследовательский ассистент с доступом к актуальному веб-поиску. 
Помогай пользователю находить и изучать научные статьи, направления исследований, методы и конкурирующие подходы.

Требования к исследованию:
- Сначала определи реальный исследовательский вопрос пользователя, важные термины и связанные подтемы.
- Весь ворфлоу и поиск делай на английском языке, а отвечай на том языке, на котором говорит пользователь.
- Выполни достаточно широкий поиск, чтобы выявить основные подходы, а затем проверь важные утверждения по первичным источникам.
- Отдавай приоритет рецензируемым статьям, оригинальным препринтам, страницам издательств, официальным страницам проектов, датасетам, бенчмаркам и документации. 
- Используй вторичные источники только для полезного дополнительного контекста.
- Текущая дата: {current_date}. Отдавай приоритет свежим работам, вышедшим за последние 1-3 месяца относительно этой даты, и только потом ищи более ранние работы, если пользователь ничего не говорит о датах.
- Указывай даты публикации и явно отличай рецензируемые работы от препринтов.
- Перепроверяй важные и потенциально спорные утверждения. 
- Описывай разногласия, ограничения, отрицательные результаты и неопределённость; не выдавай слабые доказательства за установленный факт.
- Никогда не выдумывай статьи, авторов, даты, метрики, цитаты, DOI, arXiv ID или URL. Если доказательств недостаточно, скажи об этом прямо.
- Приводи конкретные научные статьи и работы, непосредственно относящиеся к теме пользователя, а не только общее описание области.
- Для каждой рекомендуемой работы дай краткий обзор: название с прямой ссылкой на оригинал, авторов и год публикации при наличии, центральную идею, 
  использованный метод или подход, основные заявленные результаты и объяснение релевантности вопросу пользователя. 
- Отмечай существенные ограничения и статус работы — рецензируемая публикация или препринт.
- Отвечай на языке последнего сообщения пользователя.
- Не раскрывай скрытые рассуждения или внутреннюю цепочку мыслей. При необходимости показывай только краткие выводы о процессе поиска.

Требования к ответу:
- Если сообщение пользователя — приветствие, благодарность, короткая реплика или разговор не об исследованиях, ответь коротко, в одно-два предложения, и предложи сформулировать исследовательский вопрос. В таком ответе не должно быть ссылок, источников, таблиц и заголовков; остальные требования ниже относятся только к исследовательским вопросам.
- Верни только законченный ответ в Markdown.
- Давай именно статьи по порядку, а не просто другие различные факты.
- Используй ясную иерархию заголовков, короткие абзацы, **жирное выделение**, цитаты и Markdown-таблицы, когда они улучшают восприятие.
- Не используй маркированные или нумерованные списки, больше используй абзацы, не делай большие заголовки.
- Оформляй источники как кликабельные Markdown-ссылки: [точное название статьи](https://...).
- Каждая ссылка ведёт на страницу одной конкретной статьи. Предпочитай страницу arXiv (https://arxiv.org/abs/…) или DOI (https://doi.org/…), а не пересказ, блог или агрегатор: такие ссылки читатель открывает прямо в приложении.
- Никогда не ссылайся на страницы без одной конкретной статьи: списки и категории arXiv (arxiv.org/list/…, arxiv.org/archive/…), оглавления журналов, томов и выпусков, сборники трудов конференций целиком или за год (например, proceedings.neurips.cc/paper_files/paper/2024), меню и программы конференций, страницы поиска, тем, площадок и профилей авторов, страницы входа и справки, главные страницы сайтов (arxiv.org, openreview.net, semanticscholar.org, sciencedirect.com, aclanthology.org). Если страницы конкретной статьи нет, назови статью без ссылки.
- Текст ссылки — точное оригинальное название статьи на языке оригинала, без «[PDF]», arXiv ID, названия сайта, многоточий и пересказа своими словами: по этому названию статья открывается внутри приложения.
- Указывай arXiv ID только если он взят из результата поиска именно для этой статьи и название там совпадает; никогда не подставляй arXiv ID по памяти. Без такого подтверждения ссылайся на DOI или на страницу статьи в Semantic Scholar (https://www.semanticscholar.org/paper/…).
- Не рекомендуй уроки, курсы и посты в блогах, если пользователь явно не просил учебные материалы; для «с чего начать» подбирай обзорные статьи (survey) и классические работы.
- Размещай ссылки на источники рядом с фактическими, актуальными и количественными утверждениями. Не выводи непроверяемые голые URL.
- Заверши ответ таблицей с найденными работами и ссылками на уникальные источники на языке ответа, если API-цитаты уже не образуют эквивалентный список ссылок.
- Дай содержательный синтез, а не простой перечень результатов.
- Объясни различия между подходами, условия их применимости и границы имеющихся доказательств, это можно добавлять в таблицу.
"""


class SearchMessage(BaseModel):
    role: Literal["user", "assistant"]
    content: str = Field(min_length=1, max_length=100_000)


class SearchRequest(BaseModel):
    messages: list[SearchMessage] = Field(min_length=1, max_length=40)
    mode: ResearchMode = "web"


class Source(BaseModel):
    title: str
    url: str
    domain: str
    published_at: str | None = None
    # Provider position the answer's [n] markers refer to; internal only.
    citation: int | None = Field(default=None, exclude=True)


def require_internal_token(x_internal_token: str | None = Header(default=None)) -> None:
    if INTERNAL_TOKEN and x_internal_token != INTERNAL_TOKEN:
        raise HTTPException(status_code=401, detail="invalid internal token")


def system_prompt(mode: ResearchMode, current_date: date | None = None) -> str:
    prompt_date = current_date or date.today()
    if mode == "deep":
        suffix = """
Режим глубокого исследования:
- Проведи полный многоэтапный поиск и сопоставь несколько независимых первичных источников.
- Подготовь очень подробный исследовательский отчёт. Не сокращай материал ради краткости: полнота и глубина важнее объёма ответа.
- Последовательно раскрой контекст и терминологию, основные подходы и методологии, доказательства и результаты, сравнение работ, противоречия, ограничения и открытые вопросы.
- Подкрепляй источниками каждое существенное фактическое утверждение и явно отделяй подтверждённые выводы от интерпретаций.
- Заверши содержательными выводами и конкретными рекомендациями для дальнейшего чтения.
""".strip()
    else:
        suffix = """
Режим веб-поиска:
- Ответь эффективно и по существу, проверив наиболее важные утверждения и добавив прямые ссылки на источники.
- Дай небольшой, но подробный и ёмкий ответ.
""".strip()
    prompt = RESEARCH_SYSTEM_PROMPT.format(current_date=prompt_date.isoformat())
    return f"{prompt}\n\n{suffix}"


def supports_openrouter_tools(base_url: str | None = None) -> bool:
    """OpenRouter server tools (openrouter:web_search / web_fetch) exist only on
    openrouter.ai itself. Proxies such as proxyapi validate `tools` as plain
    OpenAI function tools and answer 400 ("'function' is a required property"),
    and they ignore `plugins` / `:online` too — so behind a proxy the model has
    to bring its own search (perplexity/sonar*)."""
    return "openrouter.ai" in (base_url if base_url is not None else LLM_BASE_URL)


def model_for_mode(mode: ResearchMode) -> str:
    return DEEP_LLM_MODEL if mode == "deep" else LLM_MODEL


def timeout_for_mode(mode: ResearchMode) -> float:
    return DEEP_LLM_TIMEOUT_SECONDS if mode == "deep" else LLM_TIMEOUT_SECONDS


def normalize_source(raw: Any) -> Source | None:
    if isinstance(raw, dict) and raw.get("type") == "url_citation":
        raw = raw.get("url_citation")
    if isinstance(raw, str):
        url = raw.strip()
        title = ""
        published_at = None
    elif isinstance(raw, dict):
        url = str(raw.get("url") or raw.get("link") or "").strip()
        title = str(raw.get("title") or raw.get("name") or "").strip()
        published_at = raw.get("published_at") or raw.get("published_date") or raw.get("date")
        published_at = str(published_at) if published_at else None
    else:
        return None
    parsed = urlparse(url)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        return None
    if is_non_paper_url(url):
        return None
    domain = parsed.hostname or parsed.netloc
    citation = raw.get("citation_index") if isinstance(raw, dict) else None
    return Source(
        title=title or domain,
        url=url,
        domain=domain,
        published_at=published_at,
        citation=citation if isinstance(citation, int) and citation > 0 else None,
    )


def indexed_citations(values: Any) -> list[Any]:
    """Perplexity's [n] markers point at position n of citations / search_results;
    the position must survive dropping index pages and duplicates."""
    out: list[Any] = []
    for position, value in enumerate(values or [], start=1):
        if isinstance(value, str):
            out.append({"url": value, "citation_index": position})
        elif isinstance(value, dict):
            out.append({**value, "citation_index": position})
    return out


def merge_sources(current: list[Source], values: list[Any]) -> None:
    by_url = {source.url: source for source in current}
    for value in values:
        source = normalize_source(value)
        if source is None:
            continue
        existing = by_url.get(source.url)
        if existing is None:
            current.append(source)
            by_url[source.url] = source
        elif existing.citation is None and source.citation is not None:
            existing.citation = source.citation


SOURCES_HEADINGS = ("\n## sources", "\n## источники")
MARKDOWN_LINK_URL_RE = re.compile(r"\]\((https?://[^\s)]+)\)")
CITES_RE = re.compile(r"https?://|\[\d{1,3}\]")


def answer_cites_sources(answer: str) -> bool:
    """Small talk comes back with search results attached but with no links or
    [n] markers; listing those results under a greeting is noise."""
    return CITES_RE.search(answer) is not None


def has_linked_sources_section(answer: str) -> bool:
    lowered = answer.lower()
    start = max(lowered.rfind(heading) for heading in SOURCES_HEADINGS)
    if start < 0:
        return False
    # A section of index pages only still needs the real source list.
    return any(
        not is_non_paper_url(url) for url in MARKDOWN_LINK_URL_RE.findall(lowered[start:])
    )


def markdown_sources(sources: list[Source]) -> str:
    papers = [source for source in sources if not is_non_paper_url(source.url)]
    if not papers:
        return ""
    if all(source.citation is not None for source in papers):
        numbered = sorted(((source.citation or 0, source) for source in papers), key=lambda item: item[0])
    else:
        numbered = list(enumerate(papers, start=1))
    lines = ["", "", "## Sources", ""]
    if [number for number, _ in numbered] == list(range(1, len(numbered) + 1)):
        for number, source in numbered:
            title = source.title.replace("[", "").replace("]", "")
            lines.append(f"{number}. [{title}]({source.url})")
        return "\n".join(lines)
    # Dropped citations leave gaps; a markdown list would renumber the items
    # 1..k, so each keeps its provider number as an escaped plain paragraph.
    items = []
    for number, source in numbered:
        title = source.title.replace("[", "").replace("]", "")
        items.append(f"{number}\\. [{title}]({source.url})")
    return "\n".join(lines) + "\n" + "\n\n".join(items)


def sse(event: str, payload: Any) -> str:
    return f"event: {event}\ndata: {json.dumps(payload, ensure_ascii=False)}\n\n"


def provider_body(payload: SearchRequest) -> dict[str, Any]:
    body: dict[str, Any] = {
        "model": model_for_mode(payload.mode),
        "messages": [
            {"role": "system", "content": system_prompt(payload.mode)},
            *(message.model_dump() for message in payload.messages),
        ],
        "stream": True,
    }
    if SEARCH_DOMAINS and model_for_mode(payload.mode).startswith("perplexity/"):
        body["search_domain_filter"] = SEARCH_DOMAINS
    if payload.mode == "web" and supports_openrouter_tools():
        body["tools"] = [
            {
                "type": "openrouter:web_search",
                "parameters": {
                    "engine": "auto",
                    "max_results": 5,
                    "max_total_results": 20,
                },
            },
            {
                "type": "openrouter:web_fetch",
                "parameters": {
                    "max_uses": 10,
                    "max_content_tokens": 50_000,
                },
            },
        ]
    else:
        body["reasoning"] = {"effort": "high", "summary": "concise"}
    return body


def reasoning_summary_fragments(chunk: dict[str, Any]) -> list[str]:
    fragments: list[str] = []
    for choice in chunk.get("choices") or []:
        delta = choice.get("delta") or {}
        for detail in delta.get("reasoning_details") or []:
            if not isinstance(detail, dict) or detail.get("type") != "reasoning.summary":
                continue
            summary = detail.get("summary")
            if isinstance(summary, str) and summary:
                fragments.append(summary)
    return fragments


async def provider_stream(payload: SearchRequest) -> AsyncIterator[tuple[str, Any]]:
    if not LLM_API_KEY:
        raise RuntimeError("веб-поиск не настроен: добавь WEBSEARCH_LLM_API_KEY или LLM_API_KEY")
    model = model_for_mode(payload.mode)
    if not model:
        variable = "WEBSEARCH_DEEP_LLM_MODEL" if payload.mode == "deep" else "WEBSEARCH_LLM_MODEL"
        raise RuntimeError(f"веб-поиск не настроен: добавь {variable}")

    headers = {"Authorization": f"Bearer {LLM_API_KEY}", "Content-Type": "application/json"}
    if "openrouter.ai" in LLM_BASE_URL:
        headers["HTTP-Referer"] = LLM_HTTP_REFERER
        headers["X-Title"] = LLM_APP_TITLE
    body = provider_body(payload)
    timeout = httpx.Timeout(timeout_for_mode(payload.mode), connect=20.0)
    progress_parts: list[str] = []
    async with httpx.AsyncClient(timeout=timeout) as client:
        async with client.stream(
            "POST", f"{LLM_BASE_URL}/chat/completions", headers=headers, json=body
        ) as response:
            if response.status_code // 100 != 2:
                detail = (await response.aread())[:8192].decode("utf-8", errors="replace").strip()
                raise RuntimeError(f"web search provider returned {response.status_code}: {detail or response.reason_phrase}")
            async for line in response.aiter_lines():
                line = line.strip()
                if not line or line.startswith(":"):
                    continue
                if line.startswith("data:"):
                    line = line.removeprefix("data:").strip()
                if line == "[DONE]":
                    break
                try:
                    chunk = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if payload.mode == "deep":
                    for summary in reasoning_summary_fragments(chunk):
                        progress_parts.append(summary)
                        yield "progress", "".join(progress_parts)[-4_000:]
                choices = chunk.get("choices") or []
                delta = ""
                if choices:
                    delta = str((choices[0].get("delta") or {}).get("content") or "")
                if delta:
                    yield "delta", delta
                raw_sources: list[Any] = []
                raw_sources.extend(indexed_citations(chunk.get("citations")))
                raw_sources.extend(indexed_citations(chunk.get("search_results")))
                for choice in choices:
                    raw_sources.extend((choice.get("delta") or {}).get("annotations") or [])
                    raw_sources.extend((choice.get("message") or {}).get("annotations") or [])
                if raw_sources:
                    yield "sources", raw_sources


@app.get("/health")
def health() -> dict[str, Any]:
    return {
        "status": "ok",
        "service": "websearch",
        "model": LLM_MODEL,
        "models": {"web": LLM_MODEL, "deep": DEEP_LLM_MODEL},
        "version": "0.1.0",
    }


@app.post("/v1/search/stream", dependencies=[Depends(require_internal_token)])
async def search_stream(body: SearchRequest, request: Request) -> StreamingResponse:
    async def events() -> AsyncIterator[str]:
        answer_parts: list[str] = []
        sources: list[Source] = []
        try:
            async for event, value in provider_stream(body):
                if await request.is_disconnected():
                    return
                if event == "delta":
                    answer_parts.append(value)
                    yield sse("delta", {"content": value})
                elif event == "progress":
                    yield sse("progress", {"content": value})
                elif event == "sources":
                    previous_count = len(sources)
                    merge_sources(sources, value)
                    if body.mode == "deep" and len(sources) > previous_count:
                        yield sse(
                            "source_progress",
                            {
                                "count": len(sources),
                                "sources": [source.model_dump() for source in sources[-3:]],
                            },
                        )
            # Deltas are already sent and searchapi persists their concatenation,
            # so links inside the text are not rewritten here (the frontend shows
            # non-paper links as explicit external links). Only the appended
            # section and the sources payload are guaranteed paper-only.
            answer = "".join(answer_parts)
            if answer_cites_sources(answer) and not has_linked_sources_section(answer):
                suffix = markdown_sources(sources)
                if suffix:
                    answer_parts.append(suffix)
                    yield sse("delta", {"content": suffix})
            if not "".join(answer_parts).strip():
                raise RuntimeError("web search provider returned an empty response")
            yield sse("sources", {"sources": [source.model_dump() for source in sources]})
            yield sse("done", {"status": "ok"})
        except Exception as exc:  # noqa: BLE001
            yield sse("error", {"detail": str(exc)})

    return StreamingResponse(
        events(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache, no-transform", "X-Accel-Buffering": "no"},
    )
