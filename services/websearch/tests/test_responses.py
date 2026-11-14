import asyncio
import json
from dataclasses import replace

import httpx
import pytest
from fastapi.testclient import TestClient

import app.main as main
import settings as application_settings
from app.provider import cited_text, stream_response
from app.settings import SearchSettings


def sse(*events):
    return "".join(f"data: {json.dumps(event)}\n\n" for event in events)


def settings():
    current = SearchSettings.load()

    return replace(
        current,
        base_url="https://openai.example/v1",
        api_key="test-key",
        web=replace(current.web, model="test-web"),
        deep=replace(current.deep, model="test-deep"),
    )


def collect(config, handler, *, deep=False):

    async def run():
        profile = config.deep if deep else config.web

        return [event async for event in stream_response(
            config, profile, {"stream": True}, transport=httpx.MockTransport(handler),
        )]

    return asyncio.run(run())


def completed(text="Answer", annotations=None):

    return {
        "type": "response.completed",
        "sequence_number": 4,
        "response": {
            "status": "completed",
            "usage": {"input_tokens": 20, "output_tokens": 10},
            "output": [{"type": "message", "content": [{
                "type": "output_text", "text": text, "annotations": annotations or [],
            }]}],
        },
    }


def test_stream_uses_responses_and_returns_final_clickable_citations():

    annotation = {
        "type": "url_citation", "url": "https://arxiv.org/abs/2601.00001",
        "title": "Paper", "start_index": 7, "end_index": 10,
    }

    def handler(request):
        assert request.url.path == "/v1/responses"
        assert request.headers["authorization"] == "Bearer test-key"

        return httpx.Response(200, text=sse(
            {"type": "response.output_text.delta", "sequence_number": 1, "delta": "Answer [1]"},
            completed("Answer [1]", [annotation]),
        ))

    events = collect(settings(), handler)

    assert ("delta", "Answer [1]") in events
    assert ("replace", "Answer [Paper](https://arxiv.org/abs/2601.00001)") in events
    assert ("usage", {"input_tokens": 20, "output_tokens": 10}) in events


def test_deep_reconnects_by_id_without_starting_a_second_paid_request():

    calls = []

    def handler(request):
        calls.append(request)
        if request.method == "POST":
            return httpx.Response(200, text=sse(
                {"type": "response.created", "sequence_number": 0, "response": {"id": "resp_test"}},
                {"type": "response.output_text.delta", "sequence_number": 1, "delta": "First "},
                {"type": "keepalive", "sequence_number": 999},
            ))

        assert request.url.path == "/v1/responses/resp_test"
        assert request.url.params["starting_after"] == "1"

        return httpx.Response(200, text=sse(
            {"type": "response.output_text.delta", "sequence_number": 1, "delta": "First "},
            {"type": "response.output_text.delta", "sequence_number": 2, "delta": "second."},
            completed("First second."),
        ))

    events = collect(settings(), handler, deep=True)

    assert [call.method for call in calls] == ["POST", "GET"]
    assert [value for name, value in events if name == "delta"] == ["First ", "second."]
    assert ("response", {"id": "resp_test"}) in events
    assert ("ping", None) in events


def test_missing_model_fails_before_http():

    config = settings()
    config = replace(config, web=replace(config.web, model=""))

    def handler(request):
        pytest.fail("Unconfigured model reached the network")

    with pytest.raises(RuntimeError, match="LLM_MODEL"):
        collect(config, handler)


def test_invalid_budget_fails_when_loading_search_settings(monkeypatch):

    monkeypatch.setattr(application_settings, "WEBSEARCH_MAX_TOOL_CALLS", "-1")

    with pytest.raises(ValueError, match="must be positive"):
        SearchSettings.load()


def test_unfinished_web_stream_does_not_become_a_successful_answer():

    def handler(request):
        return httpx.Response(200, text=sse({"type": "response.output_text.delta", "delta": "Partial"}))

    with pytest.raises(httpx.ReadError, match="before completion"):
        collect(settings(), handler)


def test_failed_background_response_is_cancelled_and_not_published():

    calls = []

    def handler(request):
        calls.append(request.url.path)
        if request.url.path.endswith("/cancel"):
            return httpx.Response(200, json={"status": "cancelled"})

        return httpx.Response(200, text=sse(
            {"type": "response.created", "sequence_number": 0, "response": {"id": "resp_test"}},
            {"type": "response.incomplete", "sequence_number": 1, "response": {
                "status": "incomplete", "incomplete_details": {"reason": "max_output_tokens"},
            }},
        ))

    with pytest.raises(RuntimeError, match="max_output_tokens"):
        collect(settings(), handler, deep=True)

    assert calls[-1] == "/v1/responses/resp_test/cancel"


def test_hidden_reasoning_is_not_exposed_as_progress():

    def handler(request):
        return httpx.Response(200, text=sse(
            {"type": "response.reasoning_text.delta", "delta": "private reasoning"},
            {"type": "response.reasoning_summary_text.delta", "delta": "Проверяю источники."},
            completed(),
        ))

    events = collect(settings(), handler)

    assert ("progress", "Проверяю источники.") in events
    assert "private reasoning" not in str(events)


def test_existing_markdown_links_are_not_wrapped_twice():

    text = "Answer [Paper](https://arxiv.org/abs/2601.00001)"
    annotation = {
        "type": "url_citation", "url": "https://arxiv.org/abs/2601.00001",
        "title": "Paper", "start_index": 7, "end_index": len(text),
    }

    assert cited_text(text, [annotation]) == text


def test_response_lookup_and_cancel_require_internal_token(monkeypatch):

    monkeypatch.setattr(main, "SETTINGS", replace(main.SETTINGS, internal_token="private-internal-token"))
    client = TestClient(main.app)

    assert client.get("/v1/search/responses/resp_private").status_code == 401
    assert client.delete("/v1/search/responses/resp_private").status_code == 401


def test_expired_response_releases_research_instead_of_waiting_forever(monkeypatch):

    async def missing_response(_settings, response_id):
        request = httpx.Request("GET", f"https://openai.example/v1/responses/{response_id}")
        response = httpx.Response(404, request=request)
        response.raise_for_status()

    monkeypatch.setattr(main, "response_request", missing_response)
    response = TestClient(main.app).get("/v1/search/responses/resp_expired")

    assert response.status_code == 200
    assert response.json()["status"] == "failed"
