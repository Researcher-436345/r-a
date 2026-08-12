from fastapi.testclient import TestClient

import app.main as main
from app.main import Source, has_linked_sources_section, markdown_sources, merge_sources


def test_sources_are_normalized_and_deduplicated() -> None:
    sources: list[Source] = []
    merge_sources(
        sources,
        [
            "https://arxiv.org/abs/1234.5678",
            {"url": "https://arxiv.org/abs/1234.5678", "title": "Duplicate"},
            {"url": "https://example.com/paper", "title": "Paper", "date": "2026-01-02"},
            "not-a-url",
        ],
    )
    assert [source.url for source in sources] == [
        "https://arxiv.org/abs/1234.5678",
        "https://example.com/paper",
    ]
    assert sources[1].published_at == "2026-01-02"


def test_markdown_fallback_is_only_needed_without_linked_section() -> None:
    source = Source(title="A [paper]", url="https://example.com/paper", domain="example.com")
    rendered = markdown_sources([source])
    assert rendered == "\n\n## Sources\n\n1. [A paper](https://example.com/paper)"
    assert has_linked_sources_section(f"Answer{rendered}")
    assert not has_linked_sources_section("Answer\n\n## Sources\n\nNo links")


def test_stream_exposes_text_and_structured_sources() -> None:
    async def fake_provider_stream(_payload):
        yield "delta", "Answer"
        yield "sources", [{"title": "Paper", "url": "https://example.com/paper"}]

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
    assert 'event: delta\ndata: {"content": "Answer"}' in response.text
    assert 'event: sources' in response.text
    assert '"domain": "example.com"' in response.text
    assert 'event: done\ndata: {"status": "ok"}' in response.text
