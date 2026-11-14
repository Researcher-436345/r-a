from datetime import date
from dataclasses import replace

from fastapi.testclient import TestClient

import app.main as main
from app.main import (
    SearchRequest,
    Source,
    has_linked_sources_section,
    markdown_sources,
    merge_sources,
    provider_body,
    system_prompt,
)


def test_sources_are_normalized_and_deduplicated() -> None:
    sources: list[Source] = []
    merge_sources(
        sources,
        [
            "https://arxiv.org/abs/1234.5678",
            {"url": "https://arxiv.org/abs/1234.5678", "title": "Duplicate"},
            {"url": "https://example.com/paper", "title": "Paper", "date": "2026-01-02"},
            {
                "type": "url_citation",
                "url_citation": {
                    "url": "https://example.org/cited-paper",
                    "title": "Cited paper",
                },
            },
            "not-a-url",
        ],
    )
    assert [source.url for source in sources] == [
        "https://arxiv.org/abs/1234.5678",
        "https://example.com/paper",
        "https://example.org/cited-paper",
    ]
    assert sources[1].published_at == "2026-01-02"
    assert sources[0].title == "Duplicate"


def test_provider_body_uses_openai_hosted_web_search(monkeypatch) -> None:
    monkeypatch.setattr(main, "SETTINGS", replace(main.SETTINGS, web=replace(main.SETTINGS.web, model="test-web")))
    body = provider_body(SearchRequest(messages=[{"role": "user", "content": "question"}]))

    assert body["model"] == "test-web"
    assert body["tools"][0]["type"] == "web_search"
    assert body["tool_choice"] == "required"
    assert body["background"] is False
    assert body["store"] is False
    assert body["input"] == [{"role": "user", "content": "question"}]
    assert "messages" not in body


def test_deep_research_uses_background_and_separate_budget(monkeypatch) -> None:
    monkeypatch.setattr(main, "SETTINGS", replace(main.SETTINGS, deep=replace(main.SETTINGS.deep, model="test-deep")))
    body = provider_body(SearchRequest(messages=[{"role": "user", "content": "question"}], mode="deep"))

    assert body["model"] == "test-deep"
    assert body["tools"][0]["type"] == "web_search"
    assert body["background"] is True
    assert body["store"] is True
    assert body["reasoning"]["effort"] == main.SETTINGS.deep.reasoning_effort
    assert body["max_tool_calls"] > main.SETTINGS.web.max_tool_calls
    assert body["max_output_tokens"] > main.SETTINGS.web.max_output_tokens


def test_system_prompt_contains_current_date_for_recency_window() -> None:
    prompt = system_prompt("web", current_date=date(2026, 8, 12))

    assert "Текущая дата: 2026-08-12" in prompt
    assert "последние 1-3 месяца относительно этой даты" in prompt


def test_deep_prompt_requests_full_report_without_compactness() -> None:
    web_prompt = system_prompt("web", current_date=date(2026, 8, 12))
    deep_prompt = system_prompt("deep", current_date=date(2026, 8, 12))

    assert "небольшой, но подробный и ёмкий ответ" in web_prompt
    assert "небольшой, но подробный и ёмкий ответ" not in deep_prompt
    assert "ёмк" not in deep_prompt
    assert "очень подробный исследовательский отчёт" in deep_prompt
    assert "Не сокращай материал ради краткости" in deep_prompt


def test_markdown_fallback_is_only_needed_without_linked_section() -> None:
    source = Source(title="A [paper]", url="https://example.com/paper", domain="example.com")
    rendered = markdown_sources([source])
    assert rendered == "\n\n## Sources\n\n1. [A paper](https://example.com/paper)"
    assert has_linked_sources_section(f"Answer{rendered}")
    assert not has_linked_sources_section("Answer\n\n## Sources\n\nNo links")


def test_stream_exposes_text_and_structured_sources() -> None:
    async def fake_provider_stream(_payload):
        yield "progress", "Нашёл релевантные работы; уточняю выводы."
        yield "sources", [{"title": "Paper", "url": "https://example.com/paper"}]
        yield "delta", "Answer"

    original = main.provider_stream
    main.provider_stream = fake_provider_stream
    try:
        response = TestClient(main.app).post(
            "/v1/search/stream",
            json={"messages": [{"role": "user", "content": "question"}], "mode": "deep"},
        )
    finally:
        main.provider_stream = original

    assert response.status_code == 200
    assert 'event: progress\ndata: {"content": "Нашёл релевантные работы; уточняю выводы."}' in response.text
    assert 'event: source_progress\ndata: {"count": 1, "sources": [{"title": "Paper"' in response.text
    assert 'event: delta\ndata: {"content": "Answer"}' in response.text
    assert 'event: sources' in response.text
    assert '"domain": "example.com"' in response.text
    assert 'event: done\ndata: {"status": "ok"}' in response.text


def test_web_stream_does_not_emit_source_progress() -> None:
    async def fake_provider_stream(_payload):
        yield "sources", [{"title": "Paper", "url": "https://example.com/paper"}]
        yield "delta", "Answer"

    original = main.provider_stream
    main.provider_stream = fake_provider_stream
    try:
        response = TestClient(main.app).post(
            "/v1/search/stream",
            json={"messages": [{"role": "user", "content": "question"}], "mode": "web"},
        )
    finally:
        main.provider_stream = original

    assert response.status_code == 200
    assert "event: source_progress" not in response.text


def test_provider_body_filters_scholarly_domains(monkeypatch) -> None:
    monkeypatch.setattr(main, "SETTINGS", replace(main.SETTINGS, domains=("arxiv.org", "doi.org")))
    body = provider_body(SearchRequest(messages=[{"role": "user", "content": "question"}]))

    assert body["tools"][0]["filters"] == {"allowed_domains": ["arxiv.org", "doi.org"]}


def test_empty_domains_allow_unrestricted_search(monkeypatch) -> None:
    monkeypatch.setattr(main, "SETTINGS", replace(main.SETTINGS, domains=()))
    body = provider_body(SearchRequest(messages=[{"role": "user", "content": "question"}]))

    assert "filters" not in body["tools"][0]


def test_index_pages_are_not_sources() -> None:
    sources: list[Source] = []
    merge_sources(
        sources,
        [
            "https://www.sciencedirect.com/journal/computer-vision-and-image-understanding/vol/256",
            "https://arxiv.org/list/cs.CV/recent",
            "https://www.semanticscholar.org/search?q=opencv",
            "https://arxiv.org/abs/1511.08458",
            "https://www.semanticscholar.org/paper/Title/abc123",
        ],
    )
    assert [source.url for source in sources] == [
        "https://arxiv.org/abs/1511.08458",
        "https://www.semanticscholar.org/paper/Title/abc123",
    ]
