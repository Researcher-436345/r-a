from datetime import date

from fastapi.testclient import TestClient

import app.main as main
from app.main import (
    KnownEvent,
    SearchRequest,
    Source,
    event_discovery_body,
    has_linked_sources_section,
    markdown_sources,
    merge_sources,
    provider_body,
    extract_json_object,
    reasoning_summary_fragments,
    system_prompt,
    validate_discovered_events,
)


def test_event_discovery_json_is_extracted_from_plain_text_or_fence() -> None:
    expected = {"items": [{"id": "neurips-2026"}]}
    assert extract_json_object('{"items":[{"id":"neurips-2026"}]}') == expected
    assert extract_json_object(
        '```json\n{"items":[{"id":"neurips-2026"}]}\n```'
    ) == expected


def test_event_discovery_keeps_valid_items_when_one_item_is_invalid() -> None:
    valid = {
        "id": "new-tech-event-2026",
        "title": "New Tech Event 2026",
        "summary": "Российская инженерная конференция о разработке, данных и инфраструктуре.",
        "start_date": "2026-10-10",
        "end_date": "2026-10-10",
        "city": "Москва",
        "country": "Россия",
        "format": "in_person",
        "kind": "conference",
        "region": "ru",
        "topics": ["AI"],
        "url": "https://example.com/event",
        "registration_url": None,
        "source_url": "https://example.com/event",
        "featured": False,
    }
    items, rejected = validate_discovered_events([valid, {"id": "broken"}])

    assert [item["id"] for item in items] == ["new-tech-event-2026"]
    assert rejected == 1


def test_event_discovery_requires_dates_in_official_page_citation() -> None:
    event = {
        "id": "new-tech-event-2026",
        "title": "New Tech Event 2026",
        "summary": "Российская инженерная конференция о разработке, данных и инфраструктуре.",
        "start_date": "2026-09-12",
        "end_date": "2026-09-13",
        "city": "Москва",
        "country": "Россия",
        "format": "in_person",
        "kind": "conference",
        "region": "ru",
        "topics": ["AI"],
        "url": "https://events.example.com/tech-2026",
        "registration_url": None,
        "source_url": "https://events.example.com/tech-2026",
        "featured": False,
    }
    confirmed = [{
        "type": "url_citation",
        "url_citation": {
            "url": "https://events.example.com/tech-2026/",
            "content": "Конференция состоится 12–13 сентября 2026 года в Москве.",
        },
    }]
    unconfirmed = [{
        "type": "url_citation",
        "url_citation": {
            "url": "https://events.example.com/tech-2026",
            "content": "Следите за новостями — даты будут объявлены позже.",
        },
    }]

    items, rejected = validate_discovered_events(
        [event], confirmed, require_date_citation=True
    )
    assert len(items) == 1
    assert rejected == 0

    items, rejected = validate_discovered_events(
        [event], unconfirmed, require_date_citation=True
    )
    assert items == []
    assert rejected == 1


def test_event_discovery_uses_one_bounded_web_search() -> None:
    body = event_discovery_body(
        date(2026, 8, 16),
        [
            KnownEvent(
                id="neurips-2026",
                title="NeurIPS 2026",
                start_date="2026-12-06",
                url="https://neurips.cc/Conferences/2026",
            )
        ],
    )

    assert body["model"] == "deepseek/deepseek-v4-flash"
    assert body["stream"] is False
    assert body["tools"] == [
        {
            "type": "openrouter:web_search",
            "parameters": {
                "engine": "exa",
                "max_results": 5,
                "max_total_results": 50,
                "search_context_size": "low",
            },
        }
    ]
    assert "Today is 2026-08-16" in body["messages"][1]["content"]
    assert '"title":"NeurIPS 2026"' in body["messages"][1]["content"]
    assert "at most 50 unique event editions" in body["messages"][1]["content"]
    assert body["stop_server_tools_when"] == [
        {"type": "step_count_is", "step_count": 24},
        {"type": "max_cost", "max_cost_in_dollars": 0.08},
    ]


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


def test_provider_body_uses_openrouter_web_tools() -> None:
    body = provider_body(
        SearchRequest(messages=[{"role": "user", "content": "question"}], mode="web")
    )

    assert body["model"] == "deepseek/deepseek-v4-flash"
    assert body["stream"] is True
    assert body["tools"] == [
        {
            "type": "openrouter:web_search",
            "parameters": {"engine": "auto", "max_results": 5, "max_total_results": 20},
        },
        {
            "type": "openrouter:web_fetch",
            "parameters": {"max_uses": 10, "max_content_tokens": 50_000},
        },
    ]


def test_provider_body_uses_deep_research_model() -> None:
    body = provider_body(
        SearchRequest(messages=[{"role": "user", "content": "question"}], mode="deep")
    )

    assert body["model"] == "perplexity/sonar-deep-research"
    assert "tools" not in body
    assert body["reasoning"] == {"effort": "high", "summary": "concise"}


def test_only_displayable_reasoning_summaries_become_progress() -> None:
    fragments = reasoning_summary_fragments(
        {
            "choices": [
                {
                    "delta": {
                        "reasoning": "hidden chain of thought",
                        "reasoning_details": [
                            {
                                "type": "reasoning.summary",
                                "summary": "Нашёл несколько релевантных работ; проверяю детали.",
                            },
                            {"type": "reasoning.text", "text": "hidden reasoning"},
                        ],
                    }
                }
            ]
        }
    )

    assert fragments == ["Нашёл несколько релевантных работ; проверяю детали."]


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
